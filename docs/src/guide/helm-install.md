# Install Metacontroller using Helm

## Building the chart

The chart can be built from metacontroller source:

```shell
git clone https://github.com/metacontroller/metacontroller.git
cd  metacontroller
helm package deploy/helm/metacontroller --destination deploy/helm
```

## Installing the chart

```shell
helm install metacontroller deploy/helm/metacontroller-v*.tgz
```

## Configuration

| Parameter                                 | Description                                   | Default                                                 |
|-------------------------------------------|-----------------------------------------------|---------------------------------------------------------|
| `rbac.create`                             | Create and use RBAC resources                 | `true`                                                  |
| `image.repository`                        | Image repository                              | `metacontrollerio/metacontroller`                       |
| `image.pullPolicy`                        | Image pull policy                             | `IfNotPresent`                                          |
| `image.tag`                               | Image tag                                     | `""` (`Chart.AppVersion`)                               |
| `imagePullSecrets`                        | Image pull secrets                            | `[]`                                                    |
| `nameOverride`                            | Override the deployment name                  | `""` (`Chart.Name`)                                     |
| `namespaceOverride`                       | Override the deployment namespace             | `""` (`Release.Namespace`)                              |
| `fullnameOverride`                        | Override the deployment full name             | `""` (`Release.Namespace-Chart.Name`)                   |
| `serviceAccount.create`                   | Create service account                        | `true`                                                  |
| `serviceAccount.annotations`              | ServiceAccount annotations                    | `{}`                                                    |
| `serviceAccount.name`                     | Service account name to use, when empty will be set to created account if `serviceAccount.create` is set else to `default` | `""` |
| `podAnnotations`                          | Pod annotations                               | `{}`                                                    |
| `podSecurityContext`                      | Pod security context                          | `{}`                                                    |
| `securityContext`                         | Container security context                    | `{}`                                                    |
| `resources`                               | CPU/Memory resource requests/limits           | `{}`                                                    |
| `nodeSelector`                            | Node labels for pod assignment                | `{}`                                                    |
| `tolerations`                             | Toleration labels for pod assignment          | `[]`                                                    |
| `affinity`                                | Affinity settings for pod assignment          | `{}`                                                    |
