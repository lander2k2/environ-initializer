package environ

import (
	"encoding/json"
	"log"
	"time"

	"github.com/ghodss/yaml"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
)

type DaemonSetResource struct {
	Clientset  *kubernetes.Clientset
	Configmap  *corev1.ConfigMap
	Annotation string
}

func (dsResource DaemonSetResource) getResourceController() cache.Controller {

	//dsRestClient := clientset.ExtensionsV1beta1().RESTClient()
	dsRestClient := dsResource.Clientset.ExtensionsV1beta1().RESTClient()
	dsWatchList := cache.NewListWatchFromClient(dsRestClient, "daemonsets", corev1.NamespaceAll, fields.Everything())

	// wrap the returned watchlist to workaround the inability to include
	// the `IncludeUninitialized` list option when setting up watch clients.
	dsIncludeUninitializedWatchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return dsWatchList.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return dsWatchList.Watch(options)
		},
	}

	resyncPeriod := 30 * time.Second

	_, dsController := cache.NewInformer(dsIncludeUninitializedWatchlist, &v1beta1.DaemonSet{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {

				// identify daemonsets that have annotation
				ds := obj.(*v1beta1.DaemonSet)
				hasAnnotation, annotationValue := identifyDaemonSet(dsResource.Annotation, ds)

				if hasAnnotation {
					log.Printf("Annotation '%v' found on daemonset '%v'", dsResource.Annotation, ds.Name)

					// generate daemonset patch
					patchedDaemonSet, err := patchDaemonSet(annotationValue, ds, dsResource.Configmap)
					if err != nil {
						log.Printf("Failed to generate patch for '%v': %v", ds.Name, err)
					}

					dsJson, err := json.Marshal(ds)
					if err != nil {
						log.Printf("Failed to marshal daemonset '%v' JSON data: %v", ds.Name, err)
					}

					patchedDaemonSetJson, err := json.Marshal(patchedDaemonSet)
					if err != nil {
						log.Printf("Failed to marshal patched daemonset '%v' JSON data: %v", ds.Name, err)
					}

					// patch the original daemonset
					patchBytes, err := strategicpatch.CreateTwoWayMergePatch(dsJson, patchedDaemonSetJson, v1beta1.DaemonSet{})
					if err != nil {
						log.Printf("Failed to patch %v: %v", ds.Name, err)
					}

					_, err = dsResource.Clientset.ExtensionsV1beta1().DaemonSets(ds.Namespace).Patch(ds.Name, types.StrategicMergePatchType, patchBytes)
					if err != nil {
						log.Printf("Failed to initialize %v: %v", ds.Name, err)
					}

					log.Printf("Deployment '%v' patched", ds.Name)
				}
			},
		},
	)

	return dsController

}

func identifyDaemonSet(annotation string, daemonset *v1beta1.DaemonSet) (bool, string) {

	log.Printf("Checking DaemonSet '%v' for annotation '%v'", daemonset.Name, annotation)

	// check deployment for the trigger annotation
	annotations := daemonset.ObjectMeta.GetAnnotations()
	annotationValue, hasAnnotation := annotations[annotation]

	return hasAnnotation, annotationValue
}

func patchDaemonSet(annotationValue string, daemonset *v1beta1.DaemonSet,
	cm *corev1.ConfigMap) (*v1beta1.DaemonSet, error) {

	log.Printf("Annotation value: '%v'", annotationValue)

	// make a copy of the daemonset
	daemonsetCopyObj, err := runtime.NewScheme().DeepCopy(daemonset)
	if err != nil {
		log.Printf("Failed to copy daemonset '%v'", daemonset)
		return daemonset, err
	}
	daemonsetCopy := daemonsetCopyObj.(*v1beta1.DaemonSet)

	// examine the value of the annotation to determine which environments to patch in
	var aEnvs map[string][]string
	uErr := json.Unmarshal([]byte(annotationValue), &aEnvs)
	if uErr != nil {
		log.Printf("Failed to unmarshal JSON for annotation value '%v' on daemonset '%v'", annotationValue, daemonset)
		return daemonset, uErr
	}

	// append the environment variables to the containers in the daemonset copy
	containersCopy := append([]corev1.Container(nil), daemonset.Spec.Template.Spec.Containers...)
	for env := range cm.Data {
		for _, e := range aEnvs["environments"] {
			if env == e {
				var envVars map[string][]corev1.EnvVar
				err := yaml.Unmarshal([]byte(cm.Data[env]), &envVars)
				if err != nil {
					log.Printf("Failed to unmarshal JSON data '%v' from initializer configmap", cm.Data[env])
					return daemonset, err
				}
				for i := range containersCopy {
					containersCopy[i].Env = append(containersCopy[i].Env, envVars["envVars"]...)
				}
			}
		}
	}
	daemonsetCopy.Spec.Template.Spec.Containers = containersCopy

	return daemonsetCopy, nil
}
