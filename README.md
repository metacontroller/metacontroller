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

**Examples**

* [**CatSet**](examples/catset) (JavaScript)

  This is a rewrite of StatefulSet, minus rolling updates, as a LambdaController.
  It shows that existing workload controllers already use a pattern that could
  fit within a LambdaController, namely managing child objects based on a parent
  spec.

* [**BlueGreenDeployment**](examples/bluegreen) (JavaScript)

  This is an alternative to Deployment that implements a Blue-Green rollout strategy.
  It shows how LambdaController can be used to add various automation on top of
  built-in APIs like ReplicaSet.

* [**IndexedJob**](examples/indexedjob) (Python)

  This is an alternative to Job that gives each Pod a unique index, like StatefulSet.
  It shows how to write a LambdaController in Python, and also demonstrates selector generation.

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

**Examples**

* [**podhostname**](examples/initializer)

  This is an InitializerController that fixes Pods for compatibility with newer
  clusters.

## Install

### Prerequisites

* Kubernetes v1.8+
  * Most things will work in v1.7, except garbage collection on custom resources
    and mutating Pod initializers.
  * Initializers are alpha in v1.7-v1.8 and must be enabled by a feature flag on
    the API server in order to use InitializerControllers.

### Grant yourself cluster-admin

Due to a [known issue](https://cloud.google.com/container-engine/docs/role-based-access-control#defining_permissions_in_a_role)
in GKE, you will need to first grant yourself cluster-admin privileges before you can install the
necessary RBAC manifests.

```sh
kubectl create clusterrolebinding <user>-cluster-admin-binding --clusterrole=cluster-admin --user=<user>@<domain>
```

### Install metacontroller

```sh
kubectl apply -f manifests/
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
