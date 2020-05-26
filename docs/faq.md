---
title: Frequently Asked Questions
classes: wide
permalink: /faq/
---
This page answers some common questions encountered while
evaluating, setting up, and using Metacontroller.

If you have any questions that aren't answered here,
please ask on the [mailing list](https://groups.google.com/forum/#!forum/metacontroller)
or [Slack channel](https://kubernetes.slack.com/messages/metacontroller/).

## Evaluating Metacontroller

### How does Metacontroller compare with other tools?

See the [features](/features/) page for a list of the things that are
most unique about Metacontroller's approach.

In general, Metacontroller aims to make common patterns as simple as possible,
without necessarily supporting the full flexibility you would have if you wrote
a controller from scratch.
The philosophy is analogous to that of [CustomResourceDefinition][crd] (CRD),
where the main API server does all the heavy lifting for you, but you don't have
as much control as you would if you wrote your own API server and connected it
through [aggregation][].

Just like CRD, Metacontroller started with a small set of capabilities and is
expanding over time to support more customization and more use cases as we gain
confidence in the abstractions.
Depending on your use case, you may prefer one of the alternative tools that
took the opposite approach of first allowing everything and then building
"rails" over time to encourage best practices and simplify development.

[crd]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#customresourcedefinitions
[aggregation]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#api-server-aggregation

### What is Metacontroller good for?

Metacontroller is intended to be a generic tool for creating many kinds of
Kubernetes [controllers][], but one of its earliest motivating use cases was to
simplify development of custom workload automation, so it's particularly
well-suited for this.

For example, if you've ever thought, "I wish StatefulSet would do this one
thing differently," Metacontroller gives you the tools to [define your own
custom behavior][catset] without reinventing the wheel.

Metacontroller is also well-suited to people who prefer languages other than
Go, but still want to benefit from the efficient API machinery that was
developed in Go for the core Kubernetes controllers.

Lastly, Metacontroller is good for rapid development of automation on top of
APIs that already exist as Kubernetes resources, such as:

* ad hoc scripting ("make an X for every Y")
* configuration abstraction ("when I say A, that means {X,Y,Z}")
* higher-level automation of custom APIs added by [Operators][operator]
* gluing an [external CRUD API][] into the Kubernetes control plane with a
  simple translation layer

[controllers]: /concepts/#controller
[catset]: /examples/#catset-javascript
[operator]: https://coreos.com/operators/
[external CRUD API]: #can-i-call-external-apis-from-my-hook

### What is Metacontroller not good for?

Metacontroller is not a good fit when you need to examine a large number of
objects to answer a single hook request.
For example, if you need to be sent a list of all Pods or all Nodes in order to
decide on your desired state, we'd have to call your hook with the full list of
all Pods or Nodes any time any one of them changed.
However, it might be a good fit if your desired behavior can be naturally
broken down into per-Pod or per-Node tasks, since then we'd only need to call
your hook with each object that changed.

Metacontroller is also not a good fit for writing controllers that perform long
sequences of imperative steps -- for example, a single hook that executes many
steps of a workflow by creating various children at the right times.
That's because Metacontroller hooks work best when they use a functional style
(no side effects, and output depends only on input), which is an awkward style
for defining imperative sequences.

### Do I have to use CRD?

It's common to use [CRD][], but Metacontroller doesn't know or care whether a
[resource][] is built-in or [custom][custom resource], nor whether it's served
by CRD or by an [aggregated API server][aggregation].

Metacontroller uses API discovery and the dynamic client to treat all resources
the same, so you can write automation for any type of resource.
Using the dynamic client also means Metacontroller doesn't need to be updated
when new APIs or fields are added in subsequent Kubernetes releases.

[resource]: /concepts/#resource
[custom resource]: /concepts/#custom-resource

### What does the name Metacontroller mean?

The name *Metacontroller* comes from the English words *meta* and *controller*.
Metacontroller is a *controller controller* --
a [controller](/concepts/#controller) that controls other controllers.

### How do you pronounce Metacontroller?

Please see the [pronunciation guide](/pronunciation/).

## Setting Up Metacontroller

### Do I need to be a cluster admin to install Metacontroller?

[Installing Metacontroller][install] requires permission to both install
[CRDs][crd] (representing the [Metacontroller APIs][api] themselves)
and grant permissions for Metacontroller to access other resources on
behalf of the controllers it hosts.

[install]: /guide/install/
[api]: /api/

### Why is Metacontroller shared cluster-wide?

Metacontroller currently only supports cluster-wide installation
because it's modeled after the built-in [kube-controller-manager][]
component to achieve the same benefits of sharing watches and caches.

Also, resources in general (either built-in or custom) can only be
installed cluster-wide, and a Kubernetes API object is conventionally
intended to mean the same thing regardless of what namespace it's in.

[kube-controller-manager]: https://kubernetes.io/docs/concepts/overview/components/#kube-controller-manager

### Why does Metacontroller need these permissions?

During alpha, Metacontroller simply requests wildcard permission to all
resources so the controllers it hosts can access anything they want.
For this reason, you should only give trusted users access to the
[Metacontroller APIs][api] that create hosted controllers.

By contrast, core controllers are restricted to only the minimal set of
permissions needed to do their jobs.
As part of the [beta roadmap][roadmap], we plan to offer per-controller
service accounts to mitigate the risks of confused deputy problems.

[roadmap]: {{ site.repo_url }}/issues/9

### Does Metacontroller have to be in its own namespace?

The default installation manifests put Metacontroller in its own namespace
to make it easy to see what's there and clean up if necessary,
but it can run anywhere.
The `metacontroller` namespace is also used in [examples][] for similar
convenience reasons, but you can run webhooks in any namespace
or even host them outside the cluster.

[examples]: /examples/

## Developing with Metacontroller

### Which languages can I write hooks in?

You can write [lambda hooks][] (the business logic for your controller)
in any language, as long as you can host it as a webhook that accepts
and returns JSON.
Regardless of which language you use for your business logic,
Metacontroller uses the efficient machinery written in Go for the
core controllers to interact with the API server on your behalf.

[lambda hooks]: /concepts/#lambda-hook

### How do I access the Kubernetes API from my hook?

You don't! Or at least, you don't have to, and it's best not to.
Instead, you just [declare what objects you care about][watches]
and Metacontroller will send them to you as part of the hook request.
Then, your hook should simply return a list of desired objects.
Metacontroller will take care of [reconciling your desired state][reconciling].

[watches]: /features/#declarative-watches
[reconciling]: /features/#declarative-reconcilitation

### Can I call external APIs from my hook?

Yes. Your hook code can do whatever it wants as part of computing a response to
a Metacontroller hook request, including calling external APIs.

The main thing to be careful of is to avoid synchronously waiting for
long-running tasks to finish, since that will hold up one of a fixed number of
concurrent slots in the queue of triggers for that hook.
Instead, if your hook needs to wait for some condition that's checked through an
external API, you should return a status that indicates this pending state,
and set a [resync period][] so you get a chance to check the condition again
later.

[resync period]: /api/compositecontroller/#resync-period

### How can I make sure external resources get cleaned up?

If you allocate external resources as part of your hook, you should also
implement a [finalize hook][] to make sure you get a chance to clean up those
external resources when the Kubernetes API object for which you created them
goes away.

[finalize hook]: /api/compositecontroller/#finalize-hook

### Does Metacontroller support "apply" semantics?

Yes, Metacontroller enforces [apply semantics][apply], which means your controller
will play nicely with other automation as long as you only fill in the fields
that you care about in the objects you return.

[apply]: /api/apply/

### How do I host my hook?

You can host your [lambda hooks][] with an HTTP server library in your chosen
language, with a standalone HTTP server, or with a Functions-as-a-Service platform.
See the [examples][] page for approaches in various languages.

### How can I provide a programmatic client for my API?

Since Metacontroller uses the dynamic client on your behalf, you can write your
controller's business logic without any client library at all.
That also means you can write a "dynamically typed" controller without creating
static schema (either Kubernetes' Go IDL or OpenAPI) or generating a client.

However, if you want to provide a static client for users of your API,
nothing about Metacontroller prevents you from writing Go IDL or OpenAPI
and generating a client the same way you would without Metacontroller.

### What are the best practices for designing controllers?

Please see the dedicated [best practices guide](/guide/best-practices/).

### How do I troubleshoot problems?

Please see the dedicated [troubleshooting guide](/guide/troubleshooting/).
