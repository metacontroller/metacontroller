## GlobalConfigMap

This is an example CompositeController that propagates a speficied configmap to given namespaces. It uses `customize` hook to select ConfigMap for propagation, and all namespaces to populate ConfgMap into. Please note that we ignore `labelSelector` by setting it to empty one, to select related resources just by namespace/name.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap configmap-propagation-controller -n metacontroller --from-file=sync.py
kubectl apply -f configmap-propagation.yaml
```

### Create an example configmap, several namespaces and ConfigMapPropagation custom resource

```sh
kubectl apply -f example-configmap.yaml
```

A ConfigMap will be created in every namespace.

```console
$ kubectl get cm --all-namespaces
NAMESPACE            NAME                                 DATA   AGE
default              globalsettings                       2      7s
first                globalsettings                       2      7s
global               globalsettings                       2      27s
kube-node-lease      globalsettings                       2      6s
kube-public          cluster-info                         1      198d
kube-public          globalsettings                       2      7s
kube-system          coredns                              1      198d
kube-system          extension-apiserver-authentication   6      198d
kube-system          globalsettings                       2      7s
kube-system          kube-proxy                           2      198d
kube-system          kubeadm-config                       2      198d
kube-system          kubelet-config-1.18                  1      198d
local-path-storage   globalsettings                       2      7s
local-path-storage   local-path-config                    1      198d
metacontroller       globalconfigmap-controller           1      28s
metacontroller       globalsettings                       2      7s
second               globalsettings                       2      6s
third                globalsettings                       2      7s
```


Also, adding new namespace will trigger controller and new configmap will be also created there.
