# ARCHITECTURE, CONVENTIONS, AND TRAPS

For detail on one area, read its guide: [controllers.md](controllers.md),
[webhooks.md](webhooks.md), [dynamic-client.md](dynamic-client.md),
[codegen.md](codegen.md), [testing.md](testing.md),
[helm-chart.md](helm-chart.md), [examples.md](examples.md).

## PACKAGES IN `pkg/`
```
pkg/
├── apis/         # CRD types v1alpha1
├── controller/   # sync controllers: common, composite, decorator
├── hooks/        # webhook calls and authorization
├── dynamic/      # discovery, clientset, informer, apply
├── client/       # generated clientset, informer, lister — do not edit
├── cmd/          # binary entry point (cmd/metacontroller)
├── server/       # controller-runtime manager setup
├── options/      # Configuration struct: workers, discovery interval, leader election, apply strategy
├── events/       # NewBroadcaster and the Reason* constants
├── metrics/      # Prometheus metrics
├── profile/      # pprof
├── cache/        # small cache helper
├── logging/      # zap setup
├── internal/     # test helpers (testutils)
└── third_party/  # code copied from Kubernetes, with its own license
```

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| CRD types | `pkg/apis/metacontroller/v1alpha1/types.go` | then regenerate; see [codegen.md](codegen.md) |
| Binary entry point and flags | `pkg/cmd/metacontroller/main.go` | flags fill `options.Configuration` |
| Runtime configuration struct | `pkg/options/configuration.go` | used by `cmd/`, `server/`, `controller/` |
| Server wiring | `pkg/server/server.go` | controller-runtime manager |
| Composite reconcile | `pkg/controller/composite/` | `controller.go` is the sync loop |
| Decorator reconcile | `pkg/controller/decorator/` | near-duplicate of composite |
| Shared sync code | `pkg/controller/common/` | child apply, customize hook, finalizer |
| Webhook call | `pkg/hooks/` | endpoint resolution, auth, CA bundle, retry |
| Dynamic Kubernetes access | `pkg/dynamic/` | discovery, clientset, informer, apply |
| Events and metrics | `pkg/events/`, `pkg/metrics/` | |
| Test helpers | `pkg/internal/testutils/` | fakes and builders for unit tests |
| Install manifests | `manifests/production/` | kustomize; `dev`, `dev-ssa`, `debug` are overlays |
| Helm chart | `deploy/helm/metacontroller/` | `values.yaml` is camelCase; flags go in `commandArgs` |
| User documentation | `docs/src/` | mdBook; the page list is `SUMMARY.md` |

## MOST-IMPORTED PACKAGES
A change in these packages has a large effect. Run all tests.

| Package | Role |
|---------|------|
| `pkg/apis/metacontroller/v1alpha1` | CRD types: Composite/Decorator controllers, ControllerRevision, Hook |
| `pkg/controller/common/api` | versioned object maps (v1 relative, v2 uniform) |
| `pkg/dynamic/clientset` | dynamic client for each GroupVersionResource |
| `pkg/controller/common` | shared controller context and apply primitives |
| `pkg/dynamic/discovery` | API resource discovery cache |

## CONVENTIONS
- Hook APIs have a version in the package path: `api/common` (shared builder),
  `api/v1` (`RelativeObjectMap`), `api/v2` (`UniformObjectMap`). New work goes to v2.
- Each controller kind has `metacontroller.go` with a `Metacontroller` struct and
  a `NewMetacontroller` constructor.
- Unit test names: `Test<Func>_when<Condition>_<expected>`.
- Example hooks are plain HTTP servers (Python standard library, Jsonnet, or Go).
  Each example has `manifest/` and a `v1/` kustomize overlay.
- Kustomize (`manifests/`) and Helm (`deploy/helm/`) are two separate install
  paths. Keep them equivalent, but do not mix their files.

## TRAPS
- The composite and decorator sync loops are near-duplicates. A fix in one
  usually applies to the other.
- Hook resolution has no hot reload. A changed Secret or ConfigMap for a CA
  bundle or basic auth has no effect until the controller object changes.
- The `metacontroller.io/...` import comment in `pkg/apis` `doc.go` is old. The
  module path is `metacontroller`.
- Files that are already longer than 500 lines: `composite/controller.go`,
  `decorator/controller.go`, `controller_revision.go`, `manage_children.go`,
  `manager_test.go`, `webhook_test.go`. Do not make them longer.
- The test apiserver uses `--authorization-mode AlwaysAllow`. Do not copy this
  into a real cluster configuration.
- `examples/catset` and `examples/bluegreen` run `controller-gen@latest`. Their
  generated output is not reproducible.
- These root directories are local or generated, not source: `dist/`,
  `node_modules/`, `metacontroller` (binary), `test/integration/hack/bin/`,
  `test/integration/hack/tmp/`.
