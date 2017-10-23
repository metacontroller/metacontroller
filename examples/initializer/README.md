## Initializer

This is an example InitializerController that copies some deprecated Pod annotations to
the corresponding new fields.

For this example, you need a cluster with alpha features enabled (for initializers),
and it must be v1.6.11+, v1.7.7+, or v1.8.0+ so that the initializer can modify
certain Pod fields that normally are immutable.

### Prerequisites

* [Install Metacontroller](https://github.com/GoogleCloudPlatform/kube-metacontroller#install)

### Deploy the controller

```sh
kubectl create configmap podhostname-initializer -n metacontroller --from-file=init.js
kubectl apply -f podhostname-initializer.yaml
```

### Enable the initializer config

Due to current limitations in the alpha initializer feature, it's possible to deadlock
the creation of a Pod implementing an initializer controller, while the server waits for
that initializer to process its own Pod.

As a workaround, you should ensure the initializer controller Pod is Running before enabling the
`InitializerConfiguration`.

```sh
kubectl -n metacontroller get po -l app=podhostname-initializer

kubectl apply -f initializer-config.yaml
```

### Create a Pod

Create a Pod that still uses only the old annotations, rather than the fields.
Without the initializer, the Pod DNS would not work in v1.7+ because the annotations
no longer have any effect.

```sh
kubectl apply -f bad-pod.yaml
```

Note that in the Metacontroller proof-of-concept, the latency can be up to the poll interval of 5s.
When Metacontroller is updated to use watches, this limitation will no longer apply.

Verify that the initializer properly copied the annotations to fields:

```sh
kubectl get pod bad-pod -o yaml | grep -E 'hostname|subdomain'
```
