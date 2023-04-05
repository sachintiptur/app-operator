# GRPC Application Operator
A simple kubernetes operator to manage a simple grpc application created as part of simple-gRPC-app repository.
Manages simple gRPC application using new CRD `Application` which enables the users to configure their services.

# Application CRD Example

```
apiVersion: simpleapp.github.com/v1
kind: Application
metadata:
  labels:
    app.kubernetes.io/name: application
    app.kubernetes.io/instance: application-sample
    app.kubernetes.io/part-of: app-operator
    app.kubernetes.io/managed-by: kustomize
    app.kubernetes.io/created-by: app-operator
  name: grpc-app
spec:
  name: grpc-app
  services:
  - name: backend
    port: 9000
    type: "backend"
    numberofendpoints: 2
    image: ghcr.io/sachintiptur/backend
    version: latest
  - name: frontend
    port: 8080
    type: "frontend"
    numberofendpoints: 1
    image: ghcr.io/sachintiptur/frontend
    version: latest
  grpcserver: backend:9000

```

# Build and deploy instructions
1. Build docker image and push

`make docker-build docker-push IMG=<image name and tag>`

2. Deploy to cluster
```
make install
make deploy IMG=ghcr.io/sachintiptur/grpc-app-operator:latest
```
