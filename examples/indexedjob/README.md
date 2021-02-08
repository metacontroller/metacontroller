## IndexedJob

This is an example CompositeController that's similar to Job,
except that each Pod gets assigned a unique index, similar to StatefulSet.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```
(or pass `v1beta1` for kubernetes 1.15 or older)

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
