## Metacontroller

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
./fission env create --name nodejs --image fission/node-env
```

### BlueGreenDeployment

```sh
./fission function create --name bluegreen-sync --env nodejs --code examples/bluegreen/bluegreen-sync.js
./fission route create --method POST --url /ctl.enisoc.com/bluegreendeployments/sync --function bluegreen-sync
kubectl create -f examples/bluegreen/bluegreen-controller.yaml
```

```sh
kubectl create -f examples/bluegreen/my-bluegreen.yaml
```

### CatSet

```sh
./fission function create --name catset-sync --env nodejs --code examples/catset/catset-sync.js
./fission route create --method POST --url /ctl.enisoc.com/catsets/sync --function catset-sync
kubectl create -f examples/catset/catset-controller.yaml
```

```sh
kubectl create -f examples/catset/my-catset.yaml
```

## Build

```sh
dep ensure
make
```
