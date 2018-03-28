---
title: Concepts
permalink: /concepts/
---
This page provides some background on terms that are used throughout
the Metacontroller documentation.

## Kubernetes Concepts

These are some of the general [Kubernetes Concepts](https://kubernetes.io/docs/concepts/)
that are particularly relevant to Metacontroller.

### Resource

In the context of the [Kubernetes API][], a *resource* is a [REST][]-style
collection of [API objects][].
When writing controllers, it's important to understand the following terminology.

[Kubernetes API]: https://kubernetes.io/docs/concepts/overview/kubernetes-api/
[REST]: https://en.wikipedia.org/wiki/Representational_state_transfer
[API objects]: https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/

#### Resource Name

There are many ways to refer to a resource.
For example, you may have noticed that you can fetch ReplicaSets with
any of the following commands:

```sh
kubectl get rs          # short name
kubectl get replicaset  # singular name
kubectl get replicasets # plural name
```

When writing controllers, it's important to note that the *plural name*
is the canonical form when interacting with the REST API
(it's in the URL) and API discovery (entries are keyed by plural name).

So, whenever Metacontroller asks for a resource name, you should use the
canonical, lowercase, plural form (e.g. `replicasets`).

#### API Group

Each resource lives inside a particular [API group][], which helps different
API authors avoid name conflicts.
For example, you can have two resources with the same name as long as they are
in different API groups.

[API group]: https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-groups

#### API Version

Each API group has one or more available [API versions][].
It's important to note that Kubernetes API versions are [format versions][].
That is, each version is a different lens through which you can view objects in the collection,
but you'll see the same set of underlying objects no matter which lens you view them through.

The API group and version are often combined in the form `<group>/<version>`,
such as in the `apiVersion` field of an API object.
APIs in the *core* group (like Pod) omit the group name in such cases,
specifying only `<version>`.

[API versions]: https://kubernetes.io/docs/concepts/overview/kubernetes-api/#api-versioning
[format versions]: https://cloudplatform.googleblog.com/2018/03/API-design-which-version-of-versioning-is-right-for-you.html

#### API Kind

Whereas a *resource* is a collection of objects served at a particular REST path,
the *kind* of a resource represents something like the *type* or *class* of those
objects.

Since Kubernetes resources and kinds must have a 1-to-1 correspondence within
a given API group, the resource name and kind are often used interchangeably
in Kubernetes documentation.
However, it's important to distinguish the resource and kind when writing
controllers.

The kind is often the same as the singular resource name, except that it's
written in UpperCamelCase.
This is the form that you use when writing JSON or YAML manifests,
and so it's also the form you should use when generating objects within a
[lambda hook](#lambda-hook):

```yaml
apiVersion: apps/v1
kind: ReplicaSet
[...]
```

### Custom Resource

A [custom resource][] is any [resource](#resource) that's installed through
dynamic API registration (either through CRD or aggregation),
rather than by being compiled directly into the Kubernetes API server.

[custom resource]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/

### Controller

Distributed components in the Kubernetes control plane communicate with each
other by posting records in a shared datastore (like a public message board),
rather than sending direct messages (like email).

This design helps avoid silos of information. All participants can see what
everyone is saying to everyone else, so each participant can easily access
whatever information it needs to make the best decision, even as those needs change.
The lack of silos also means extensions have the same power as built-in features.

In the context of the Kubernetes control plane, a *controller* is a
long-running, automated, autonomous agent that participates in the
control plane via this shared datastore (the Kubernetes API server).
In the message board analogy, you can think of controllers like bots.

A given controller might participate by:

* observing objects in the API server as inputs and
  creating or updating other objects in the API server as outputs
  (e.g. creating Pods for a ReplicaSet);
* observing objects in the API server as inputs
  and taking action in some other domain
  (e.g. spawning containers for a Pod);
* creating or updating objects in the API server
  to report observations from some other domain
  (e.g. "the container is running");
* or any combination of the above.

### Custom Controller

A [custom controller][] is any [controller](#controller) that can be installed,
upgraded, and removed in a running cluster, independently of the cluster's own
lifecycle.

[custom controller]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers

## Metacontroller Concepts

These are some concepts that are specific to Metacontroller.

### Metacontroller

*Metacontroller* is a server that extends Kubernetes with APIs that encapsulate
the common parts of writing [custom controllers](#custom-controllers).

Just like [kube-controller-manager][], this server hosts multiple controllers.
However, the set of hosted controllers changes dynamically in response to
updates in objects of the Metacontroller API types.
Metacontroller is thus itself a controller that watches the Metacontroller API
objects and launches hosted controllers in response.
In other words, it's a controller-controller -- hence the name.

[kube-controller-manager]: https://kubernetes.io/docs/concepts/overview/components/#kube-controller-manager

### Lambda Controller

When you create a controller with one of the Metacontroller APIs, you provide
a function that contains only the business logic specific to your controller.
Since these functions are called via webhooks, you can write them in any
language that can understand HTTP and JSON, and optionally host them with
a Functions-as-a-Service provider.

The Metacontroller server then executes a control loop on your behalf,
calling your function whenever necessary to decide what to do.

These callback-based controllers are called *lambda controllers*.
To keep the interface as simple as possible, each lambda controller API targets
a specific controller pattern, such as:

* [CompositeController][]: *objects composed of other objects*
* [DecoratorController][]: *attach new behavior to existing objects*

Support for other types of controller patterns will be added in the future,
such as coordinating between Kubernetes API objects and external state
in another domain.

[CompositeController]: /api/compositecontroller/
[DecoratorController]: /api/decoratorcontroller/

### Lambda Hook

Each [lambda controller](#lambda-controller) API defines a set of hooks,
which it calls to let you implement your business logic.

Currently, these [lambda hooks](/api/hook/) must be implemented as webhooks,
but other mechanisms could be added in the future,
such as gRPC or embedded scripting languages.
