## Example Go Controller

This controller doesn't do anything useful.
It's just an example skeleton for writing Metacontroller hooks with Go.

**WARNING**

There's a [known issue](https://github.com/GoogleCloudPlatform/metacontroller/issues/76)
that makes it difficult to produce JSON according to the rules that Metacontroller
requires if you import the official Go structs for Kubernetes APIs.
In particular, some fields will always be emitted, even if you never set them,
which goes against Metacontroller's [apply semantics](https://metacontroller.app/api/apply/).

### Prerequisites

* [Install Metacontroller](https://metacontroller.app/guide/install/)

### Install Thing Controller

```sh
kubectl apply -f thing-controller.yaml
```

### Create a Thing

```sh
kubectl apply -f my-thing.yaml
```

Look at the thing:

```sh
kubectl get thing -o yaml
```

Look at the thing the thing created:

```sh
kubectl get pod thing-1 -a
```

Look at what the thing the thing created said:

```sh
kubectl logs thing-1
```

### Clean up

```sh
kubectl delete -f thing-controller.yaml
```

### Building

You don't need to build to run the example above,
but if you make changes:

```sh
go get -u github.com/golang/dep/cmd/dep
dep ensure
go build -o thing-controller
```

Or just make a new container image:

```sh
docker build . -t <yourname>/thing-controller
```
