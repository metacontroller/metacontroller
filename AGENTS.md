# METACONTROLLER — AGENT GUIDE

`CLAUDE.md` is a symlink to this file. Edit `AGENTS.md` only.

Keep this file short. Put detail in `.agents/`. Do not add `AGENTS.md` files in
subdirectories.

## OVERVIEW
Metacontroller is a Kubernetes add-on. It lets you write custom controllers as
JSON webhooks in any language. The Go binary does the watches, caches, work
queues, retries, and reconciliation.

Go module path: `metacontroller` (no domain prefix). Versions are in `go.mod`
(libraries) and `.mise.toml` (tools). Do not copy version numbers into agent docs.

## GUIDE INDEX (`.agents/`)
Each guide is a small file for one subject. Read a guide before you change a
file in its scope. One task frequently needs more than one guide.

| Guide | Read it when | What it gives you |
|-------|--------------|-------------------|
| [architecture.md](.agents/architecture.md) | You do not know where the code for a task is. You start work in this repo. | Map of `pkg/`, most-imported packages, conventions, known traps. |
| [features.md](.agents/features.md) | You add or change a feature, a command line flag, a hook, RBAC, or a metric. | Checklist of all files that a feature touches, the work order, code rules. |
| [testing.md](.agents/testing.md) | You write a test, run tests, or a test fails. You change `test/integration/`. | The three test levels, `make` targets, how to run one test, the integration test pattern. |
| [dependencies.md](.agents/dependencies.md) | You update a Go module, Kubernetes libraries, a tool in `.mise.toml`, a base image, an npm package, or you complete a Renovate PR. | Where each version is, the procedure for each type of update, commit types. |
| [codegen.md](.agents/codegen.md) | You change `pkg/apis/`, a CRD field, or a generator version. `git status` shows changes in generated files. | Source file, `make` target, and output file for each generator. |
| [release-ci.md](.agents/release-ci.md) | You write a commit message or a PR. A CI job fails. You must know if a change starts a release. | Commit format and types, the CI workflows, the release flow, files that the release job writes. |
| [controllers.md](.agents/controllers.md) | You change `pkg/controller/`: sync loops, child management, the customize hook, finalizers, rollouts. | File roles, shared primitives, the composite and decorator duplication rule. |
| [webhooks.md](.agents/webhooks.md) | You change `pkg/hooks/`: the webhook call, authorization, TLS, CA bundles, ETag, retries. | File and function map, test naming, the no-hot-reload trap. |
| [dynamic-client.md](.agents/dynamic-client.md) | You change `pkg/dynamic/`: discovery, clients, informers, apply, owner references. A `client-go` update breaks the build. | Package map, import aliases, the order discovery, clientset, informer. |
| [helm-chart.md](.agents/helm-chart.md) | You change `deploy/helm/`, add a chart value, or change RBAC or the StatefulSet. | Chart rules, the `ci/` values files, the chart-testing command. |
| [examples.md](.agents/examples.md) | You add or change an example in `examples/`. An e2e test fails. You must run the e2e tests locally. | Layout of one example, how to run the tests on kind, traps. |

When you add a guide, add a row to this table.

## STRUCTURE
```
.
├── pkg/               # all Go source
├── test/integration/  # separate Go module: suites against a local apiserver + etcd
├── deploy/helm/       # Helm chart
├── manifests/         # kustomize install path: production / dev / dev-ssa / debug
├── examples/          # example controllers + e2e smoke tests
├── docs/src/          # mdBook documentation
├── .agents/           # task and area guides for agents
├── .mise.toml         # pinned tool versions
└── Makefile           # build / test / lint / generate targets
```

## COMMANDS
Use `make`. Do not use bare `go build` or `go test` for verification.
Run `mise install` one time first; it installs each tool at its pinned version.

```bash
make build             # goreleaser snapshot build -> ./metacontroller (runs generated_files first)
make unit-test         # gotestsum -race + coverage; root module without test/integration and examples
make integration-test  # suites in test/integration; -timeout 5m -parallel 1
make lint              # golangci-lint run --timeout 10m ./...
make generated_files   # deepcopy + clientset + lister + informer
make generate_crds     # controller-gen -> manifests/production + deploy/helm crds/
make image             # docker build localhost/metacontroller:dev
```

## DONE MEANS
A change is complete only when these commands pass:

1. `make lint`
2. `make unit-test`
3. `make integration-test` (always, when you change `pkg/` or `test/integration/`)
4. `make build`

If you cannot run a step, say so. Do not report it as passed.

## WRITING STYLE
Write all text in ASD-STE100 Simplified Technical English. This applies to
agent docs (`AGENTS.md`, `.agents/`), user docs (`docs/src/`), README files,
code comments, commit messages, and PR titles and descriptions.

- Write short sentences. Put one idea in each sentence.
- Use the active voice.
- Use common words. Explain a technical term the first time you use it.
- Use the same term for the same thing each time.
- Write instructions as commands: "Run `make lint`."

Do not rewrite existing text only to change its style. Apply the rules to
text that you add or change.

## HARD RULES
- Do not edit generated files: `zz_generated.deepcopy.go`, `pkg/client/generated/`,
  `metacontroller-crds-v1.yaml` (in `manifests/production/` and in the chart `crds/`).
  Change the source and regenerate. See [.agents/codegen.md](.agents/codegen.md).
- Composite and decorator sync loops are near-duplicates. Apply a fix to both,
  or say why only one needs it.
- There are two Go modules: the root and `test/integration`. A dependency change
  in the root usually needs `go mod tidy` in both.
- Commit headers follow the angular convention. The type decides if a release
  starts. See [.agents/release-ci.md](.agents/release-ci.md).
- Do not edit `CHANGELOG.md`, the chart `version`/`appVersion`, or the image tag in
  `manifests/production/metacontroller.yaml`. The release job writes them.
