## Service-Per-Pod Decorator

This is an example DecoratorController that adds a Service for each Pod in a
StatefulSet, for any StatefulSet that requests this by adding an annotation
that specifies the name of the label containing the Pod name.

In Kubernetes 1.9+, StatefulSet automatically adds the Pod name as a label on
each of its Pods, so you can enable Service-Per-Pod like this:

```yaml
apiVersion: apps/v1beta2
kind: StatefulSet
metadata:
  annotations:
    service-per-pod-label: "statefulset.kubernetes.io/pod-name"
    service-per-pod-ports: "80:8080"
...
```

For earlier versions, this example also contains a second DecoratorController
that adds the Pod name label since StatefulSet previously didn't do it.

The Pod name label is only added to Pods that request it with an annotation,
which you can add in the StatefulSet's Pod template:

```yaml
apiVersion: apps/v1beta2
kind: StatefulSet
metadata:
  annotations:
    service-per-pod-label: "pod-name"
    service-per-pod-ports: "80:8080"
...
spec:
  template:
    metadata:
      annotations:
        pod-name-label: "pod-name"
...
```

### Prerequisites

* Kubernetes 1.8+ is recommended for its improved CRD support,
  especially garbage collection.
* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller).

### Deploy the DecoratorControllers

```sh
kubectl create configmap service-per-pod-hooks -n metacontroller --from-file=hooks
kubectl apply -f service-per-pod.yaml
```

### Create an Example StatefulSet

```sh
kubectl apply -f my-statefulset.yaml
```

Watch for the Services to get created:

```sh
kubectl get services --watch
```

Check that the StatefulSet's Pods can be selected by `pod-name` label:

```sh
kubectl get pod -l pod-name=nginx-0
kubectl get pod -l pod-name=nginx-1
kubectl get pod -l pod-name=nginx-2
```

Check that the per-Pod Services get cleaned up when the StatefulSet is deleted:

```sh
kubectl delete -f my-statefulset.yaml
kubectl get services
```
