# Examples

This page lists some examples of what you can make with Metacontroller.

If you'd like to add a link to another example that demonstrates a new
language or technique, please send a pull request against
[this document](https://www.github.com/metacontroller/metacontroller/tree/master/docs/src/examples.md).

[[_TOC_]]

## CompositeController

[CompositeController](./api/compositecontroller.md)
is an API provided by Metacontroller, designed to facilitate
custom controllers whose primary purpose is to manage a set of child objects
based on the desired state specified in a parent object.
Workload controllers like Deployment and StatefulSet are examples of existing
controllers that fit this pattern.

### CatSet (JavaScript)

[CatSet](https://www.github.com/metacontroller/metacontroller/tree/master/examples/catset) is a rewrite of
[StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/),
including rolling updates, as a CompositeController.
It shows that existing workload controllers already use a pattern that could
fit within a CompositeController, namely managing child objects based on a
parent spec.

### BlueGreenDeployment (JavaScript)

[BlueGreenDeployment](https://www.github.com/metacontroller/metacontroller/tree/master/examples/bluegreen)
is an alternative to [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
that implements a [Blue-Green](https://martinfowler.com/bliki/BlueGreenDeployment.html)
rollout strategy.
It shows how CompositeController can be used to add various automation on top
of built-in APIs like ReplicaSet.

### IndexedJob (Python)

[IndexedJob](https://www.github.com/metacontroller/metacontroller/tree/master/examples/indexedjob)
is an alternative to [Job](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/)
that gives each Pod a unique index, like StatefulSet.
It shows how to write a CompositeController in Python, and also demonstrates
[selector generation](./api/compositecontroller.md#generate-selector).

## DecoratorController

[DecoratorController](./api/decoratorcontroller.md)
is an API provided by Metacontroller, designed to facilitate
adding new behavior to existing resources. You can define rules for which
resources to watch, as well as filters on labels and annotations.

For each object you watch, you can add, edit, or remove labels and annotations,
as well as create new objects and attach them. Unlike CompositeController,
these new objects don't have to match the main object's label selector.
Since they're attached to the main object, they'll be cleaned up automatically
when the main object is deleted.

### Service Per Pod (Jsonnet)

[Service Per Pod](https://www.github.com/metacontroller/metacontroller/tree/master/examples/service-per-pod)
is an example DecoratorController that creates an individual Service for
every Pod in a StatefulSet (e.g. to give them static IPs), effectively adding
new behavior to StatefulSet without having to reimplement it.

## Customize hook examples
[Customize hook](./api/customize.md) is addition to Composite/Decorator controllers, extending information given in `sync` hook of other objects (called `related`) in addition to parent.

### ConfigMapPropagation

[ConfigMapPropagation](https://www.github.com/metacontroller/metacontroller/tree/master/examples/configmappropagation) is
a simple mechanizm to propagate given `ConfigMap` to other namespaces, specified in given objects. Source `ConfigMap` is also specifcied.


### Global Config Map

[Global Config Map](https://www.github.com/metacontroller/metacontroller/tree/master/examples/globalconfigmap) is similar to `ConfigMapPropagation`. but populates `ConfigMap` to all namespaces.

### Secret propagation

[Secret propagation](https://www.github.com/metacontroller/metacontroller/tree/master/examples/secretpropagation) is modyfication of `ConfigMapPropagation` concept, 
using label selector on `Namespace` object to choose where to
propagate `Secret`.
