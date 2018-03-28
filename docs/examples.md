---
title: Examples
permalink: /examples/
---
This page lists some examples of what you can make with Metacontroller.

If you'd like to add a link to another example that demonstrates a new
language or technique, please send a pull request against
[this document]({{ site.repo_file }}/docs/examples.md).

## CompositeController

[CompositeController](/api/compositecontroller/)
is an API provided by Metacontroller, designed to facilitate
custom controllers whose primary purpose is to manage a set of child objects
based on the desired state specified in a parent object.
Workload controllers like Deployment and StatefulSet are examples of existing
controllers that fit this pattern.

### CatSet (JavaScript)

[CatSet]({{ site.repo_dir }}/examples/catset) is a rewrite of
[StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/),
including rolling updates, as a CompositeController.
It shows that existing workload controllers already use a pattern that could
fit within a CompositeController, namely managing child objects based on a
parent spec.

### BlueGreenDeployment (JavaScript)

[BlueGreenDeployment]({{ site.repo_dir }}/examples/bluegreen)
is an alternative to [Deployment](https://kubernetes.io/docs/concepts/workloads/controllers/deployment/)
that implements a [Blue-Green](https://martinfowler.com/bliki/BlueGreenDeployment.html)
rollout strategy.
It shows how CompositeController can be used to add various automation on top
of built-in APIs like ReplicaSet.

### IndexedJob (Python)

[IndexedJob]({{ site.repo_dir }}/examples/indexedjob)
is an alternative to [Job](https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/)
that gives each Pod a unique index, like StatefulSet.
It shows how to write a CompositeController in Python, and also demonstrates
[selector generation](/api/compositecontroller/#selector-generation).

### Vitess Operator (Jsonnet)

The [Vitess Operator]({{ site.repo_dir }}/examples/vitess)
is an example of using Metacontroller to write an Operator for a complex
stateful application, in this case [Vitess](https://vitess.io).
It shows how CompositeController can be layered to handle complex systems
by breaking them down.

## DecoratorController

[DecoratorController](/api/decoratorcontroller/)
is an API provided by Metacontroller, designed to facilitate
adding new behavior to existing resources. You can define rules for which
resources to watch, as well as filters on labels and annotations.

For each object you watch, you can add, edit, or remove labels and annotations,
as well as create new objects and attach them. Unlike CompositeController,
these new objects don't have to match the main object's label selector.
Since they're attached to the main object, they'll be cleaned up automatically
when the main object is deleted.

### Service Per Pod (Jsonnet)

[Service Per Pod]({{ site.repo_dir }}/examples/service-per-pod)
is an example DecoratorController that creates an individual Service for
every Pod in a StatefulSet (e.g. to give them static IPs), effectively adding
new behavior to StatefulSet without having to reimplement it.