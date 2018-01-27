package tests

import (
	"testing"

	"github.com/lander2k2/environ-initializer/environ"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIdentify(t *testing.T) {

	testAnnotation := "test.annotation"
	testVal := "testVal"

	matchDeploy := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-a",
			Annotations: map[string]string{
				testAnnotation: testVal,
			},
		},
		Spec: appsv1beta1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"role": "test",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "test-a",
							Image: "quay.io/lander2k2/crashcart",
						},
					},
				},
			},
		},
	}

	hasAnnotation, annotationValue := environ.Identify(testAnnotation, matchDeploy)
	if hasAnnotation != true {
		t.Errorf("environ.Identify failed to identify annotation")
	}
	if annotationValue != testVal {
		t.Errorf("environ.Identify failed to return correct annotation value")
	}

	nomatchDeploy := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-b",
			Annotations: map[string]string{
				"foo": "bar",
			},
		},
		Spec: appsv1beta1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"role": "test",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "test-b",
							Image: "quay.io/lander2k2/crashcart",
						},
					},
				},
			},
		},
	}

	noHasAnnotation, _ := environ.Identify(testAnnotation, nomatchDeploy)
	if noHasAnnotation != false {
		t.Errorf("environ.Identify identified wrong annotation")
	}
}

func TestPatch(t *testing.T) {

	annotationValue := "{\"environments\":[\"environ-x\", \"environ-y\"]}"

	unpatchedDeploy := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-c",
		},
		Spec: appsv1beta1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"role": "test",
					},
				},
				Spec: apiv1.PodSpec{
					Containers: []apiv1.Container{
						{
							Name:  "test-c",
							Image: "quay.io/lander2k2/crashcart",
						},
					},
				},
			},
		},
	}

	environConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "environ-initializer-config",
		},
		Data: map[string]string{
			"environ-x": "envVars:\n- name: XVAR\n  value: xval\n",
			"environ-y": "envVars:\n- name: YVAR\n  value: yval\n",
		},
	}

	patchedDeploy, err := environ.Patch(annotationValue, unpatchedDeploy, environConfig)
	if err != nil {
		t.Errorf("environ.Patch threw error: %v", err)
	}

	if len(patchedDeploy.Spec.Template.Spec.Containers[0].Env) == 0 {
		t.Errorf("environ.Patch failed tp patch in environment vars")
	}
}
