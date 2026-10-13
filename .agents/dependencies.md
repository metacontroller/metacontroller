# DEPENDENCY UPDATES

Renovate (`renovate.json`) opens most update PRs. Use this guide when you do an
update by hand, or when you complete a Renovate PR that needs more work.

## WHERE VERSIONS LIVE
| What | File | Notes |
|------|------|-------|
| Go libraries (binary) | `go.mod`, `go.sum` | has a `replace` block for some `k8s.io/*` modules and `golang.org/x/net` |
| Go libraries (integration tests) | `test/integration/go.mod`, `go.sum` | separate module; `replace metacontroller => ../..` plus its own `k8s.io/*` replaces |
| Tools (Go, linters, generators, kubectl, helm, kind) | `.mise.toml` | CI uses the same file through `jdx/mise-action` |
| Go language version | `go` directive in `go.mod` and `test/integration/go.mod`; `go` in `.mise.toml`; `golang:` image in `Dockerfile.debug` | keep them consistent |
| Base images | `Dockerfile`, `Dockerfile.debug`, `Dockerfile.distroless*` | alpine images are pinned by digest |
| Test apiserver and etcd | `KUBE_VERSION`, `ETCD_VERSION` in `test/integration/hack/get-kube-binaries.sh` | binaries are cached in `test/integration/hack/bin/` |
| e2e Kubernetes versions | `.github/config/kind-matrix-config.json` | ascending order; the last entry is used for the Helm tests |
| Release tooling (npm) | `package.json`, `package-lock.json` | semantic-release only; not part of the binary |
| GitHub Actions | `.github/workflows/*.yaml` | `actionlint` checks them in CI |

## PROCEDURE: ONE GO MODULE
1. In the repo root: `go get <module>@<version>`, then `go mod tidy`.
2. In `test/integration`: `go mod tidy`. Do this each time. The module follows
   the root through `replace`, so its `go.mod` and `go.sum` change too.
3. If the same module is in a `replace` block, change the version there in
   both `go.mod` files.
4. Do the verification steps below.

## PROCEDURE: KUBERNETES LIBRARIES
Move all `k8s.io/*` modules together. Do not mix minor versions.

1. Set the new version for `k8s.io/api`, `apiextensions-apiserver`, `apimachinery`,
   and `client-go` in `go.mod` (the `require` and the `replace` blocks).
2. Do the same in `test/integration/go.mod`.
3. Make sure `sigs.k8s.io/controller-runtime` supports that Kubernetes minor
   version. Update it if necessary.
4. `go mod tidy` in the root, then in `test/integration`.
5. In `.mise.toml`, set the four `go:k8s.io/code-generator/cmd/*-gen` tools to the
   same version as the libraries. Set `kubectl` to the matching release.
6. `mise install`, then `make generated_files` and `make generate_crds`.
   Commit the regenerated output in its own commit.
7. For a new minor version: update `.github/config/kind-matrix-config.json`
   (add the new version, remove the oldest, for both `dev` and `dev-ssa`), and
   examine `KUBE_VERSION`/`ETCD_VERSION` in `get-kube-binaries.sh`.
8. Do the verification steps below. API changes in client-go or
   controller-runtime frequently need code fixes in `pkg/dynamic/` and `pkg/server/`.

## PROCEDURE: A TOOL IN `.mise.toml`
1. Change the version, then run `mise install`.
2. Run the target that uses the tool:

| Tool | Then run |
|------|----------|
| `kube-controller-tools` (controller-gen) | `make generate_crds` — the version is written into the CRD YAML |
| `go:k8s.io/code-generator/cmd/*` | `make generated_files` |
| `golangci-lint` | `make lint` — new versions can add findings |
| `goreleaser` | `make build` and `make image` |
| `gotestsum` | `make unit-test` |
| `helm`, `helm-ct`, `yamllint`, `pipx:yamale` | chart lint; CI runs `ct lint-and-install` |
| `go` | update the `go` directive and `Dockerfile.debug`, then do all verification steps |

## PROCEDURE: NPM RELEASE TOOLING
1. Change `package.json`, then run `npm install` to update `package-lock.json`.
2. If `conventional-changelog-angular` changes, compare its default
   `commitPartial` with the copy in `release.config.js`. The copy must stay in sync.
3. These packages run only in the release job. No local test covers them.

## VERIFICATION
```bash
make lint
make unit-test
make integration-test
make build
```
If a generator or its version changed, also run `git status` after
`make generated_files` and `make generate_crds`. Commit the differences.

## COMMIT MESSAGES
Follow the pattern in `git log`:

| Change | Header |
|--------|--------|
| Go module or Dockerfile image | `fix(deps): update module <name> to <version>` — starts a patch release |
| `test/integration/go.mod` only | `test(deps): ...` or `chore: tidy integration test module after <cause>` |
| Tool in `.mise.toml`, npm package, GitHub Action | `chore(deps): update dependency <name> to <version>` — no release |
| Regenerated code or CRDs | `chore: regenerate <what> with <tool> <version>` |

Keep one logical update in one commit. Put regenerated output in a different commit.

## TRAPS
- `test-setup` does not download a binary that is already in `hack/bin/`. After
  you change `KUBE_VERSION` or `ETCD_VERSION`, delete the old binaries first.
- `get-kube-binaries.sh` downloads `linux/amd64` binaries only.
- `Dockerfile.debug` installs `dlv@latest`. This is not pinned.
- `examples/catset` and `examples/bluegreen` have Makefiles with a dead
  `CODE_GENERATOR_VERSION` pin. They run `controller-gen@latest`. Do not rely on it.
- Do not remove a `replace` line because it looks redundant. Run `go mod tidy`
  and all tests before you decide.
