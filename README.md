## Metacontroller

This is not an official Google product.

This is an alpha-quality add-on that can be installed in any Kubernetes
cluster to make it easier to write and deploy custom controllers.

### Lambda Controllers

*Metacontroller* is a server that extends Kubernetes with APIs that encapsulate
the common parts of writing custom controllers.

When you create a controller with one of these APIs, you provide a function
that contains only the business logic specific to your controller.
Since these functions are called via webhooks, you can write them in any
language that can understand HTTP and JSON, and optionally host them with
a Functions-as-a-Service provider.

The Metacontroller server then executes a control loop on your behalf,
calling your function whenever necessary to decide what to do.

These callback-based custom controllers are called *Lambda Controllers*.
To keep the interface as simple as possible, each Lambda Controller API targets
a specific controller pattern, such as:

* CompositeController: *objects composed of other objects*
* DecoratorController: *attach new behavior to existing objects*

Support for other types of controller patterns will be added in the future.

#### CompositeController

CompositeController is an API provided by Metacontroller, designed to facilitate
custom controllers whose primary purpose is to manage a set of child objects
based on the desired state specified in a parent object.
Workload controllers like Deployment and StatefulSet are examples of existing
controllers that fit this pattern.

Metacontroller will handle all the behaviors necessary to interact with the
Kubernetes API, including watches, label selectors, owner references,
orphaning/adoption, optimistic concurrency, and exponential back-off.
Object caches will be shared among all controllers implemented via
Metacontroller, keeping the watch load on the API server low.

The only thing you need to write is the hook that takes as input the current
state and outputs a desired state, both of which are in the form of versioned
JSON manifests representing Kubernetes API objects.
The process is conceptually similar to writing a static generator or template
for pre-processing files to be sent to `kubectl`, except that Metacontroller
turns it into a dynamic controller that constantly maintains your desired state
and reacts to any changes made to the parent object.

**Examples**

* [**CatSet**](examples/catset) (JavaScript)

  This is a rewrite of StatefulSet, including rolling updates, as a
  CompositeController.
  It shows that existing workload controllers already use a pattern that could
  fit within a CompositeController, namely managing child objects based on a
  parent spec.

* [**BlueGreenDeployment**](examples/bluegreen) (JavaScript)

  This is an alternative to Deployment that implements a Blue-Green rollout
  strategy.
  It shows how CompositeController can be used to add various automation on top
  of built-in APIs like ReplicaSet.

* [**IndexedJob**](examples/indexedjob) (Python)

  This is an alternative to Job that gives each Pod a unique index, like
  StatefulSet.
  It shows how to write a CompositeController in Python, and also demonstrates
  selector generation.

* [**Vitess Operator**](examples/vitess) (Jsonnet)

  This is an example of using Metacontroller to write an Operator for a complex
  stateful application, in this case [Vitess](https://vitess.io).
  It shows how CompositeController can be layered to handle complex systems
  by breaking them down.
  
#### DecoratorController

DecoratorController is an API provided by Metacontroller, designed to facilitate
adding new behavior to existing resources. You can define rules for which
resources to watch, as well as filters on labels and annotations.

For each object you watch, you can add, edit, or remove labels and annotations,
as well as create new objects and attach them. Unlike CompositeController,
these new objects don't have to match the main object's label selector.
Since they're attached to the main object, they'll be cleaned up automatically
when the main object is deleted.

**Examples**

* [**Service Per Pod**](examples/service-per-pod) (Jsonnet)

  This is an example DecoratorController that creates an individual Service for
  every Pod in a StatefulSet (e.g. to give them static IPs), effectively adding
  new behavior to StatefulSet without having to reimplement it.

## Install

### Prerequisites

* Kubernetes v1.8+ is recommended for the improved CRD support, especially
  garbage collection on custom resources.

### Grant yourself cluster-admin

Due to a [known issue](https://cloud.google.com/container-engine/docs/role-based-access-control#defining_permissions_in_a_role)
in GKE, you will need to first grant yourself cluster-admin privileges before
you can install the necessary RBAC manifests.

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
go get -u k8s.io/code-generator/cmd/{lister,client,informer,deepcopy}-gen
dep ensure
make
```

## Contributing

* See [CONTRIBUTING.md](CONTRIBUTING.md)

## Licensing

* See [LICENSE](LICENSE)
