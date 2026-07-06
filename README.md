![GitHub release (latest SemVer)](https://img.shields.io/github/v/release/metacontroller/metacontroller)
![GitHub Release Date](https://img.shields.io/github/release-date/metacontroller/metacontroller)
![GitHub](https://img.shields.io/github/license/metacontroller/metacontroller)
![Docker Image Size (latest semver)](https://img.shields.io/docker/image-size/metacontrollerio/metacontroller)
![Docker Pulls](https://img.shields.io/docker/pulls/metacontrollerio/metacontroller)
![GitHub contributors](https://img.shields.io/github/contributors/metacontroller/metacontroller)
[![Go Report Card](https://goreportcard.com/badge/github.com/metacontroller/metacontroller)](https://goreportcard.com/report/github.com/metacontroller/metacontroller)
[![codecov](https://codecov.io/gh/metacontroller/metacontroller/branch/master/graph/badge.svg?token=VU0L35J51Z)](https://codecov.io/gh/metacontroller/metacontroller)

# Metacontroller

Metacontroller is an add-on for Kubernetes that makes it easy to write and
deploy [custom controllers](https://kubernetes.io/docs/concepts/api-extension/custom-resources/#custom-controllers)
in the form of [simple scripts](https://metacontroller.github.io/metacontroller/).

Instead of writing and maintaining a full Go controller with client libraries,
informers, and boilerplate, you describe what resources to watch declaratively
and provide a webhook (in any language) that receives the current state as
JSON and returns the desired state as JSON. Metacontroller takes care of the
rest: watches, caching, work queues, retries, and level-triggered
reconciliation.

This is a continuation of great work started by [GKE](https://cloud.google.com/kubernetes-engine/)
[here](https://github.com/GoogleCloudPlatform/metacontroller). We are excited
to move forward with Metacontroller as a community maintained project. A big
thank you to all of the wonderful Metacontroller community members that made
this happen!

## Why Metacontroller?

* **Write controllers in any language** - all you need is a webhook that
  speaks JSON, so Python, JavaScript, Jsonnet, Go, or anything else works.
* **No boilerplate** - no schema/IDL, no generated code, no client library
  dependencies.
* **Production-ready behavior for free** - label selectors, adopt/orphan
  semantics, garbage collection, watches, caching, work queues, optimistic
  concurrency, and retries with backoff all come built in.
* **Build reusable abstractions** - compose existing Kubernetes APIs into
  higher-level operators, or reimplement APIs like `StatefulSet` as
  Metacontroller hooks.

See the [Features](https://metacontroller.github.io/metacontroller/features.html)
page for the full picture.

## Getting Started

* :book: Read the [documentation site](https://metacontroller.github.io/metacontroller/)
  for a full introduction, concepts, and API reference.
* :rocket: Follow the [install guide](https://metacontroller.github.io/metacontroller/guide/install.html)
  (via `kubectl apply -k` or [Helm](https://metacontroller.github.io/metacontroller/guide/helm-install.html)).
* :bulb: Browse [examples](https://metacontroller.github.io/metacontroller/examples.html)
  to see working controllers you can adapt.
* :hammer_and_wrench: Walk through [creating your first controller](https://metacontroller.github.io/metacontroller/guide/create.html).

### Quick install

```sh
kubectl apply -k https://github.com/metacontroller/metacontroller/manifests/production
```

See the [install guide](https://metacontroller.github.io/metacontroller/guide/install.html)
for prerequisites, the Helm alternative, and notes on migrating from the
original GKE project.

## Migrating from https://github.com/GoogleCloudPlatform/metacontroller

Please follow [this guide](https://metacontroller.github.io/metacontroller/guide/install.html?highlight=migrat#migrating-from-googlecloudplatformmetacontroller).

## Community & Contact

* File [GitHub issues](https://github.com/metacontroller/metacontroller/issues)
  for bugs, feature requests, and proposals.
* Join the [#metacontroller](https://kubernetes.slack.com/messages/metacontroller/)
  channel on [Kubernetes Slack](http://slack.kubernetes.io).

## Contributing

Contributions are very welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) and the
[contributor guide](https://metacontroller.github.io/metacontroller/contrib.html)
for how to build the project, run tests, and submit changes.

## Licensing

This project is licensed under the [Apache License 2.0](LICENSE).
