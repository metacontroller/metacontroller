## Namespace List v2

This example demonstrates the **Hook Version v2** capability where a **Namespaced parent** can access **Cluster-scoped** related objects via the `customize` hook.

The `NamespaceList` controller (Namespaced) takes a `labelSelector` in its spec. It uses a `customize` hook to request `Namespace` objects matching the selector. The `sync` hook then receives these `Namespace` objects and creates a child ConfigMap in the *parent's namespace* that lists the names of all matching namespaces.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```

### Create example resources

```sh
kubectl apply -f example.yaml
```

The example creates:
1. `test-ns` namespace (with `example-controller: namespace-list` label) and a `NamespaceList` CR named `filtered-namespaces`.

Metacontroller will call the `customize` hook for `filtered-namespaces` in `test-ns`, which will request `Namespace` objects.

Verify that the child ConfigMap in `test-ns` contains the `test-ns` namespace:

```sh
kubectl get cm -n test-ns filtered-namespaces-list -o yaml
```

You should see `test-ns` in the `data.namespaces` field.

Also check the status of the parent:
```sh
kubectl get namespacelist -n test-ns filtered-namespaces
```
It should show a `count` of at least 1.
