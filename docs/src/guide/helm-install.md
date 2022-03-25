# Install Metacontroller using Helm

## Building the chart from source code

The chart can be built from metacontroller source:

```shell
git clone https://github.com/metacontroller/metacontroller.git
cd  metacontroller
helm package deploy/helm/metacontroller --destination deploy/helm
```

## Installing the chart from package

```shell
helm install metacontroller deploy/helm/metacontroller-v*.tgz
```

## Installing chart from ghcr.io

Charts are published as [packages on ghcr.io](https://github.com/metacontroller/metacontroller/pkgs/container/metacontroller-helm)

You can pull them like:
* `HELM_EXPERIMENTAL_OCI=1 helm pull oci://ghcr.io/metacontroller/metacontroller-helm --version=v2.2.5`

as OCI is currently (at least for helm 3.8.x) a beta feature.

## Configuration

| Parameter                     | Description                                                                                                                | Default                                                   |
|-------------------------------|----------------------------------------------------------------------------------------------------------------------------|-----------------------------------------------------------|
| `rbac.create`                 | Create and use RBAC resources                                                                                              | `true`                                                    |
| `image.repository`            | Image repository                                                                                                           | `metacontrollerio/metacontroller`                         |
| `image.pullPolicy`            | Image pull policy                                                                                                          | `IfNotPresent`                                            |
| `image.tag`                   | Image tag                                                                                                                  | `""` (`Chart.AppVersion`)                                 |
| `imagePullSecrets`            | Image pull secrets                                                                                                         | `[]`                                                      |
| `nameOverride`                | Override the deployment name                                                                                               | `""` (`Chart.Name`)                                       |
| `namespaceOverride`           | Override the deployment namespace                                                                                          | `""` (`Release.Namespace`)                                |
| `fullnameOverride`            | Override the deployment full name                                                                                          | `""` (`Release.Namespace-Chart.Name`)                     |
| `serviceAccount.create`       | Create service account                                                                                                     | `true`                                                    |
| `serviceAccount.annotations`  | ServiceAccount annotations                                                                                                 | `{}`                                                      |
| `serviceAccount.name`         | Service account name to use, when empty will be set to created account if `serviceAccount.create` is set else to `default` | `""`                                                      |
| `podAnnotations`              | Pod annotations                                                                                                            | `{}`                                                      |
| `podSecurityContext`          | Pod security context                                                                                                       | `{}`                                                      |
| `securityContext`             | Container security context                                                                                                 | `{}`                                                      |
| `resources`                   | CPU/Memory resource requests/limits                                                                                        | `{}`                                                      |
| `nodeSelector`                | Node labels for pod assignment                                                                                             | `{}`                                                      |
| `tolerations`                 | Toleration labels for pod assignment                                                                                       | `[]`                                                      |
| `affinity`                    | Affinity settings for pod assignment                                                                                       | `{}`                                                      |
| `zap.logLevel`                | Zap Level to configure the verbosity of logging                                                                            | `4`                                                       |
| `zap.devel`                   | Development Mode or Production Mode                                                                                        | `"production"`                                            |
| `zap.encoder`                 | Zap log encoding (‘json’ or ‘console’)                                                                                     | `"json"`                                                  |
| `zap.stacktraceLevel`         | Zap Level at and above which stacktraces are captured (one of ‘info’ or ‘error’)                                           | `"info"`                                                  |
| `commandArgs`                 | Custom arguments which are used to start metacontroller                                                                    | `[]`                                                      |
| `discoveryInterval`           | How often to refresh discovery cache to pick up newly-installed resources                                                  | `"20s"`                                                   |
| `cacheFlushInterval`          | How often to flush local caches and relist objects from the API server                                                     | `30m`                                                     |
| `priorityClassName`           | The name of the `PriorityClass` that will be assigned to metacontroller                                                    | `""`                                                      |
| `clusterRole.aggregationRule` | The `aggregationRule` applied to metacontroller `ClusterRole`                                                              | `{}`                                                      |
| `clusterRole.rules`           | The `rules` applied to metacontroller `ClusterRole`                                                                        | ```{ "apiGroups": "*", "resources": "*", "verbs": "*" }```|
| `replicas`                    | Specifies the number of metacontroller pods that will be deployed                                                          | `1`                                                       |
| `podDisruptionBudget`         | The `podDisruptionBudget` applied to metacontroller `pods`                                                                 | `{}`                                                      |
