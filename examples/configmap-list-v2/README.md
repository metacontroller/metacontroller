## ConfigMap List v2

This example demonstrates the **Hook Version v2** capability where a **Namespaced parent** can access related objects from **other namespaces** via the `customize` hook.

The `ConfigMapList` controller (Namespaced) takes an **optional** `sourceNamespace` and `sourceLabels` in its spec. It uses a `customize` hook to request ConfigMaps:
- If `sourceNamespace` is specified: only ConfigMaps from that namespace are returned (a `v2` cross-namespace capability).
- If `sourceNamespace` is omitted: ConfigMaps from **ALL** namespaces matching the labelSelector are returned (a `v2` cluster-wide capability).

In **Hook Version v1**, a namespaced parent using a labelSelector was always silently restricted to its own namespace. **Hook Version v2** provides the flexibility to reach into specific other namespaces or across the entire cluster.

The `sync` hook then receives these ConfigMaps and creates a child ConfigMap in the *parent's namespace* that lists the `namespace/name` of all matching source ConfigMaps.

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
1. `source-ns` and `other-source-ns` namespaces with several ConfigMaps labeled `app: my-app`.
2. `target-ns` namespace with two `ConfigMapList` CRs:
   - `my-config-list`: searches only in `source-ns`.
   - `all-configs-list`: searches in **all** namespaces.

Metacontroller will call the `customize` hook for these objects.

Verify that the child ConfigMap for `my-config-list` contains ConfigMaps only from `source-ns`:

```sh
kubectl get cm -n target-ns my-config-list-list -o yaml
```

Verify that the child ConfigMap for `all-configs-list` contains ConfigMaps from **both** source namespaces:

```sh
kubectl get cm -n target-ns all-configs-list-list -o yaml
```

You should see:
```yaml
data:
  configmaps: |-
    other-source-ns/cm-3
    source-ns/cm-1
    source-ns/cm-2
```

Also check the status of the parent:
```sh
kubectl get configmaplist -n target-ns my-config-list
```
It should show `count: 2`.
