## Target Label Selector

This example shows you how to configure metacontroller to use a [label selector](https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#label-selectors) to restrict an instance of metacontroller to manage specific Composite and Decorator controllers, which enables the ability to run multiple metacontroller instances on the same cluster (e.g. `--target-label-selector=controller-group=cicd`

See the [configuration guide](https://metacontroller.github.io/metacontroller/guide/configuration.html) for more information.

### Prerequisites

Install metacontroller which has the `target-label-selector` defined:

```sh
kubectl apply -k instance
```

### Deploy the controller

```sh
kubectl apply -k manifest
```

### Create an example secret, several namespaces and SecretPropagation custom resource

```sh
kubectl apply -f ../secretpropagation/example-secret.yaml
```

A Secret will be created in namespaces `alpha` and `beta`:

```console
$ kubectl get secret shareable -n alpha
NAME        TYPE     DATA   AGE
shareable   Opaque   2      8m56s
$ kubectl get secret shareable -n beta
NAME        TYPE     DATA   AGE
shareable   Opaque   2      9m25s
```

, but not `gamma` (as the last one does not have matching labels):

```console
$ kubectl get secret shareable -n gamma
Error from server (NotFound): secrets "shareable" not found
```

### e2e test

Try out the end-to-end test in this folder `./test.sh`. Open it up to see more detail.
