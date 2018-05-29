## CatSet

This is a reimplementation of StatefulSet (now including rolling updates) as a CompositeController.

For this example, you need a cluster with a default storage class and a dynamic provisioner.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap catset-controller -n metacontroller --from-file=sync.js
kubectl apply -f catset-controller.yaml
```

### Create a CatSet

```sh
kubectl apply -f my-catset.yaml
```
