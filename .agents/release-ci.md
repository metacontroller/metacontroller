# COMMITS, CI, AND RELEASES

## COMMIT AND PR TITLE
Format: `<type>(<scope>): [#issue_number] - <subject>`

The issue number is optional. `.github/semantic.yml` checks PR titles and
commits. Types are from the angular convention.

| Type | Release on merge to `master` |
|------|------------------------------|
| `feat` | minor |
| `fix`, `perf` | patch |
| `chore`, `docs`, `test`, `refactor`, `ci`, `style` | none |

Select the type by its effect on users, because the type starts the release.
A Go module update is `fix(deps)`. A tool update is `chore(deps)`.

Examples from `git log`:
```
fix(customize): #1253 - call customize hook only for parents that match the selector
fix(deps): update module sigs.k8s.io/controller-runtime to v0.25.2
chore(deps): update dependency kube-controller-tools to v0.22.0
chore: regenerate client code with code-generator v0.37.1
```

Remove the template text from the PR body before you submit a PR.

## CI WORKFLOWS (`.github/workflows/`)
| File | Trigger | Jobs |
|------|---------|------|
| `static.yaml` | pull request | golangci-lint, actionlint, image build, trivy, dockle, mdbook build, nancy |
| `test_release.yaml` | pull request, push to `master` | image build (dev, debug), `make unit-test`, `make integration-test`, e2e on kind (matrix), helm `ct lint-and-install`, release |
| `master-static.yaml` | push to `master` | image build, trivy, dockle |
| `helm_release.yaml` | tag `v*.*.*` | package the chart, push to `oci://ghcr.io/metacontroller` |
| `documentation.yaml` | push to `master` | mdbook build, publish to `gh-pages`, Docker Hub readme |

The build, lint, and test jobs get their tools from `.mise.toml` through `jdx/mise-action`. A tool
version that works locally with `mise install` is the version that CI uses.

Local equivalents of the PR checks:
```bash
make lint
make unit-test
make integration-test
make image              # the image that trivy and dockle scan
(cd docs && mdbook build)   # needs mdbook + plugins; see docs/src/contrib/build.md
```

## RELEASE FLOW
1. A merge to `master` runs `test_release.yaml`.
2. After tests pass, `npx semantic-release` reads the commits (`release.config.js`).
3. It writes `CHANGELOG.md`, the image tag in `manifests/production/metacontroller.yaml`,
   and `version`/`appVersion` in `deploy/helm/metacontroller/Chart.yaml`.
   It commits them as `chore(release): <version>`.
4. `.release.sh` runs `goreleaser release` (`.goreleaser.yaml`): binaries and
   multi-arch images.
5. The new tag starts `helm_release.yaml`.

Rules for agents:
- Do not edit the files in step 3 by hand.
- Do not run `semantic-release`, `goreleaser release`, or `.release.sh` locally.
- Do not make or push tags.
- The release and publish jobs run only when the repository owner is
  `metacontroller`. They do not run on forks.
