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
helm install metacontroller deploy/helm/metacontroller-helm-v*.tgz
```

## Installing chart from ghcr.io

Charts are published as [packages on ghcr.io](https://github.com/metacontroller/metacontroller/pkgs/container/metacontroller-helm)

You can pull them like:
* `HELM_EXPERIMENTAL_OCI=1 helm pull oci://ghcr.io/metacontroller/metacontroller-helm --version=4.11.19`

as OCI is currently (at least for helm 3.8.x) a beta feature.

## Configuration

| Parameter                     | Description                                                                                                                | Default                                                                              |
|-------------------------------|----------------------------------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------------|
| `command`                     | Command which is used to start metacontroller                                                                              | `/usr/bin/metacontroller`                                                            |
| `commandArgs`                 | Command arguments which are used to start metacontroller. See [configuration.md](configuration.md) for additional details. | `[ "--zap-log-level=4", "--discovery-interval=20s", "--cache-flush-interval=30m" ]`  |
| `rbac.create`                 | Create and use RBAC resources                                                                                              | `true`                                                                               |
| `image.repository`            | Image repository                                                                                                           | `metacontrollerio/metacontroller`                                                    |
| `image.pullPolicy`            | Image pull policy                                                                                                          | `IfNotPresent`                                                                       |
| `image.tag`                   | Image tag                                                                                                                  | `""` (`Chart.AppVersion`)                                                            |
| `imagePullSecrets`            | Image pull secrets                                                                                                         | `[]`                                                                                 |
| `nameOverride`                | Override the deployment name                                                                                               | `""` (`Chart.Name`)                                                                  |
| `namespaceOverride`           | Override the deployment namespace                                                                                          | `""` (`Release.Namespace`)                                                           |
| `fullnameOverride`            | Override the deployment full name                                                                                          | `""` (`Release.Namespace-Chart.Name`)                                                |
| `serviceAccount.create`       | Create service account                                                                                                     | `true`                                                                               |
| `serviceAccount.annotations`  | ServiceAccount annotations                                                                                                 | `{}`                                                                                 |
| `serviceAccount.name`         | Service account name to use, when empty will be set to created account if `serviceAccount.create` is set else to `default` | `""`                                                                                 |
| `podAnnotations`              | Pod annotations                                                                                                            | `{}`                                                                                 |
| `podSecurityContext`          | Pod security context                                                                                                       | `{}`                                                                                 |
| `securityContext`             | Container security context                                                                                                 | `{}`                                                                                 |
| `resources`                   | CPU/Memory resource requests/limits                                                                                        | `{}`                                                                                 |
| `nodeSelector`                | Node labels for pod assignment                                                                                             | `{}`                                                                                 |
| `tolerations`                 | Toleration labels for pod assignment                                                                                       | `[]`                                                                                 |
| `affinity`                    | Affinity settings for pod assignment                                                                                       | `{}`                                                                                 |
| `priorityClassName`           | The name of the `PriorityClass` that will be assigned to metacontroller                                                    | `""`                                                                                 |
| `clusterRole.aggregationRule` | The `aggregationRule` applied to metacontroller `ClusterRole`                                                              | `{}`                                                                                 |
| `clusterRole.rules`           | The `rules` applied to metacontroller `ClusterRole`                                                                        | ```{ "apiGroups": "*", "resources": "*", "verbs": "*" }```                           |
| `replicas`                    | Specifies the number of metacontroller pods that will be deployed                                                          | `1`                                                                                  |
| `podDisruptionBudget`         | The `podDisruptionBudget` applied to metacontroller `pods`                                                                 | `{}`                                                                                 |
| `service.enabled`             | If `true`, then create a `Service` to expose ports                                                                         | `false`                                                                              |
| `service.ports`               | List of ports that are exposed on the `Service`                                                                            | `[]`                                                                                 |
