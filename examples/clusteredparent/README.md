## ClusterRole service account binding

This is an example DecoratorController that creates a namespaced resources from a
cluster scoped parent resource.

This controller will bind any ClusterRole with the "default-service-account-binding"
annotation to the default service account in the default namespace.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap cluster-parent-controller -n metacontroller --from-file=sync.py
kubectl apply -f cluster-parent.yaml
```

### Create a ClusterRole

```sh
kubectl apply -f my-clusterole.yaml
```

A RoleBinding should be created for the ClusterRole:

```console
$ kubectl get rolebinding -n default my-clusterrole -o wide
NAME             AGE       ROLE                         USERS     GROUPS    SERVICEACCOUNTS
my-clusterrole   40s       ClusterRole/my-clusterrole                       default/default
```
