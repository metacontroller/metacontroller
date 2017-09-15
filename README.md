## Metacontroller

This is not an official Google product.

This is a prototype for an add-on that can be installed in any Kubernetes cluster to make it easier
to write and deploy custom controllers.

Note: This code is in proof-of-concept, pre-alpha stage, and could bring down a cluster.
In particular, it currently "fakes" watches by polling, resulting in extreme traffic to the API
server. It should only be installed on a test cluster.

### LambdaController

LambdaController is an API provided by Metacontroller, designed to facilitate custom controllers
whose primary purpose is to manage a set of child objects based on the desired state specified
in a parent object.
Workload controllers like Deployment and StatefulSet are examples of existing controllers
that fit this pattern.

The Metacontroller will handle all the behaviors necessary to interact with the Kubernetes API,
including watches, label selectors, owner references, orphaning/adoption, optimistic concurrency,
and exponential back-off. Object caches will be shared among all controllers implemented via
Metacontroller, keeping the watch load on the API server low.

The only thing you need to write is the hook that takes as input the current state and outputs a
desired state, both of which are in the form of versioned JSON manifests representing Kubernetes
API objects.
The process is conceptually similar to writing a static generator or template for pre-processing
files to be sent to `kubectl`, except that Metacontroller turns it into a dynamic controller that
constantly maintains your desired state and reacts to any changes made to the parent object.

### InitializerController

InitializerController is another API provided by Metacontroller, designed to facilitate custom
controllers that implement [initializers](https://kubernetes.io/docs/admin/extensible-admission-controllers/#initializers).

Similar to LambdaControllers, the Metacontroller will handle watching uninitialized objects and
other interactions with the Kubernetes API. However, in this case the hook you implement is even
simpler. You just accept a single uninitialized object and return a modified form of it.

Metacontroller will only call your initializer hook when it's at the top of the Pending list,
and will automatically remove you from the Pending list upon success in a single, atomic update.
The watch stream will be shared among all custom controllers served through Metacontroller,
including both InitializerControllers and LambdaControllers.

## Examples

### Grant yourself cluster-admin

Due to a [known issue](https://cloud.google.com/container-engine/docs/role-based-access-control#defining_permissions_in_a_role)
in GKE, you will need to first grant yourself cluster-admin privileges before you can install the
necessary RBAC manifests.

```sh
kubectl create clusterrolebinding <user>-cluster-admin-binding --clusterrole=cluster-admin --user=<user>@<domain>
```

### Install metacontroller

```sh
kubectl create -f manifests/
```

### Install fission.io and node.js environment

These examples use [fission](http://fission.io/) to quickly deploy the simple code that corresponds
to each custom controller.
You could use any method you like to wrap the code up into an HTTP endpoint accessible by
the Metacontroller Pod.

```sh
kubectl create -f examples/fission/
```

```sh
curl -Lo fission https://github.com/fission/fission/releases/download/nightly20170705/fission-cli-linux && chmod +x fission
export FISSION_URL=http://<external IP for fission/controller service>
./fission env create --name node --image fission/node-env
```

### BlueGreenDeployment

This is an example LambdaController that implements a custom rollout strategy
based on a technique called Blue-Green Deployment.

The controller ramps up a completely separate ReplicaSet in the background for any change to the
Pod template. It then waits for the new ReplicaSet to be fully Ready and Available
(all Pods satisfy minReadySeconds), and then switches a Service to point to the new ReplicaSet.
Finally, it scales down the old ReplicaSet.

```sh
./fission function create --name bluegreen-sync --env node --code examples/bluegreen/bluegreen-sync.js
./fission route create --method POST --url /ctl.enisoc.com/bluegreendeployments/sync --function bluegreen-sync
kubectl create -f examples/bluegreen/bluegreen-controller.yaml
```

```sh
kubectl create -f examples/bluegreen/my-bluegreen.yaml
```

### CatSet

This is a reimplementation of StatefulSet (except for rolling updates) as a LambdaController.

For this example, you need a cluster with a default storage class and a dynamic provisioner.

```sh
./fission function create --name catset-sync --env node --code examples/catset/catset-sync.js
./fission route create --method POST --url /ctl.enisoc.com/catsets/sync --function catset-sync
kubectl create -f examples/catset/catset-controller.yaml
```

```sh
kubectl create -f examples/catset/my-catset.yaml
```

### Initializer

This is an example InitializerController that copies some deprecated Pod annotations to
the corresponding new fields.

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

If you want to build your own image for the Metacontroller Deployment,
you'll need the following prerequisites:

```sh
go get -u k8s.io/code-generator/cmd/deepcopy-gen
dep ensure
make
```

## Contributing changes

* See [CONTRIBUTING.md](CONTRIBUTING.md)

## Licensing

* See [LICENSE](LICENSE)
