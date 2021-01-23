# Installation

This page describes how to install Metacontroller, either to develop your own
controllers or just to run third-party controllers that depend on it.

## Prerequisites

* Kubernetes v1.11+
* You should have `kubectl` available and configured to talk to the desired cluster.

### Running on kubernetes older than 1.16

As metacontroller does not have direct dependency on kubernetes API
it have wide range of supported kubernetes versions.
However, to be able to install it on clusters older than 1.16, CRD with schema in version `v1beta1` must be used then.

### Grant yourself cluster-admin (GKE only)

Due to a [known issue](https://cloud.google.com/container-engine/docs/role-based-access-control#defining_permissions_in_a_role)
in GKE, you'll need to first grant yourself `cluster-admin` privileges before
you can install the necessary RBAC manifests.

```sh
kubectl create clusterrolebinding <user>-cluster-admin-binding --clusterrole=cluster-admin --user=<user>@<domain>
```

Replace `<user>` and `<domain>` above based on the account you use to authenticate to GKE.

## Install Metacontroller

```sh
# Apply all set of production resources defined in kustomization.yaml in `production` directory .
kubectl apply -k {{ site.repo_raw }}/manifests/production

```

If you prefer to build and host your own images, please see the
[build instructions](../contrib/build.md) in the contributor guide.

If your `kubectl` version does does not support `-k` flag, please 
install resources mentioned in `manifests/production/kustomization.yaml`
one by one manually with `kubectl apply -f {{filename}}` command.

**Compatibility note**
CRD's are shipped in two versions:
* `v1` - supposed to be used when your kubernetes cluster is 1.16+
* `v1beta1` otherwise

## Configuration

The Metacontroller server has a few settings that can be configured
with command-line flags (by editing the Metacontroller StatefulSet
in `manifests/metacontroller.yaml`):

| Flag | Description |
| ---- | ----------- |
| `-v` | Set the logging verbosity level (e.g. `-v=4`). Level 4 logs Metacontroller's interaction with the API server. Levels 5 and up additionally log details of Metacontroller's invocation of lambda hooks. See the [troubleshooting guide](./troubleshooting.md) for more. |
| `--discovery-interval` | How often to refresh discovery cache to pick up newly-installed resources (e.g. `--discovery-interval=10s`). |
| `--cache-flush-interval` | How often to flush local caches and relist objects from the API server (e.g. `--cache-flush-interval=30m`). |
| `--client-config-path` | Path to kubeconfig file (same format as used by kubectl); if not specified, use in-cluster config (e.g. `--client-config-path=/path/to/kubeconfig`). |
| `--client-go-qps` | Number of queries per second client-go is allowed to make (default 5, e.g. `--client-go-qps=100`) |
| `--client-go-burst` |Allowed burst queries for client-go (default 10, e.g. `--client-go-burst=200`) |
| `--workers` | Number of sync workers to run (default 5, e.g. `--workers=100`) |
