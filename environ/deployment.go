package environ

import (
	"encoding/json"
	"log"
	"time"

	"github.com/ghodss/yaml"

	"k8s.io/api/apps/v1beta1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type DeploymentResource struct {
	Clientset  *kubernetes.Clientset
	Configmap  *corev1.ConfigMap
	Annotation string
}

func (deployResource DeploymentResource) getResourceController() cache.Controller {

	//restClient := clientset.AppsV1beta1().RESTClient()
	restClient := deployResource.Clientset.AppsV1beta1().RESTClient()
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

	_, controller := cache.NewInformer(includeUninitializedWatchlist, &appsv1beta1.Deployment{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {

				// identify deployments that have annotation
				deploy := obj.(*appsv1beta1.Deployment)
				hasAnnotation, annotationValue := identifyDeployment(deployResource.Annotation, deploy)

				if hasAnnotation {
					log.Printf("Annotation '%v' found on deployment '%v'", deployResource.Annotation, deploy.Name)

					// generate deployment patch
					patchedDeploy, err := patchDeployment(annotationValue, deploy, deployResource.Configmap)
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

					// patch the original deployment
					patchBytes, err := strategicpatch.CreateTwoWayMergePatch(deployJson, patchedDeployJson, appsv1beta1.Deployment{})
					if err != nil {
						log.Printf("Failed to patch %v: %v", deploy.Name, err)
					}

					_, err = deployResource.Clientset.AppsV1beta1().Deployments(deploy.Namespace).Patch(deploy.Name, types.StrategicMergePatchType, patchBytes)
					if err != nil {
						log.Printf("Failed to initialize %v: %v", deploy.Name, err)
					}

					log.Printf("Deployment '%v' patched", deploy.Name)
				}
			},
		},
	)

	return controller

}

func identifyDeployment(annotation string, deployment *v1beta1.Deployment) (bool, string) {

	log.Printf("Checking Deployment '%v' for annotation '%v'", deployment.Name, annotation)

	// check deployment for the trigger annotation
	annotations := deployment.ObjectMeta.GetAnnotations()
	annotationValue, hasAnnotation := annotations[annotation]

	return hasAnnotation, annotationValue
}

func patchDeployment(annotationValue string, deployment *v1beta1.Deployment,
	cm *corev1.ConfigMap) (*v1beta1.Deployment, error) {

	log.Printf("Annotation value: '%v'", annotationValue)

	// make a copy of the deployment
	deploymentCopyObj, err := runtime.NewScheme().DeepCopy(deployment)
	if err != nil {
		log.Printf("Failed to copy deployment '%v'", deployment)
		return deployment, err
	}
	deploymentCopy := deploymentCopyObj.(*v1beta1.Deployment)

	// examine the value of the annotation to determine which environments to patch in
	var aEnvs map[string][]string
	uErr := json.Unmarshal([]byte(annotationValue), &aEnvs)
	if uErr != nil {
		log.Printf("Failed to unmarshal JSON for annotation value '%v' on deployment '%v'", annotationValue, deployment)
		return deployment, uErr
	}

	// append the environment variables to the containers in the deployment copy
	containersCopy := append([]corev1.Container(nil), deployment.Spec.Template.Spec.Containers...)
	for env := range cm.Data {
		for _, e := range aEnvs["environments"] {
			if env == e {
				var envVars map[string][]corev1.EnvVar
				err := yaml.Unmarshal([]byte(cm.Data[env]), &envVars)
				if err != nil {
					log.Printf("Failed to unmarshal JSON data '%v' from initializer configmap", cm.Data[env])
					return deployment, err
				}
				for i := range containersCopy {
					containersCopy[i].Env = append(containersCopy[i].Env, envVars["envVars"]...)
				}
			}
		}
	}
	deploymentCopy.Spec.Template.Spec.Containers = containersCopy

	return deploymentCopy, nil
}
