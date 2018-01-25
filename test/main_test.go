package tests

import (
	"flag"
	"fmt"
	"testing"
	"time"

	"github.com/franela/goblin"

	appsv1beta1 "k8s.io/api/apps/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1beta1 "k8s.io/api/rbac/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	kubeconfig *string
	repo       *string
)

func init() {
	kubeconfig = flag.String("kubeconfig", "", "absolute path to the kubeconfig file")
	repo = flag.String("repo", "", "repository to pull test image from")
	flag.Parse()
}

func Test(t *testing.T) {

	config, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		panic(err.Error())
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}

	g := goblin.Goblin(t)

	g.Describe("Initialized deployments...", func() {
		g.Before(func() {

			err = deployTargets(clientset)
			if err != nil {
				panic(err)
			}

			err = deploySA(clientset)
			if err != nil {
				panic(err)
			}

			err = deployClusterRole(clientset)
			if err != nil {
				panic(err)
			}

			err = deployClusterRoleBinding(clientset)
			if err != nil {
				panic(err)
			}

			err = deployConfigMap(clientset)
			if err != nil {
				panic(err)
			}

		})

		// check test env vars do not exist
		g.It("should not have any test env vars when deployed", func() {
			deploymentsClient := clientset.AppsV1beta1().Deployments(apiv1.NamespaceDefault)
			targetDeploys := []string{"target-a", "target-b"}
			var beginningVars []string

			for i := range targetDeploys {
				result, err := deploymentsClient.Get(targetDeploys[i], metav1.GetOptions{})
				if err != nil {
					panic(err)
				}
				containerList := result.Spec.Template.Spec.Containers
				for c := range containerList {
					envVars := containerList[c].Env
					for e := range envVars {
						beginningVars = append(beginningVars, envVars[e].Name)
					}
				}
			}

			g.Assert(len(beginningVars)).Equal(0)
		})

		g.It("should have correct env vars after initializer is deployed", func() {
			deploymentsClient := clientset.AppsV1beta1().Deployments(apiv1.NamespaceDefault)

			// pause to allow targets to come up before creating initializer
			time.Sleep(time.Second * 20)

			err := deployInitializer(clientset)
			if err != nil {
				panic(err)
			}

			// pause to allow initializer to come up before checking target env vars
			time.Sleep(time.Second * 20)

			var targetAVars []string
			targetA, err := deploymentsClient.Get("target-a", metav1.GetOptions{})
			if err != nil {
				panic(err)
			}
			containerList := targetA.Spec.Template.Spec.Containers
			for c := range containerList {
				envVars := containerList[c].Env
				for e := range envVars {
					targetAVars = append(targetAVars, envVars[e].Name)
				}
			}

			targetAHasVars := true
			expectedAVars := []string{"XVAR", "YVAR"}
			for _, v := range expectedAVars {
				hasVar := contains(targetAVars, v)
				if !hasVar {
					targetAHasVars = false
				}
			}

			g.Assert(targetAHasVars).IsTrue()

		})

		g.After(func() {
			deploymentsClient := clientset.AppsV1beta1().Deployments(apiv1.NamespaceDefault)
			configmapClient := clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault)
			serviceaccountClient := clientset.CoreV1().ServiceAccounts(apiv1.NamespaceDefault)
			clusterroleClient := clientset.RbacV1beta1().ClusterRoles()
			clusterrolebindingClient := clientset.RbacV1beta1().ClusterRoleBindings()
			deletePolicy := metav1.DeletePropagationForeground

			if err := deploymentsClient.Delete("target-a", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := deploymentsClient.Delete("target-b", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := serviceaccountClient.Delete("environ-initializer", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := clusterroleClient.Delete("initialize-deployments", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := clusterrolebindingClient.Delete("environ-initializer", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := configmapClient.Delete("environ-initializer-config", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}

			if err := deploymentsClient.Delete("environ-initializer", &metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			}); err != nil {
				panic(err)
			}
		})
	})
}

func deployTargets(clientset *kubernetes.Clientset) error {
	deploymentsClient := clientset.AppsV1beta1().Deployments(apiv1.NamespaceDefault)

	targetADeployment := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "target-a",
			Annotations: map[string]string{
				"initializers.kubernetes.io/environ": "{\"environments\":[\"environ-x\", \"environ-y\"]}",
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
							Name:  "target-a",
							Image: "quay.io/lander2k2/crashcart",
						},
					},
				},
			},
		},
	}

	targetBDeployment := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "target-b",
			Annotations: map[string]string{
				"initializers.kubernetes.io/environ": "{\"environments\":[\"environ-x\"]}",
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
							Name:  "target-b",
							Image: "quay.io/lander2k2/crashcart",
						},
					},
				},
			},
		},
	}

	_, errA := deploymentsClient.Create(targetADeployment)
	if errA != nil {
		return errA
	}

	_, errB := deploymentsClient.Create(targetBDeployment)
	if errB != nil {
		return errB
	}

	return nil
}

