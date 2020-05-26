## Vitess Operator

**NOTE: The [Vitess Operator][] has moved to its own repository,
and is now maintained by the Vitess project.**

[Vitess Operator]: https://github.com/vitessio/vitess-operator

This is an example of an app-specific [Operator](https://coreos.com/operators/), 
in this case for [Vitess](http://vitess.io), built with Metacontroller.

It's meant to demonstrate the following patterns:

* Building an Operator for a complex, stateful application out of a set of small
  Lambda Controllers that each do one thing well.
  * In addition to presenting a k8s-style API to users, this Operator uses
    custom k8s API objects to coordinate within itself.
  * Each controller manages one layer of the hierarchical Vitess cluster topology.
    The user only needs to create and manage a single, top-level VitessCluster
    object.
* Replacing static, client-side template rendering with Lambda Controllers,
  which can adjust based on dynamic cluster state.
  * Each controller aggregates status and orchestrates app-specific rolling
    updates for its immediate children.
  * The top-level object contains a continuously-updated, aggregate "Ready"
    condition for the whole app, and can be directly edited to trigger rolling
    updates throughout the app.
* Using a functional-style language ([Jsonnet](http://jsonnet.org)) to
  define Lambda Controllers in terms of template-like transformations on JSON
  objects.
  * You can use any language to write a Lambda Controller webhook, but the
    functional style is a good fit for a process that conceptually consists of
    declarative input, declarative output, and no side effects.
  * As a JSON templating language, Jsonnet is a particularly good fit for
    generating k8s manifests, providing functionality missing from pure
    JavaScript, such as first-class *merge* and *deep equal* operations.
* Using the "Apply" update strategy feature of CompositeController, which
  emulates the behavior of `kubectl apply`, except that it attempts to do
  pseudo-strategic merges for CRDs.

See the [Vitess Operator][] repository for details.
