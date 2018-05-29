---
title: Features
permalink: /features/
---
This is a high-level overview of what Metacontroller provides
for Kubernetes controller authors.

## Dynamic Scripting

With Metacontroller's [hook-based design][], you can write [controllers][]
in any language while still taking advantage of the efficient machinery we
developed in Go for core controllers.

This makes Metacontroller especially useful for rapid development
of automation in dynamic scripting languages like Python or JavaScript,
although you're also free to use statically-typed languages like Go or Java.

To support fast ramp-up and iteration on your ideas,
Metacontroller makes it possible to write controllers with:

* No schema/IDL
* No generated code
* No library dependencies
* No container image build/push

[hook-based design]: /concepts/#lambda-controller
[controllers]: /concepts/#controller

## Controller Best Practices

Controllers you write with Metacontroller automatically behave like
first-class citizens out of the box, before you write any code.

All interaction with the Kubernetes API happens inside the Metacontroller
server in response to your instructions.
This allows Metacontroller to implement best practices learned from writing
core controllers without polluting your business logic.

Even the simplest [Hello, World][] example with Metacontroller
already takes care of:

* Label selectors (for defining flexible collections of objects)
* Orphan/adopt semantics (controller reference)
* Garbage collection (owner references for automatic cleanup)
* Watches (for low latency)
* Caching (shared informers/reflectors/listers)
* Work queues (deduplicated parallelism)
* Optimistic concurrency (resource version)
* Retries with exponential backoff
* Periodic relist/resync

[Hello, World]: /guide/create/

## Declarative Watches

Rather than writing boilerplate code for each type of [resource][]
you want to watch, you simply list those resources declaratively:

```yaml
childResources:
- apiVersion: v1
  resource: pods
- apiVersion: v1
  resource: persistentvolumeclaims
```

Behind the scenes, Metacontroller sets up watch streams that are shared across
all controllers that use Metacontroller.

That means, for example, that you can create as many [lambda controllers][]
as you want that watch Pods, and the API server will only need to send one Pod
watch stream (to Metacontroller itself).

Metacontroller then acts like a demultiplexer, determining which controllers will
care about a given event in the stream and triggering their hooks only as needed.

[resource]: /concepts/#resource
[lambda controllers]: /concepts/#lambda-controller

## Declarative Reconciliation

A large part of the expressiveness of the Kubernetes API is due to its focus on
declarative management of cluster state, which lets you directly specify an
end state without specifying how to get there.
Metacontroller expands on this philosophy, allowing you to define controllers
in terms of what they want without specifying how to get there.

Instead of thinking about imperative operations like create/read/update/delete,
you just generate a list of all the things you want to exist.
Based on the current cluster state, Metacontroller will then determine what
actions are required to move the cluster towards your desired state and
maintain it once its there.

Just like the built-in controllers, the reconciliation that Metacontroller
performs for you is [level-triggered][] so it's resilient to downtime
(missed events), yet optimized for low latency and low API load through shared
watches and caches.

However, the clear separation of *deciding what you want* (the hook you write)
from *running a low-latency, level-triggered reconciliation loop*
(what Metacontroller does for you) means you don't have to think about this.

[level-triggered]: https://hackernoon.com/level-triggering-and-reconciliation-in-kubernetes-1f17fe30333d

## Declarative Declarative Rolling Update

Another big contributor to the power of Kubernetes APIs like Deployment and
StatefulSet is the ability to declaratively specify gradual state transitions.
When you update your app's container image or configuration, for example, these
controllers will slowly roll out Pods with the new template and automatically
pause if things don't look right.

Under the hood, implementing gradual state transitions with level-triggered
reconcilation loops involves careful bookkeeping with auxilliary records,
which is why StatefulSet originally launched without rolling updates.
Metacontroller lets you easily build your own APIs that offer declarative
rolling updates without making you think about all this additional bookkeeping.

In fact, Metacontroller provides a declarative interface for configuring how
you want to implement declarative rolling updates in your controller
(*declarative declarative rolling update*),
so you don't have to write any code to take advantage of this feature.

For example, [adding support for rolling updates][catset update]
to a Metacontroller-based [rewrite of StatefulSet][catset]
looks essentially like this:

```diff
   childResources:
   - apiVersion: v1
     resource: pods
+    updateStrategy:
+      method: RollingRecreate
+      statusChecks:
+        conditions:
+        - type: Ready
+          status: "True"
```

For comparison, the corresponding pull request to
[add rolling updates to StatefulSet itself][statefulset update] involved
over 9,000 lines of changes to business logic, boilerplate, and generated code.

[catset]: /examples/#catset-javascript
[catset update]: {{ site.repo_url }}/pull/22/files
[statefulset update]: https://github.com/kubernetes/kubernetes/pull/46669
