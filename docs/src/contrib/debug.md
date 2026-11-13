# Local development and debugging

Tips and tricks for contributors

[[_TOC_]]

## Local run of metacontroller

There are different flavours of manifests shipped to help with local development:

- manifests/dev
- manifests/debug

### Development build

The main difference it that image defined in manifest is `localhost/metacontroller:dev`, therefore:

- apply dev manifests - `kubectl apply -k manifests/dev`
- build docker image with command - `make image` - this will compile the binary and build the container image
- load image into cluster (i.e. `kind load docker-image localhost/metacontroller:dev` in kind)
- restart pod (i.e. `kubectl delete pod/metacontroller-0 --namespace metacontroller`)

### Debug build

Debug requires building go sources in special way, which is done with `make build_debug`; the following image
built with the `Dockerfile.debug` dockerfile will then add it to the debug Docker image:

- apply debug manifests - `kubectl apply -k manifests/debug`
- build debug binary and image - `make image_debug`
- load image into cluster (i.e. `kind load docker-image localhost/metacontroller:debug` in kind)
- restart pod
- on startup, `go` process will wait for debugger on port 40000
- port forward port 40000 from container into localhost, i.e. `kubectl port-forward metacontroller-0 40000:40000`
- attach `go` debugger to port 40000 on localhost

## Running end-to-end tests locally

End-to-end tests exercise the full example suite against a real Kubernetes
cluster managed by [kind](https://kind.sigs.k8s.io/). You need `kind` and
`docker` on your PATH.

```sh
make e2e-test
```

This target builds the `localhost/metacontroller:dev` image from the current
working tree, creates a throwaway kind cluster, installs metacontroller using
`manifests/dev`, runs all examples under `examples/test.sh`, and deletes the
cluster when done.

The following variables let you tune the run:

| Variable                      | Default                    | Description                                                                                                               |
| ----------------------------- | -------------------------- | ------------------------------------------------------------------------------------------------------------------------- |
| `E2E_VARIANT`                 | `dev`                      | Manifest variant to install. Use `dev-ssa` to exercise the server-side-apply path (`--apply-strategy=server-side-apply`). |
| `E2E_NODE_IMAGE`              | `kindest/node:v1.35.0@...` | kind node image; change to test against a different Kubernetes version.                                                   |
| `E2E_CLUSTER_NAME`            | `metacontroller-e2e`       | Name of the ephemeral kind cluster.                                                                                       |
| `E2E_KEEP_CLUSTER_ON_FAILURE` | _(unset)_                  | Set to any non-empty value to keep the cluster alive and dump metacontroller logs when a test fails, for debugging.       |

Examples:

```sh
# Run against the server-side-apply variant
make e2e-test E2E_VARIANT=dev-ssa

# Keep the cluster if something goes wrong
make e2e-test E2E_KEEP_CLUSTER_ON_FAILURE=1
```

Note: the target switches your current kube-context to the kind cluster. This
mirrors the `e2etests` job in CI.
