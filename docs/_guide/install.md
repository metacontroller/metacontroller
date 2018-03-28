---
title: Installation
classes: wide
toc: false
---
This page describes how to install Metacontroller, either to develop your own
controllers or just to run third-party controllers that depend on it.

## Prerequisites

* Kubernetes v1.8+ is recommended for the improved CRD support, especially
  garbage collection on custom resources.
* You should have `kubectl` available and configured to talk to the desired cluster.

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
# Create 'metacontroller' namespace, service account, and role/binding.
kubectl apply -f {{ site.repo_raw }}/manifests/metacontroller-rbac.yaml
# Create CRDs for Metacontroller APIs, and the Metacontroller Deployment.
kubectl apply -f {{ site.repo_raw }}/manifests/metacontroller.yaml
```

If you prefer to build and host your own images, please see the
[build instructions](/contrib/build/) in the contributor guide.

## Configuration

The Metacontroller server has a few settings that can be configured
with command-line flags (by editing the Metacontroller Deployment
in `manifests/metacontroller.yaml`):

| Flag | Description |
| ---- | ----------- |
| `-v` | Set the logging verbosity level (e.g. `-v=4`). Level 4 logs Metacontroller's interaction with the API server. Levels 5 and up additionally log details of Metacontroller's invocation of lambda hooks. See the [troubleshooting guide](/guide/troubleshooting/) for more. |
| `--discovery-interval` | How often to refresh discovery cache to pick up newly-installed resources (e.g. `--discovery-interval=10s`). |
| `--cache-flush-interval` | How often to flush local caches and relist objects from the API server (e.g. `--cache-flush-interval=30m`). |
