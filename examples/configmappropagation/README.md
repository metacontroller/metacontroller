## ConfigMap propagation

This is an example CompositeController that propagates a specified configmap to given namespaces. 
It uses `customize` hook to select ConfigMap for propagation. 
Please note that we ignore `labelSelector` setting it to empty one, to select related resources just by namespace/name.

Also, in `CompositeControler` we set `labelSelector`
```yaml
  parentResource:
    apiVersion: examples.metacontroller.io/v1alpha1
    resource: configmappropagations
    labelSelector:
      matchLabels:
        version: v1
```

to process only one of the `ConfigMapPropagation` CR's.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```
(or pass `v1beta1` for kubernetes 1.15 or older)

### Create an example configmap, several namespaces and ConfigMapPropagation custom resource

```sh
kubectl apply -f example-configmap.yaml
```

A ConfigMap will be created in every namespace mentioned in CR.spec.targetNamespaces: (`one`, `two`, `three`)

```console
$ kubectl get cm -n one settings
NAME       DATA   AGE
settings   2      2m
```
