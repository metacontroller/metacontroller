## Metacontroller

This is not an official Google product.

This is a prototype for an add-on that can be installed in any Kubernetes cluster to make it easier
to write and deploy custom controllers.

### Grant yourself cluster-admin

[known issue](https://cloud.google.com/container-engine/docs/role-based-access-control#defining_permissions_in_a_role)

```sh
kubectl create clusterrolebinding <user>-cluster-admin-binding --clusterrole=cluster-admin --user=<user>@<domain>
```

### Install metacontroller

```sh
kubectl create -f manifests/
```

## Examples

### Install fission.io and node.js environment

```sh
kubectl create -f examples/fission/
```

```sh
curl -Lo fission https://github.com/fission/fission/releases/download/nightly20170705/fission-cli-linux && chmod +x fission
export FISSION_URL=http://<external IP for fission/controller service>
./fission env create --name node --image fission/node-env
```

### BlueGreenDeployment

```sh
./fission function create --name bluegreen-sync --env node --code examples/bluegreen/bluegreen-sync.js
./fission route create --method POST --url /ctl.enisoc.com/bluegreendeployments/sync --function bluegreen-sync
kubectl create -f examples/bluegreen/bluegreen-controller.yaml
```

```sh
kubectl create -f examples/bluegreen/my-bluegreen.yaml
```

### CatSet

```sh
./fission function create --name catset-sync --env node --code examples/catset/catset-sync.js
./fission route create --method POST --url /ctl.enisoc.com/catsets/sync --function catset-sync
kubectl create -f examples/catset/catset-controller.yaml
```

```sh
kubectl create -f examples/catset/my-catset.yaml
```

### Initializer

For this example, you need a cluster with alpha features enabled (for initializers),
and it must be v1.6.11+, v1.7.7+, or v1.8.0+ so that the initializer can modify
certain Pod fields that normally are immutable.

```sh
./fission function create --name podhostname-init --env node --code examples/initializer/podhostname-init.js
./fission route create --method POST --url /ctl.enisoc.com/podhostname/init --function podhostname-init
kubectl create -f examples/initializer/podhostname-initializer.yaml
```

Create a Pod that still uses only the old annotations, rather than the fields.
Without the initializer, the Pod DNS would not work in v1.7+ because the annotations
no longer have any effect.

```sh
kubectl create -f examples/initializer/bad-pod.yaml
```

Verify that the initializer properly copied the annotations to fields:

```sh
kubectl get pod bad-pod -o yaml | grep -E 'hostname|subdomain'
```

## Build

```sh
go get -u k8s.io/code-generator/cmd/deepcopy-gen
dep ensure
make
```

## Contributing changes

* See [CONTRIBUTING.md](CONTRIBUTING.md)

## Licensing

* See [LICENSE](LICENSE)
