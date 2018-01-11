# Environment Initializer

This Kubernetes initializer will load environment variables into a deployment's
containers.  The initializer is triggered by annotations applied to the desired
deployments and adds env vars specified in a configmap.

## Example Use Case
K8s cluster is installed behind an egress HTTP proxy so you need to load common
environment variables into many deployments for them to access the public internet.

