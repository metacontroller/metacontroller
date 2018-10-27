## Noop

This is an example DecoratorController returning a status

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap noop-controller -n metacontroller --from-file=sync.js
kubectl apply -f noop-controller.yaml
```

### Create a Noop

```sh
kubectl apply -f my-noop.yaml
```
