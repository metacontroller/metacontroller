## BlueGreenDeployment

This is an example CompositeController that implements a custom rollout strategy
based on a technique called Blue-Green Deployment.

The controller ramps up a completely separate ReplicaSet in the background for any change to the
Pod template. It then waits for the new ReplicaSet to be fully Ready and Available
(all Pods satisfy minReadySeconds), and then switches a Service to point to the new ReplicaSet.
Finally, it scales down the old ReplicaSet.

### Prerequisites

* Install [Metacontroller](https://github.com/GoogleCloudPlatform/metacontroller)

### Deploy the controller

```sh
kubectl create configmap bluegreen-controller -n metacontroller --from-file=sync.js
kubectl apply -f bluegreen-controller.yaml
```

### Create a BlueGreenDeployment

```sh
kubectl apply -f my-bluegreen.yaml
```
