# Installation

This page describes how to install Metacontroller, either to develop your own
controllers or just to run third-party controllers that depend on it.

[[_TOC_]]

## Docker images
Images are hosted in two places:
* [dockerhub](https://hub.docker.com/r/metacontrollerio/metacontroller)
* [ghcr.io](https://github.com/metacontroller/metacontroller/pkgs/container/metacontroller)

Feel free to use whatever suits your need, they identical. Note - currently in `helm` charts the dockerhub one's are used. 

## Prerequisites

* Kubernetes v1.17+ (because of maintainability, e2e test suite might not cover all releases)
* You should have `kubectl` available and configured to talk to the desired cluster.

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

Metacontroller runs as a `StatefulSet` by default. Because it is stateless, you can
deploy it as a `Deployment` instead, which gives the scheduler more freedom to place
the Pods:

```sh
kubectl apply -k https://github.com/metacontroller/metacontroller/manifests/deployment
```

With Helm, use `--set kind=Deployment` instead (see
[helm-install.md](helm-install.md)).

Switching the kind of an existing installation cannot be done in place: delete the
current workload first (for example
`kubectl -n metacontroller delete statefulset/metacontroller`) and then apply the new
manifests, so that a StatefulSet and a Deployment do not run side by side.

If you prefer to build and host your own images, please see the
[build instructions](../contrib/build.md) in the contributor guide.

If your `kubectl` version does does not support `-k` flag, please
install resources mentioned in `manifests/production/kustomization.yaml`
one by one manually with `kubectl apply -f {{filename}}` command.


## Install Metacontroller using Helm

Alternatively, metacontroller can be [installed using an Helm chart](helm-install.md).

## Migrating from /GoogleCloudPlatform/metacontroller

As current version of metacontroller uses different name of the finalizer than GCP version (GCP - `metacontroller.app`,
current version - `metacontroller.io`) thus after installing `metacontroller` you might need to clean up old finalizers,
i.e. by running:

```shell
kubectl get <comma separated list of your resource types here> --no-headers --all-namespaces | awk '{print $2 " -n " $1}' | xargs -L1 -P 50 -r kubectl patch -p '{"metadata":{"finalizers": [null]}}' --type=merge
```
