# Environment Initializer

This Kubernetes initializer will load environment variables into a deployment's
containers.  The initializer is triggered by annotations applied to the desired
deployments and adds env vars specified in a configmap.

## Example Use Case

K8s cluster is installed behind an egress HTTP proxy so you need to load common
environment variables into many deployments for them to access the public internet.

## Example Usage

### Prerequisite

A Kubernetes cluster with [initializers enabled](https://kubernetes.io/docs/admin/extensible-admission-controllers/#enable-initializers-alpha-feature)

### Sample Workloads

Deploy two sample workloads into the default namespace.  The image in this example is just an Ubuntu image that sleeps.  The containers in these deployments will have env vars added to them by the environ-initializer after they are deployed.  The trigger for the initializer is the `initializers.kubernetes.io/environ` annotation.  The value of the annotation specifies which env vars are added.

    $ kubectl apply -f examples/sample-proxy-deploy.yaml
    $ kubectl apply -f examples/sample-all-deploy.yaml

### Examine Environments

Take a look at the environment variables before initialization.

    $ ALL_POD=$(kubectl get po -o name | grep all | awk -F'/' '{print $2}')
    $ kubectl exec $ALL_POD env
    $ PROXY_POD=$(kubectl get po -o name | grep proxy | awk -F'/' '{print $2}')
    $ kubectl exec $PROXY_POD env

### Create Namespace

Create a namespace for the initializer.

	$ kubectl create ns cluster-addons

### Deploy RBAC

If you have [RBAC enabled](https://kubernetes.io/docs/admin/authorization/rbac/) in your cluster give your initializer the needed permissions.

	$ kubectl apply -f examples/environ-initializer-rbac.yaml

### Deploy Configuration

The configmap for environ-initizlizer defines the different environments, the variables and values for each.  In this example there are two environments: 1) `http-proxy` and 2) `service-x`.  You can inject the environment variables from one or more of the environments into your deployments' containers.

	$ kubectl apply -f examples/environ-initializer-config.yaml

### Deploy Initializer

Start the deployment and examine it's logs.

	$ kubectl apply -f examples/environ-initializer-deploy.yaml
	$ INIT_POD=$(kubectl get po -n cluster-addons -o name | awk -F'/' '{print $2}')
	$ kubectl logs -n cluster-addons $INIT_POD

### Re-examine Environments

You should now see the desired env vars where applicable.

    $ ALL_POD=$(kubectl get po -o name | grep all | awk -F'/' '{print $2}')
    $ kubectl exec $ALL_POD env
    $ PROXY_POD=$(kubectl get po -o name | grep proxy | awk -F'/' '{print $2}')
    $ kubectl exec $PROXY_POD env

## Options

    $ ./environ-initializer -h
    Usage of ./environ-initializer:
      -annotation string
            The annotation to trigger initialization (default "initializers.kubernetes.io/environ")
      -configmap string
            The environ initializer's configmap (default "environ-initializer-config")
      -namespace string
            The namespace where the configmap lives (default "default")

## Build Instructions

Set environment variables `IMAGE_REPO` and `IMAGE_TAG`.

Build go binary: `$ make build-go`

Build container image: `$ make build-image`

Build go binary, container image and push to container registry: `$ make release`

## Tests

Run all tests: `$ make test`

### Unit Tests

Unit tests only: `$ make unit-test`

### End-to-End Tests

In addiont to the env vars needed for builds, set the `KUBECONFIG` environment variable for a cluster in which you can test.

E2E tests will use the image tag `test` instead of `$IMAGE_TAG`.

If you want to re-test an image that has already been pushed: `$ make e2e-test`

