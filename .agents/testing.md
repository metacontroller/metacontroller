# TESTING

There are three test levels. Each level has a different location and a
different command.

| Level | Location | Command | Needs |
|-------|----------|---------|-------|
| Unit | `pkg/**/*_test.go`, next to the code | `make unit-test` | nothing external |
| Integration | `test/integration/` (separate Go module) | `make integration-test` | kube-apiserver, etcd, kubectl in `test/integration/hack/bin/` |
| End-to-end | `examples/*/test.sh` | `cd examples && ./test.sh` | a live cluster with metacontroller installed |

## MAKE TARGETS
```bash
make test-setup        # download kube binaries to test/integration/hack/bin, create hack/tmp
make unit-test         # runs test-setup, then gotestsum -race with coverage
make integration-test  # runs test-setup, then gotestsum in test/integration, -timeout 5m -parallel 1
```

- The Makefile puts `test/integration/hack/bin` first on `PATH`. The integration
  framework needs that. A bare `go test ./...` from the repo root does not
  run the integration suites and does not find the binaries.
- `unit-test` covers the root module without `/test/integration/` and `/examples/`.
- Coverage files go to `test/integration/hack/tmp/`. Do not read or commit them.
- Tools come from `.mise.toml`. Run `mise install` if `gotestsum` is not found.

## RUN ONE TEST (FAST LOOP ONLY)
`make` has no filter parameter. For a fast loop, call `gotestsum` directly.
Always complete the work with the full `make` targets.

```bash
# unit: one package, one test
gotestsum -- -race ./pkg/hooks/... -run 'TestResolveCABundle'

# integration: one suite, one test (run make test-setup one time first)
cd test/integration && PATH="$PWD/hack/bin:$PATH" \
  gotestsum -- ./composite/... -run 'TestSyncWebhookV2' -timeout 5m -parallel 1
```

## UNIT TESTS
- Name: `Test<Func>_when<Condition>_<expected>`, for example
  `TestResolveCABundle_whenSecretRefKeyNotFound_returnsError`. This is the
  given-when-then format: `<Func>` is the subject, `when<Condition>` is the
  start state or the input, and `<expected>` is the result. Start
  `<expected>` with a verb: `returnsError`, `removesFinalizer`, `isNotDone`.
  Some old tests use other names. Do not rename them only to change the style.
- Body: put the setup, the call, and the assertions in three blocks. Put one
  empty line between the blocks. See
  `pkg/controller/common/finalizer/cleanup_test.go`.
- Assertions: `github.com/stretchr/testify` and `github.com/google/go-cmp`.
- Shared fakes and builders: `pkg/internal/testutils/{common,dynamic,hooks}`.
  Use them before you write a new fake.
- `goconst` is off for `_test.go` files. All other linters apply.
- A change in `pkg/controller/composite` usually needs the same test in
  `pkg/controller/decorator`, and the other way round.

## INTEGRATION TESTS
The framework starts etcd, kube-apiserver, and kube-controller-manager as
processes. It starts the metacontroller server inside the test binary. There
are no nodes and no kubelets, so pods do not run.

Suites, one directory each:
`composite`, `decorator`, `composite_with_auth`, `decorator_with_auth`,
`composite_with_target_label_selector`, `decorator_with_target_label_selector`.

Framework files in `test/integration/framework/`:

| File | Role |
|------|------|
| `main.go` | `TestMain`, `TestMainWithTargetLabelSelector`; starts all processes |
| `fixture.go` | `Fixture`, `NewFixture`, `TearDown`, `Wait`, `CreateNamespace` |
| `apiserver.go`, `etcd.go`, `controllermanager.go` | process setup; `ApiserverURL`, `ApiserverConfig` |
| `crd.go` | `CreateCRD`, `UnstructuredCRD` |
| `metacontroller.go` | `CreateCompositeController`, `CreateDecoratorController`, `CRDResourceRule` |
| `webhook.go` | `ServeWebhook` |
| `secret.go` | Secrets for the authorization suites |
| `finalizer.go` | `RunFinalizerCleanupOnDelete`, `RunFinalizerCleanupNotInOperation`; shared tests for the two controller kinds |

Pattern for a new test (see `test/integration/composite/composite_v2_test.go`):
```go
f := framework.NewFixture(t)
defer f.TearDown()

f.CreateNamespace(ns)
parentCRD, parentClient := f.CreateCRD("MyTestParent", apiextensions.NamespaceScoped)
childCRD, childClient := f.CreateCRD("MyTestChild", apiextensions.NamespaceScoped)

hook := f.ServeWebhook(func(body []byte) ([]byte, error) { /* decode request, build response */ })
f.CreateCompositeController("my-test", hook.URL, "", framework.CRDResourceRule(parentCRD), framework.CRDResourceRule(childCRD), nil)

err := f.Wait(func() (bool, error) { /* poll until the expected state */ })
```

Rules:
- Name a new test in the given-when-then format of unit tests:
  `Test<Feature>_when<Condition>_<expected>`. `<Feature>` is the behavior
  that the test examines, not a function. For example:
  `TestFinalizerCleanup_whenControllerIsDeleted_removesParentFinalizers`.
- Use a namespace and CRD kinds that are unique for each test. All tests in
  a suite share one apiserver.
- Poll with `f.Wait`. Do not use `time.Sleep`.
- Put hook API v2 tests in a `_v2_test.go` file in the same directory. Do not
  make a new directory for v2.
- Make a new suite directory only if the test needs a different server
  configuration. Add a `TestMain(m *testing.M)` function in its test file that
  calls `framework.TestMain(m.Run)`, or a new variant from `framework/main.go`.
- All six suites import `framework/`. After a framework change, run the full
  `make integration-test`.
- The suites install the CRDs from `manifests/production/`. After a type change,
  run `make generate_crds` before the tests.

## END-TO-END TESTS
- `examples/test.sh` runs each example `test.sh` in series.
  Options: `-i/--ignore <dir>`, `--crd_version <v>`.
- CI runs them on kind for each entry in `.github/config/kind-matrix-config.json`,
  with `manifests/dev` and `manifests/dev-ssa`.
- Local run: `make image`, load the image into kind, `kubectl apply -k manifests/dev`,
  then `cd examples && ./test.sh`.
- `examples/webhook-auth/test.sh` must not run on a production cluster.

## WHICH TESTS FOR WHICH CHANGE
| Change | Minimum tests |
|--------|---------------|
| Pure logic in one package | unit test in that package |
| Sync behavior, hooks, finalizers, selectors | unit test + integration test for composite and decorator |
| CRD type or schema | regenerate, then unit + integration |
| Helm chart | a values file in `deploy/helm/metacontroller/ci/`; CI runs `ct lint-and-install` |
| User-visible workflow | an example in `examples/` with a `test.sh` |

## TRAPS
- The test apiserver uses `--authorization-mode AlwaysAllow`. This is for
  tests only. RBAC errors do not show in integration tests.
- `-parallel 1` is intentional. Do not add `t.Parallel()` to integration tests.
- `test-setup` keeps old binaries. If `KUBE_VERSION` changed, delete
  `test/integration/hack/bin/` first.
