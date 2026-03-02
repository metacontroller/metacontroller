# Update Strategy Example: OnDelete and Recreate

This example demonstrates the two non-rolling `ChildUpdateMethod` values
supported by Metacontroller's `CompositeController`:

| Method       | Behaviour                                                                                                                                                                                                                                      |
| ------------ | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| **OnDelete** | The child resource is **never automatically updated**. Metacontroller withholds the change until the child is deleted (by an operator or external process). On the next sync after deletion the child is recreated with the new desired state. |
| **Recreate** | When the desired state of a child changes, Metacontroller **immediately deletes** the existing child. On the next sync iteration the child is recreated with the new desired state.                                                            |

## Directory layout

```
update-strategy/
├── manifest/
│   ├── kustomization.yaml
│   └── update-strategy-controllers.yaml  # CompositeControllers, Deployment, Service, ConfigMap (sync hook)
├── v1/
│   ├── kustomization.yaml
│   └── crdv1.yaml                        # OnDeleteDemo and RecreateDemo CRDs
├── my-ondeletedemo.yaml                  # Sample OnDeleteDemo CR instance
├── my-recreatedemo.yaml                  # Sample RecreateDemo CR instance
├── test.sh                               # Integration test
└── README.md
```

## Custom Resource Definitions

Two CRDs are registered, each managed by its own `CompositeController`:

- **`ondeletedemos.ctl.example.com`** (`OnDeleteDemo`) — controller uses `OnDelete` update strategy.
- **`recreatedemos.ctl.example.com`** (`RecreateDemo`) — controller uses `Recreate` update strategy.

Each CR has a single `spec.configData` string field. The sync webhook creates a
child `ConfigMap` named `<instance-name>-data` whose `data.value` mirrors
`spec.configData`.

## Running the test

Prerequisites: a running Kubernetes cluster with Metacontroller already deployed,
and `kubectl` configured to access it.

```bash
# from this directory
chmod +x test.sh
./test.sh
```

The test:

1. Installs both CRDs and controllers.
2. Creates `odd-instance` (OnDeleteDemo) and `rcd-instance` (RecreateDemo) with `configData: v1`.
3. Verifies that both child ConfigMaps appear with `value=v1`.
4. Patches both parents to `configData: v2`.
5. **Recreate assertion** — waits for `rcd-instance-data` to be automatically
   updated to `value=v2` by Metacontroller.
6. **OnDelete assertion** — waits 10 seconds and asserts that `odd-instance-data`
   still has `value=v1` (i.e. the update was withheld).
7. Manually deletes `odd-instance-data` and verifies it is recreated with `value=v2`.

## How it works

The `CompositeController` spec for each strategy looks like this:

### OnDelete

```yaml
childResources:
  - apiVersion: v1
    resource: configmaps
    updateStrategy:
      method: OnDelete
```

When Metacontroller computes a diff and finds a change in the desired child,
it calls `childUpdateOnDelete` internally, which is a no-op — it simply logs
that the update is withheld and moves on. The child is only updated after it
has been deleted and the controller recreates it from scratch.

### Recreate

```yaml
childResources:
  - apiVersion: v1
    resource: configmaps
    updateStrategy:
      method: Recreate
```

When Metacontroller detects a change it calls `childUpdateRecreate`, which
deletes the existing child immediately. On the following sync the child is
created anew with the latest desired state.

## Cleanup

The test registers an `EXIT` trap that deletes all created resources
automatically. To clean up manually:

```bash
kubectl delete -f my-ondeletedemo.yaml
kubectl delete -f my-recreatedemo.yaml
kubectl delete -k manifest
kubectl delete -k v1
```
