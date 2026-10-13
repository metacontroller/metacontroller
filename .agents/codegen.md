# CRD TYPES AND CODE GENERATION

## THE API PACKAGE (`pkg/apis/metacontroller/v1alpha1/`)
The Go types for the `metacontroller.k8s.io/v1alpha1` CRDs: CompositeController,
DecoratorController, and ControllerRevision. This is the most-imported package
in the repo.

| File | Role |
|------|------|
| `types.go` | all CRD types; the one source for the CRD YAML and the chart CRDs |
| `register.go` | group name `metacontroller.k8s.io`, scheme registration |
| `doc.go` | the `+k8s:deepcopy-gen=package,register` marker that controls generation |
| `zz_generated.deepcopy.go` | generated; do not edit |
| `roundtrip_test.go` | makes sure that marshal and unmarshal do not lose data |

A type change has an effect on `pkg/hooks`, `pkg/controller`, `pkg/client`, and
`pkg/dynamic`. Examine all four before you say that a type change is safe.

## SOURCE AND OUTPUT
| Source (edit this) | Target | Output (do not edit) |
|--------------------|--------|----------------------|
| `pkg/apis/metacontroller/v1alpha1/types.go` | `make deepcopy` | `pkg/apis/metacontroller/v1alpha1/zz_generated.deepcopy.go` |
| same | `make clientset` | `pkg/client/generated/clientset/` |
| same | `make lister` | `pkg/client/generated/lister/` |
| same | `make informer` | `pkg/client/generated/informer/` |
| same, with kubebuilder markers | `make generate_crds` | `manifests/production/metacontroller-crds-v1.yaml`, then copied to `deploy/helm/metacontroller/crds/` |

`make generated_files` runs `deepcopy`, `clientset`, `lister`, and `informer`.
`make build` runs `generated_files` first. It does not run `generate_crds`.

## TOOLS
The generators come from `.mise.toml`:
- `deepcopy-gen`, `client-gen`, `lister-gen`, `informer-gen` from `k8s.io/code-generator`.
  Their version must be the same as the `k8s.io/*` libraries in `go.mod`.
- `controller-gen` from `kube-controller-tools`.

Run `mise install` before you generate. If a generator has a different version,
the output changes in files that you did not touch.

## PROCEDURE: CHANGE A CRD TYPE
1. Edit `types.go`. Add JSON tags and kubebuilder markers as the adjacent fields do.
2. `make generated_files`
3. `make generate_crds`
4. `git status` — expect changes in the deepcopy file, the two CRD YAML files,
   and possibly `pkg/client/generated/`.
5. Update `docs/src/api/`.
6. `make unit-test` and `make integration-test`. The integration suites install
   the CRDs from `manifests/production/`.

## RULES
- The two CRD YAML files must be identical. `generate_crds` writes one and
  copies it. Do not change one of them by hand.
- The header for generated Go files is `hack/boilerplate.go.txt`.
- `types.go` has a `TODO(hot-reload)` comment: the CA bundle is resolved one time,
  at create or update of the controller object. See [webhooks.md](webhooks.md).
- `pkg/apis/.../doc.go` has an old `metacontroller.io/...` import comment. The
  module path is `metacontroller`. Do not change imports to match the comment.
- `.golangci.yaml` excludes `pkg/client/generated`. Lint findings there are
  not yours to fix.
- When only a generator version changed, commit the output as
  `chore: regenerate <what> with <tool> <version>`.
