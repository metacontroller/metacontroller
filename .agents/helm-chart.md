# HELM CHART (`deploy/helm/metacontroller/`)

The chart name is `metacontroller-helm`. The release job publishes it to
`oci://ghcr.io/metacontroller`.

## STRUCTURE
```
deploy/helm/metacontroller/
├── Chart.yaml     # version and appVersion are written by the release job
├── values.yaml    # default configuration
├── templates/     # statefulset, service, RBAC (clusterrole*, serviceaccount), pdb, _helpers.tpl
├── crds/          # metacontroller-crds-v1.yaml — generated, do not edit
└── ci/            # one values file for each feature toggle; used by chart-testing
```

## RULES
- Keys in `values.yaml` are camelCase.
- Binary flags go in the `commandArgs` list as `--flag=value` strings. Do not
  add one value for each flag.
- Do not edit `crds/metacontroller-crds-v1.yaml`. Change the Go types and run
  `make generate_crds`. See [codegen.md](codegen.md).
- Do not edit `version` or `appVersion` in `Chart.yaml`. They move together, and
  the release job writes them.
- When you add a toggle to `values.yaml`, add a values file in `ci/` that
  turns it on. CI installs the chart one time for each file in `ci/`.
- The kustomize manifests in `manifests/` are a second, separate install
  path. A change in RBAC or in the StatefulSet usually applies to the two
  paths: `manifests/production/` and `templates/`. Do not mix their files.
- User documentation is `docs/src/guide/helm-install.md`.

## CI
`test_release.yaml`, job `helmtests`, runs on the newest Kubernetes version in
`.github/config/kind-matrix-config.json`:

```bash
ct lint-and-install --all --chart-dirs deploy/helm \
  --chart-yaml-schema .github/config/chart-testing/etc/chart_schema.yaml \
  --lint-conf .github/config/chart-testing/etc/lintconf.yaml \
  --validate-maintainers=false --check-version-increment=false
```

CI makes `ci/default-values.yaml` from `values.yaml` with the local `dev` image.
This file is not in the repo.

`helm_release.yaml` packages and pushes the chart when a `v*.*.*` tag is made.
