## IndexedJob

This is an example CompositeController that's similar to Job,
except that each Pod gets assigned a unique index, similar to StatefulSet.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap indexedjob-controller -n metacontroller --from-file=sync.py
kubectl apply -f indexedjob-controller.yaml
```

### Create an IndexedJob

```sh
kubectl apply -f my-indexedjob.yaml
```

Each Pod created should print its index:

```console
$ kubectl logs print-index-2
2
```

### Failure Policy

Implementing `activeDeadlineSeconds` and `backoffLimit` is left as an exercise for the reader.
