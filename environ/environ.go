package environ

import (
	"encoding/json"
	"log"

	"github.com/ghodss/yaml"

	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func Identify(annotation string, deployment *v1beta1.Deployment) (bool, string) {

	log.Printf("Checking deployment '%v' for annotation '%v'", deployment.Name, annotation)

	// check deployment for the trigger annotation
	annotations := deployment.ObjectMeta.GetAnnotations()
	annotationValue, hasAnnotation := annotations[annotation]

	return hasAnnotation, annotationValue
}

func Patch(annotationValue string, deployment *v1beta1.Deployment,
	cm *corev1.ConfigMap) (*v1beta1.Deployment, error) {

	log.Printf("Annotation value: '%v'", annotationValue)

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

	// inject the env vars into the existing containers.
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
