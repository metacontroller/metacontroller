## Secret propagation

This is an example CompositeController that propagates a speficied Secret to all namespaces matching specified label selector. It uses `customize` hook to select Secret for propagation and namespaces matching label selector.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl create configmap secret-propagation-controller -n metacontroller --from-file=sync.py
kubectl apply -f secret-propagation.yaml
```

### Create an example configmap, several namespaces and ConfigMapPropagation custom resource

```sh
kubectl apply -f example-secret.yaml
```

A Secret will be created in namespaces `alpha` and `beta`:

```console
$ kubectl get secret shareable -n alpha
NAME        TYPE     DATA   AGE
shareable   Opaque   2      8m56s
$ kubectl get secret shareable -n beta
NAME        TYPE     DATA   AGE
shareable   Opaque   2      9m25s
```

, but not `gamma` (as the last one does not have matching labels):

```console
$ kubectl get secret shareable -n gamma
Error from server (NotFound): secrets "shareable" not found
```