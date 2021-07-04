## BlueGreenDeployment

This is an example CompositeController that implements a custom rollout strategy
based on a technique called Blue-Green Deployment.

The controller ramps up a completely separate ReplicaSet in the background for any change to the
Pod template. It then waits for the new ReplicaSet to be fully Ready and Available
(all Pods satisfy minReadySeconds), and then switches a Service to point to the new ReplicaSet.
Finally, it scales down the old ReplicaSet.

### Prerequisites

* Install [Metacontroller](https://github.com/metacontroller/metacontroller)

### Deploy the controller

```sh
kubectl apply -k v1
```
(or pass `v1beta1` for kubernetes 1.15 or older)

### Create a BlueGreenDeployment

```sh
kubectl apply -f my-bluegreen.yaml
```
