# ADDING OR CHANGING A FEATURE

## BEFORE YOU START
1. Read [architecture.md](architecture.md) to find the code.
2. Read the area guide for each directory that you will change. The list is
   in [AGENTS.md](../AGENTS.md).
3. Decide which surfaces the feature touches. Use the checklist below.

## CHECKLIST BY SURFACE
A feature is frequently more than Go code. Do each row that applies.

| If the feature changes... | Then also change... |
|---------------------------|---------------------|
| A CRD field (`pkg/apis/metacontroller/v1alpha1/types.go`) | run `make generated_files` and `make generate_crds`; update `docs/src/api/*.md`. See [codegen.md](codegen.md) |
| A command line flag (`pkg/cmd/metacontroller/main.go`) | `pkg/options/configuration.go`; the flag table in `docs/src/guide/configuration.md`; if a default deployment needs it: `manifests/*/args.yaml` and the chart `values.yaml` (`commandArgs` list) |
| Composite sync behavior (`pkg/controller/composite/`) | the same behavior in `pkg/controller/decorator/`, or a reason why not |
| Shared sync code (`pkg/controller/common/`) | tests for the two controller kinds |
| Hook request or response types (`*/api/v1`, `*/api/v2`) | `docs/src/api/hook.md`, `compositecontroller.md`, `decoratorcontroller.md`, `customize.md`; new work goes to v2 |
| Webhook call, auth, TLS (`pkg/hooks/`) | `docs/src/api/hook.md`; `*_with_auth` integration suites |
| RBAC needs of the binary | `manifests/production/metacontroller-rbac.yaml` and the chart `templates/` |
| Helm chart | a values file in `deploy/helm/metacontroller/ci/` for the new toggle; `docs/src/guide/helm-install.md` |
| A user-visible workflow | an example in `examples/<name>/` with `README.md`, `manifest/`, `test.sh`; a line in `docs/src/examples.md` |
| Metrics or events | `pkg/metrics/`, `pkg/events/events.go` (`Reason*` constants) |

## WORK ORDER
1. Write a unit test that fails for the new behavior.
2. Write the code in `pkg/`.
3. If types changed, regenerate code and CRDs.
4. Add an integration test in `test/integration/`. Most behavior of
   metacontroller is API objects in, API objects out, so this level is the
   important one. See [testing.md](testing.md).
5. Update docs in `docs/src/`. If you add a page, add it to `docs/src/SUMMARY.md`.
6. Run `make lint`, `make unit-test`, `make integration-test`, `make build`.

## CODE RULES
- Each controller kind has the same shape: `metacontroller.go` (lifecycle),
  `controller.go` (sync), `hooks.go` (hook plumbing), `api/` (hook types).
  Put new code in the file that matches its role.
- These files are already longer than 500 lines: `composite/controller.go`,
  `decorator/controller.go`, `controller_revision.go`, `manage_children.go`.
  Do not make them longer. Put new logic in a new file.
- New `.go` files need the Apache 2.0 license header with
  `Copyright <year> Metacontroller authors.`
- `gofmt` and `goimports` are enforced by `make lint`. The linter list is in
  `.golangci.yaml`. `exhaustive` is on: handle each enum value in a `switch`.
- A new CRD field must be optional, or have a default. Existing controller
  objects in user clusters must stay valid.
- CA bundles and auth material are resolved one time, at create or update of the
  controller object. Do not write a feature that depends on live secret rotation.

## COMMIT TYPE
- `feat(<scope>): ...` starts a minor release.
- `fix(<scope>): ...` and `perf(<scope>): ...` start a patch release.
- Use `docs`, `test`, `chore`, or `refactor` for changes that must not release.

See [release-ci.md](release-ci.md).
