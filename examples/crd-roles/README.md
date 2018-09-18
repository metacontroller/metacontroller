## CRD Roles Controller

This is an example DecoratorController that manages Cluster scoped resources.
Both the parent and child resouces are Cluster scoped.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap crd-role-contoller -n metacontroller --from-file=sync.py
kubectl apply -f crd-role-controller.yaml
```

### Create a CRD

```sh
kubectl apply -f my-crd.yaml
```

A ClusterRole should be created configured with read access to the CRD.

```console
$ kubectl get clusterrole my-crd-reader
NAME            AGE
my-crd-reader   3m
```
