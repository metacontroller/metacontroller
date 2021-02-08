## CRD Roles Controller

This is an example DecoratorController that manages Cluster scoped resources.
Both the parent and child resouces are Cluster scoped.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```
(or pass `v1beta1` for kubernetes 1.15 or older)

### Create a CRD

```sh
kubectl apply -f v1/my-crd.yaml
```
(or pass 'v1beta' directory for kubernetes 1.15 or older)


A ClusterRole should be created configured with read access to the CRD.

```console
$ kubectl get clusterrole my-crd-reader
NAME            AGE
my-crd-reader   3m
```
