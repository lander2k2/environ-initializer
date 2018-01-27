package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/lander2k2/environ-initializer/environ"

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

				// identify deployments that have annotation
				deploy := obj.(*v1beta1.Deployment)
				hasAnnotation, annotationValue := environ.Identify(annotation, deploy)

				if hasAnnotation {
					log.Printf("Annotation '%v' found on deployment '%v'", annotation, deploy.Name)

					// generate deployment patch
					patchedDeploy, err := environ.Patch(annotationValue, deploy, cm)
					if err != nil {
						log.Printf("Failed to generate patch for '%v': %v", deploy.Name, err)
					}

					deployJson, err := json.Marshal(deploy)
					if err != nil {
						log.Printf("Failed to marshal deployment '%v' JSON data: %v", deploy.Name, err)
					}

					patchedDeployJson, err := json.Marshal(patchedDeploy)
					if err != nil {
						log.Printf("Failed to marshal patched deployment '%v' JSON data: %v", deploy.Name, err)
					}

					// patch the original deployment.
					patchBytes, err := strategicpatch.CreateTwoWayMergePatch(deployJson, patchedDeployJson, v1beta1.Deployment{})
					if err != nil {
						log.Printf("Failed to patch %v: %v", deploy.Name, err)
					}

					_, err = clientset.AppsV1beta1().Deployments(deploy.Namespace).Patch(deploy.Name, types.StrategicMergePatchType, patchBytes)
					if err != nil {
						log.Printf("Failed to initialize %v: %v", deploy.Name, err)
					}

					log.Printf("Deployment '%v' patched", deploy.Name)
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