func deploySA(clientset *kubernetes.Clientset) error {
	serviceaccountClient := clientset.CoreV1().ServiceAccounts(apiv1.NamespaceDefault)

	environSA := &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name: "environ-initializer",
		},
	}
	_, err := serviceaccountClient.Create(environSA)
	if err != nil {
		return err
	}

	return nil
}

func deployClusterRole(clientset *kubernetes.Clientset) error {
	clusterroleClient := clientset.RbacV1beta1().ClusterRoles()

	configmapsRule := rbacv1beta1.PolicyRule{
		APIGroups: []string{""},
		Resources: []string{"configmaps"},
		Verbs:     []string{"get"},
	}
	deploymentsRule := rbacv1beta1.PolicyRule{
		APIGroups: []string{"apps"},
		Resources: []string{"deployments"},
		Verbs:     []string{"get", "list", "patch", "update", "watch"},
	}
	initClusterRole := &rbacv1beta1.ClusterRole{
		ObjectMeta: metav1.ObjectMeta{
			Name: "initialize-deployments",
		},
		Rules: []rbacv1beta1.PolicyRule{configmapsRule, deploymentsRule},
	}
	_, err := clusterroleClient.Create(initClusterRole)
	if err != nil {
		return err
	}

	return nil
}

func deployClusterRoleBinding(clientset *kubernetes.Clientset) error {
	clusterrolebindingClient := clientset.RbacV1beta1().ClusterRoleBindings()

	saSubject := rbacv1beta1.Subject{
		Kind:      "ServiceAccount",
		Name:      "environ-initializer",
		Namespace: "default",
	}
	initClusterRoleBinding := &rbacv1beta1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: "environ-initializer",
		},
		Subjects: []rbacv1beta1.Subject{saSubject},
		RoleRef: rbacv1beta1.RoleRef{
			APIGroup: "rbac.authorization.k8s.io",
			Kind:     "ClusterRole",
			Name:     "initialize-deployments",
		},
	}
	_, err := clusterrolebindingClient.Create(initClusterRoleBinding)
	if err != nil {
		return err
	}

	return nil
}

func deployConfigMap(clientset *kubernetes.Clientset) error {
	configmapClient := clientset.CoreV1().ConfigMaps(apiv1.NamespaceDefault)

	environConfig := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name: "environ-initializer-config",
		},
		Data: map[string]string{
			"environ-x": "envVars:\n- name: XVAR\n  value: xval\n",
			"environ-y": "envVars:\n- name: YVAR\n  value: yval\n",
		},
	}
	_, err := configmapClient.Create(environConfig)
	if err != nil {
		return err
	}

	return nil
}

func deployInitializer(clientset *kubernetes.Clientset) error {
	deploymentsClient := clientset.AppsV1beta1().Deployments(apiv1.NamespaceDefault)

	initDeployment := &appsv1beta1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "environ-initializer",
		},
		Spec: appsv1beta1.DeploymentSpec{
			Template: apiv1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"role": "initializer",
					},
				},
				Spec: apiv1.PodSpec{
					ServiceAccountName: "environ-initializer",
					Containers: []apiv1.Container{
						{
							Name:  "environ-initializer",
							Image: fmt.Sprintf("%v:test", *repo),
						},
					},
				},
			},
		},
	}
	_, err := deploymentsClient.Create(initDeployment)
	if err != nil {
		return err
	}

	return nil
}

func contains(s []string, v string) bool {
	for _, a := range s {
		if a == v {
			return true
		}
	}
	return false
}
