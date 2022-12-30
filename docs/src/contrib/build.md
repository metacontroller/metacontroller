# Building

The page describes how to build Metacontroller for yourself.

[[_TOC_]]

First, check out the code:

```sh
# If you're going to build locally, make sure to
# place the repo according to the Go import path:
#   $GOPATH/src/metacontroller.io
cd $GOPATH/src
git clone git@github.com:metacontroller/metacontroller.git metacontroller
cd metacontroller
```

Then you can build a `metacontroller` binary like so:

```sh
make build
```

Note that you will need the following k8s code-generation tools:
* deepcopy-gen
* client-gen
* lister-gen
* informer-gen

...all of which can be found [here](https://github.com/kubernetes/code-generator).

## Local build and development

Check [debug section](./debug.md)

## Documentation build

Documentation is generated from `.md` files with [mdbook](https://github.com/rust-lang/mdBook).
To generate documentation, you need to install:
* mdbook
* mdbook plugins:
    * [linkcheck](https://crates.io/crates/mdbook-linkcheck) - verifies link corectness
    * [toc](https://crates.io/crates/mdbook-toc) - creates TOC's - table of content
    * [graphviz](https://crates.io/crates/mdbook-graphviz) - generation of dot diagrams
    * [open-on-gh](https://crates.io/crates/mdbook-open-on-gh) - adds open-on-gh link
* graphviz

To generate documentation
* `cd docs`
* `mdbook build`
There will be `book` folder generated with html content.

You can also use `mdbook serve` to expose documentation on `http://localhost:3000`.

## Tests

To run tests, first make sure you can successfully complete a [local build](#local-build).

### Unit Tests

Unit tests in Metacontroller focus on code that does some kind of non-trival
local computation without depending on calls to remote servers -- for example,
the `./dynamic/apply` package.

Unit tests live in `_test.go` files alongside the code that they test.
To run only unit tests (excluding [integration tests](#integration-tests))
for all Metacontroller packages, use this command:

```sh
make unit-test
```

### Integration Tests

Integration tests in Metacontroller focus on verifying behavior at the level of
calls to remote services like user-provided webhooks and the Kubernetes API server.
Since Metacontroller's job is ultimately to manipulate Kubernetes API objects in
response to other Kubernetes API objects, most of the important features or
behaviors of Metacontroller can and should be tested at this level.

In the integration test environment, we start a standalone `kube-apiserver` to
serve the REST APIs, and an `etcd` instance to back it.
We do *not* run any kubelets (Nodes), nor any controllers other than
Metacontroller.
This makes it easy for tests to control exactly what API objects Metacontroller
sees without interference from the normal controller for each API,
and also greatly reduces the requirements to run tests.

Other than the Metacontroller codebase, all you need to run integration tests
is to download a few binaries from a Kubernetes release.
You can run the following script from the `test/integration` directory in to
order to fetch the versions of these binaries currently used in continuous
integration, and place them in `./hack/bin`:

```sh
hack/get-kube-binaries.sh
```

You can then run the integration tests with this command, which will
automatically set the PATH to include `./hack/bin`:

```sh
make integration-test
```

Unlike unit tests, integration tests do not live alongside the code they test,
but instead are gathered in `./test/integration/...`.
This makes it easier to run them separately, since they require a special
environment, and also enforces that they test packages at the level of their
public interfaces.

### End-to-End Tests

End-to-end tests in Metacontroller focus on verifying example workflows that we
expect to be typical for end users. That is, we run the same `kubectl` commands
that a human might run when using Metacontroller.

Since these tests verify end-to-end behavior, they require a fully-functioning
Kubernetes cluster.
Before running them, you should have `kubectl` in your PATH, and it should be
configured to talk to a suitable, empty test cluster that has had the
Metacontroller manifests applied.

Then you can run the end-to-end tests against your cluster with the following:

```sh
cd examples
./test.sh
```

This will run all the end-to-end tests in series, and print the location of a
log file containing the output of the latest test that was run.

You can also run each test individually, which will show the output as it runs.
For example:

```sh
cd examples/bluegreen
./test.sh
```

Note that currently our continuous integration only runs unit and integration
tests on PRs, since those don't require a full cluster.
If you have access to a suitable test cluster, you can help speed up review of
your PR by running these end-to-end tests yourself to see if they catch anything.
