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

[controllers]: /concepts/#controller
[catset]: /examples/#catset-javascript
[operator]: https://coreos.com/operators/

### What is Metacontroller not good for?

Metacontroller is currently not a good fit for "bridging" other APIs
(like a cloud provider or app-specific API) into the Kubernetes API,
for example by representing it as a CRD and doing 2-way reconciliation.
If something changes in the external system, Metacontroller won't know
that it needs to call your hook again unless you set it to [poll][resync period].

That's because the initial controller patterns we support are focused on use
cases where both the inputs and outputs of your controller can be expressed as
Kubernetes API objects (either built-in or custom).
If you have a concrete use case that involves reconciling external state,
we'd appreciate if you [file an issue][issues] describing it so we can
work on defining additional patterns.

[resync period]: /api/compositecontroller/#resync-period
[issues]: {{ site.repo_url }}/issues

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
