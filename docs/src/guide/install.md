# Installation

This page describes how to install Metacontroller, either to develop your own
controllers or just to run third-party controllers that depend on it.

[[_TOC_]]

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

## Install Metacontroller using Kustomize

```sh
# Apply all set of production resources defined in kustomization.yaml in `production` directory .
kubectl apply -k https://github.com/metacontroller/metacontroller/manifests/production

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

## Install Metacontroller using Helm

Alternatively, metacontroller can be [installed using an Helm chart](helm-install.md).

## Migrating from /GoogleCloudPlatform/metacontroller

As current version of metacontroller uses different name of the finalizer than GCP version (GCP - `metacontroller.app`,
current version - `metacontroller.io`) thus after installing `metacontroller` you might need to clean up old finalizers,
i.e. by running:

```shell
kubectl get <comma separated list of your resource types here> --no-headers --all-namespaces | awk '{print $2 " -n " $1}' | xargs -L1 -P 50 -r kubectl patch -p '{"metadata":{"finalizers": [null]}}' --type=merge
```
