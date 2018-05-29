---
permalink: /
toc: false
classes: wide
---

# Introduction

Metacontroller is an add-on for [Kubernetes](https://kubernetes.io/)
that makes it easy to write and deploy [custom controllers](https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers).
Although the [open-source project]({{ site.repo_url }}) was started at Google,
the add-on works the same in any Kubernetes cluster.

While [custom resources](https://kubernetes.io/docs/concepts/api-extension/custom-resources/)
provide *storage* for new types of objects, custom controllers define the *behavior*
of a new extension to the [Kubernetes API](https://kubernetes.io/docs/concepts/overview/working-with-objects/kubernetes-objects/).
Just like the [CustomResourceDefinition](https://kubernetes.io/docs/tasks/access-kubernetes-api/extend-api-custom-resource-definitions/)
(CRD) API makes it easy to request storage for a custom resource,
the [Metacontroller APIs](/api/) make it easy to define behavior for a new extension API
or add custom behavior to existing APIs.

## Simple Automation

Kubernetes provides a lot of powerful automation through its built-in APIs,
but sometimes you just want to tweak one little thing or add a bit of logic on top.
With Metacontroller, you can write and deploy new [level-triggered](https://hackernoon.com/level-triggering-and-reconciliation-in-kubernetes-1f17fe30333d)
API logic in minutes.

The code for your custom controller could be as simple as this example in [Jsonnet](http://jsonnet.org/)
that [adds a label to Pods]({{ site.repo_dir }}/examples/service-per-pod):

```jsonnet
// This example is written in Jsonnet (a JSON templating language),
// but you can write hooks in any language.
function(request) {
  local pod = request.object,
  local labelKey = pod.metadata.annotations["pod-name-label"],

  // Inject the Pod name as a label with the key requested in the annotation.
  labels: {
    [labelKey]: pod.metadata.name
  }
}
```

Since all you need to provide is a webhook that understands [JSON](http://www.json.org/),
you can use any programming language, often without any dependencies beyond the standard library.
The code above is not a snippet; it's the entire script.

You can quickly deploy your code through any [FaaS](https://en.wikipedia.org/wiki/Function_as_a_service)
platform that offers HTTP(S) endpoints, or just [load your script into a ConfigMap]({{ site.repo_dir }}/examples/service-per-pod#deploy-the-decoratorcontrollers)
and launch a simple HTTP server to run it:

```sh
kubectl create configmap service-per-pod-hooks -n metacontroller --from-file=hooks
```

Finally, you declaratively specify how your script interacts with the Kubernetes API,
which is analogous to writing a CustomResourceDefinition (to specify how to store objects):

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: DecoratorController
metadata:
  name: pod-name-label
spec:
  resources:
  - apiVersion: v1
    resource: pods
    annotationSelector:
      matchExpressions:
      - {key: pod-name-label, operator: Exists}
  hooks:
    sync:
      webhook:
        url: http://service-per-pod.metacontroller/sync-pod-name-label
```

This declarative specification means that your code never has to talk to the Kubernetes API,
so you don't need to import any Kubernetes client library nor depend on any code provided by
Kubernetes.
You merely receive JSON describing the observed state of the world
and return JSON describing your desired state.

Metacontroller remotely handles all interaction with the Kubernetes API.
It runs a level-triggered reconciliation loop on your behalf, much the way
CRD provides a declarative interface to request that the API Server
store objects on your behalf.

## Reusable Building Blocks

In addition to making ad hoc automation simple, Metacontroller also makes it
easier to build and compose general-purpose abstractions.

For example, many built-in workload APIs like [StatefulSet](https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/)
are almost trivial to [reimplement as Metacontroller hooks]({{ site.repo_dir }}/examples/catset),
meaning you can easily fork and customize such APIs.
Feature requests that used to take months to implement in the core Kubernetes
repository can be hacked together in an afternoon by anyone who wants them.

You can also compose existing APIs into higher-level abstractions,
such as how [BlueGreenDeployment]({{ site.repo_dir }}/examples/bluegreen)
builds on top of the [ReplicaSet](https://kubernetes.io/docs/concepts/workloads/controllers/replicaset/)
and [Service](https://kubernetes.io/docs/concepts/services-networking/service/) APIs.

Users can even invent new general-purpose APIs like [IndexedJob]({{ site.repo_dir }}/examples/indexedjob),
which is a Job-like API that provides unique Pod identities like StatefulSet.

## Complex Orchestration

Extension APIs implemented with Metacontroller can also build on top of other
extension APIs that are themselves implemented with Metacontroller.
This pattern can be used to compose complex orchestration out of
simple building blocks that each do one thing well.

For example, the [Vitess Operator]({{ site.repo_dir }}/examples/vitess)
is implemented entirely as Jsonnet webhooks with Metacontroller.
The end result is much more complex than ad hoc automation or even
general-purpose workload abstractions, but the key is that this complexity
arises solely from the challenge of orchestrating [Vitess](https://vitess.io),
a distributed MySQL clustering system.

Building [Operators](https://coreos.com/operators/) with Metacontroller
frees developers from learning the internal machinery of implementing
Kubernetes controllers and APIs, allowing them to focus on solving
problems in the application domain.
It also means they can take advantage of existing API machinery like
shared caches without having to write their Operators in Go.

Metacontroller's webhook APIs are designed to make it feel like you're
writing a one-shot, client-side generator that spits out JSON that gets
piped to [`kubectl apply`](https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/#kubectl-apply).

In other words, if you already know how to manually manage an application
in Kubernetes with `kubectl`, Metacontroller lets you write automation for
that app without having to learn a new language or how to use Kubernetes
client libraries.

## Get Started

* [Install Metacontroller](/guide/install/)
* [Learn concepts](/concepts/)
* [See examples](/examples/)
* [Create a controller](/guide/create/)
* Give feedback by filing [GitHub issues]({{ site.repo_url }}/issues).
* [Contribute](/contrib/)!