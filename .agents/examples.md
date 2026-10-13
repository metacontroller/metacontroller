# EXAMPLES AND END-TO-END TESTS (`examples/`)

Each subdirectory is an example controller that you can run: a webhook, the
manifests, and a smoke test for a live cluster. CI uses these smoke tests as
the end-to-end tests.

## LAYOUT OF ONE EXAMPLE
```
examples/<name>/
├── README.md
├── test.sh      # smoke test; optional crd_version argument (default v1)
├── manifest/    # controller object + hook server (Python standard library, Jsonnet, or Node.js)
└── v1/          # kustomize overlay with `bases: ../manifest`
```

`catset/` and `bluegreen/` also have `api/` (Go types) and a `Makefile`.
`nodejs/` is not an example. It is the image source for the Node.js hook server.

## WHERE TO LOOK
| Task | Location |
|------|----------|
| Run all examples | `examples/test.sh`; options `-i/--ignore <dir>`, `--crd_version <v>` |
| Load the example images into kind | `examples/kind-load-images.sh` (expects a cluster named `metacontroller`) |
| Examples with Go types for the CRD | `catset/`, `bluegreen/` |
| Webhook authorization (TLS, basic, bearer) | `webhook-auth/`; needs `openssl` |
| Secret and ConfigMap propagation | `secretpropagation/`, `configmappropagation/`, `globalconfigmap/` |
| Jsonnet hooks | `service-per-pod/`, `service-per-pod-v2/` |
| Update strategies and rollouts | `update-strategy/`, `bluegreen/` |
| Hook API v2 | directories with the `-v2` suffix |

## RUN
Each `test.sh` needs a live cluster with metacontroller installed and `kubectl`
configured. No test runs offline.

```bash
make image
kind load docker-image localhost/metacontroller:dev
kubectl apply -k manifests/dev        # or manifests/dev-ssa
cd examples && ./test.sh              # or: cd examples/<name> && ./test.sh
```

CI runs this for each entry in `.github/config/kind-matrix-config.json`.

## RULES
- Python hooks use only the standard library (`http.server`). Do not add a web
  framework or a third-party package.
- Most scripts use `set -euo` without `pipefail`. Follow the adjacent scripts.
- `v1/crdv1.yaml` in `catset/` and `bluegreen/` is generated from the Go types
  in `api/`. Edit the types and run `make generate_crds` in that directory. Do
  not edit the YAML.
- A new example needs `README.md`, `manifest/`, `v1/`, `test.sh`, and a line in
  `docs/src/examples.md`.
- `.golangci.yaml` excludes `examples/`. `make unit-test` excludes it too.

## TRAPS
- `webhook-auth/test.sh` must not run on a production cluster. The script says so.
- The `catset` and `bluegreen` Makefiles set `CODE_GENERATOR_VERSION`, but they
  install `controller-gen@latest`. The pin has no effect, and the generated
  output can change between runs.
