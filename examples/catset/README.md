## CatSet

This is a reimplementation of StatefulSet (now including rolling updates) as a CompositeController.

CatSet also demonstrates using a finalizer with a lambda hook to support
graceful, ordered teardown when the parent object is deleted.
Unlike StatefulSet, which previously exhibited this behavior only because of a
client-side kubectl feature, CatSet ordered teardown happens on the server side,
so it works when the CatSet is deleted through any means (not just kubectl).

For this example, you need a cluster with a default storage class and a dynamic provisioner.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```
(or pass `v1beta1` for kubernetes 1.15 or older)

### Create a CatSet

```sh
kubectl apply -f my-catset.yaml
```
