package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/lander2k2/environ-initializer/environ"

	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
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
		log.Fatal(err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(configmap, v1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	//deployController := environ.InitializeDeployment(clientset, annotation, cm)
	//dsController := environ.InitializeDaemonSet(clientset, annotation, cm)

	deploymentResource := environ.DeploymentResource{Clientset: *clientset, Configmap: *cm, Annotation: annotation}
	daemonsetResource := environ.DaemonSetResource{Clientset: *clientset, Configmap: *cm, Annotation: annotation}

	deployController := environ.Initialize(deploymentResource)
	dsController := environ.Initialize(daemonsetResource)

	stop := make(chan struct{})
	go deployController.Run(stop)
	go dsController.Run(stop)

	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-signalChan

	log.Println("Shutdown signal received, exiting...")
	close(stop)

}
