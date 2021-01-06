## ConfigMap propagation

This is an example CompositeController that propagates a speficied configmap to given namespaces. It uses `customize` hook to select ConfigMap for propagation. Please note that we ignore `labelSelector` setting it to empty one, to select related resources just by namespace/name.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap configmap-propagation-controller -n metacontroller --from-file=sync.py
kubectl apply -f configmap-propagation.yaml
```

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
