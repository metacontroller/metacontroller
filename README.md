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

## Example

### Install fission.io

```sh
kubectl create -f examples/fission/
```

```sh
curl -Lo fission https://github.com/fission/fission/releases/download/nightly20170705/fission-cli-linux && chmod +x fission
```

### Install CatSet

```sh
export FISSION_URL=http://<external IP for fission/controller service>
./fission env create --name nodejs --image fission/node-env
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
