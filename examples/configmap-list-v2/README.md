## ConfigMap List v2

This example demonstrates the **Hook Version v2** capability where a **Namespaced parent** can access related objects from **other namespaces** via the `customize` hook.

The `ConfigMapList` controller (Namespaced) takes a `sourceNamespace` and `sourceLabels` in its spec. It uses a `customize` hook to request ConfigMaps from the `sourceNamespace`. The `sync` hook then receives these ConfigMaps and creates a child ConfigMap in the *parent's namespace* that lists the `namespace/name` of all matching source ConfigMaps.

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
1. `source-ns` namespace with several ConfigMaps labeled `app: my-app`.
2. `target-ns` namespace with a `ConfigMapList` CR named `my-config-list`.

Metacontroller will call the `customize` hook for `my-config-list` in `target-ns`, which will request ConfigMaps from `source-ns`.

Verify that the child ConfigMap in `target-ns` contains the list of ConfigMaps from `source-ns`:

```sh
kubectl get cm -n target-ns my-config-list-list -o yaml
```

You should see:
```yaml
data:
  configmaps: |-
    source-ns/cm-1
    source-ns/cm-2
```

Also check the status of the parent:
```sh
kubectl get configmaplist -n target-ns my-config-list
```
It should show `count: 2`.
