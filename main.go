package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
)

const (
	defaultAnnotation = "initializers.kubernetes.io/environ"
	defaultConfigmap  = "environ-initializer-config"
	defaultNamespace  = "default"
)

var (
	annotation string
	configmap  string
	namespace  string
)

func main() {
	flag.StringVar(&annotation, "annotation", defaultAnnotation, "The annotation to trigger initialization")
	flag.StringVar(&configmap, "configmap", defaultConfigmap, "The environ initializer's configmap")
	flag.StringVar(&namespace, "namespace", defaultNamespace, "The namespace where the configmap lives")
	flag.Parse()

	log.Println("Starting environ-initializer...")

	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	// load configuration from a Kubernetes ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(configmap, metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	// watch uninitialized Deployments in all namespaces.
	restClient := clientset.AppsV1beta1().RESTClient()
	watchlist := cache.NewListWatchFromClient(restClient, "deployments", corev1.NamespaceAll, fields.Everything())

	// wrap the returned watchlist to workaround the inability to include
	// the `IncludeUninitialized` list option when setting up watch clients.
	includeUninitializedWatchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return watchlist.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return watchlist.Watch(options)
		},
	}

	resyncPeriod := 30 * time.Second

	_, controller := cache.NewInformer(includeUninitializedWatchlist, &v1beta1.Deployment{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {

				deployment := obj.(*v1beta1.Deployment)
				deploymentCopyObj, err := runtime.NewScheme().DeepCopy(deployment)
				if err != nil {
					panic(err.Error())
				}
				deploymentCopy := deploymentCopyObj.(*v1beta1.Deployment)

				// check deployment for the trigger annotation
				annotations := deployment.ObjectMeta.GetAnnotations()
				annotationValue, hasAnnotation := annotations[annotation]

				if hasAnnotation {
					log.Printf("Annotation '%v' found on deployment '%v'", annotation, deployment.Name)
					log.Printf("Annotation value: '%v'", annotationValue)

					// examine the value of the annotation to determine which environments to patch in
					var aEnvs map[string][]string
					err := json.Unmarshal([]byte(annotationValue), &aEnvs)
					if err != nil {
						log.Printf("Error: could not unmarshal json: %v", annotationValue)
					}

					// inject the env vars into the existing containers.
					containersCopy := append([]corev1.Container(nil), deployment.Spec.Template.Spec.Containers...)
					for env := range cm.Data {
						for _, e := range aEnvs["environments"] {
							if env == e {
								var envVars map[string][]corev1.EnvVar
								err := yaml.Unmarshal([]byte(cm.Data[env]), &envVars)
								if err != nil {
									log.Printf("Error: could not unmarshal data from configmap: %v", cm.Data[env])
								}
								for i := range containersCopy {
									containersCopy[i].Env = append(containersCopy[i].Env, envVars["envVars"]...)
								}
							}
						}
					}
					deploymentCopy.Spec.Template.Spec.Containers = containersCopy

					oldData, err := json.Marshal(deployment)
					if err != nil {
						panic(err.Error())
					}

					newData, err := json.Marshal(deploymentCopy)
					if err != nil {
						panic(err.Error())
					}

					// patch the original deployment.
					patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1beta1.Deployment{})
					if err != nil {
						panic(err.Error())
					}

					_, err = clientset.AppsV1beta1().Deployments(deployment.Namespace).Patch(deployment.Name, types.StrategicMergePatchType, patchBytes)
					if err != nil {
						panic(err.Error())
					}

					log.Printf("Deployment '%v' patched", deployment.Name)

				}
			},
		},
	)

	stop := make(chan struct{})
	go controller.Run(stop)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")
	close(stop)

}
