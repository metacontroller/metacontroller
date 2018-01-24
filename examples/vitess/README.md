## Vitess Operator

This is an example of an app-specific [Operator](https://coreos.com/operators/), 
in this case for [Vitess](http://vitess.io), built with Metacontroller.

It's meant to demonstrate the following patterns:

* Building an Operator for a complex, stateful application out of a set of small
  Lambda Controllers that each do one thing well.
  * In addition to presenting a k8s-style API to users, this Operator uses
    custom k8s API objects to coordinate within itself.
  * Each controller manages one layer of the hierarchical Vitess cluster topology.
    The user only needs to create and manage a single, top-level VitessCluster
    object.
* Replacing static, client-side template rendering with Lambda Controllers,
  which can adjust based on dynamic cluster state.
  * Each controller aggregates status and orchestrates app-specific rolling
    updates for its immediate children.
  * The top-level object contains a continuously-updated, aggregate "Ready"
    condition for the whole app, and can be directly edited to trigger rolling
    updates throughout the app.
* Using a functional-style language ([Jsonnet](http://jsonnet.org)) to
  define Lambda Controllers in terms of template-like transformations on JSON
  objects.
  * You can use any language to write a Lambda Controller webhook, but the
    functional style is a good fit for a process that conceptually consists of
    declarative input, declarative output, and no side effects.
  * As a JSON templating language, Jsonnet is a particularly good fit for
    generating k8s manifests, providing functionality missing from pure
    JavaScript, such as first-class *merge* and *deep equal* operations.
* Using the "Apply" update strategy feature of CompositeController, which
  emulates the behavior of `kubectl apply`, except that it attempts to do
  pseudo-strategic merges for CRDs.

### Vitess Components

A typical VitessCluster might expand to the following tree once it's fully
deployed.
Objects in **bold** are custom resource kinds defined by this Operator.

* **VitessCluster**: The top-level specification for a Vitess cluster.
  This is the only one the user creates.
  * **VitessCell**: Each Vitess [cell](http://vitess.io/overview/concepts/#cell-data-center)
    represents an independent failure domain (e.g. a Zone or Availability Zone).
    * EtcdCluster ([etcd-operator](https://github.com/coreos/etcd-operator)):
      Vitess needs its own etcd cluster to coordinate its built-in load-balancing
      and automatic shard routing.
    * Deployment ([orchestrator](https://github.com/github/orchestrator)):
      An optional automated failover tool that works with Vitess.
    * Deployment ([vtctld](http://vitess.io/overview/#vtctld)):
      A pool of stateless Vitess admin servers, which serve a dashboard UI as well
      as being an endpoint for the Vitess CLI tool (vtctlclient).
    * Deployment ([vtgate](http://vitess.io/overview/#vtgate)):
      A pool of stateless Vitess query routers.
      The client application can use any one of these vtgate Pods as the entry
      point into Vitess, through a MySQL-compatible interface.
    * **VitessKeyspace** (db1): Each Vitess [keyspace](http://vitess.io/overview/concepts/#keyspace)
      is a logical database that may be composed of many MySQL databases (shards).
      * **VitessShard** (db1/0): Each Vitess [shard](http://vitess.io/overview/concepts/#shard)
      is a single-master tree of replicating MySQL instances.
        * Pod(s) ([vttablet](http://vitess.io/overview/#vttablet)): Within a shard, there may be many Vitess [tablets](http://vitess.io/overview/concepts/#tablet)
          (individual MySQL instances).
          VitessShard acts like an app-specific [replacement for StatefulSet](https://github.com/GoogleCloudPlatform/kube-metacontroller/tree/master/examples/catset),
          creating both Pods and PersistentVolumeClaims.
        * PersistentVolumeClaim(s)
      * **VitessShard** (db1/1)
        * Pod(s) (vttablet)
        * PersistentVolumeClaim(s)
    * **VitessKeyspace** (db2)
      * **VitessShard** (db2/0)
        * Pod(s) (vttablet)
        * PersistentVolumeClaim(s)

### Prerequisites

* Kubernetes 1.8+ is required for its improved CRD support, especially garbage
  collection.
  * This config currently requires a dynamic PersistentVolume provisioner and a
    default StorageClass.
  * The example `my-vitess.yaml` config results in a lot of Pods.
    If the Pods don't schedule due to resource limits, you can try lowering the
    limits, lowering `replicas` values, or removing the `batch` config under
    `tablets`.
* Install [kube-metacontroller](https://github.com/GoogleCloudPlatform/kube-metacontroller).
* Install [etcd-operator](https://github.com/coreos/etcd-operator) in the
  namespace where you plan to create a VitessCluster.

### Deploy the Operator

```sh
kubectl create configmap vitess-operator-hooks -n metacontroller --from-file=hooks
kubectl apply -f vitess-operator.yaml
```

### Create a VitessCluster

```sh
kubectl apply -f my-vitess.yaml
```

### View the Vitess Dashboard

Wait until the cluster is ready:

```sh
kubectl get vitessclusters -o 'custom-columns=NAME:.metadata.name,READY:.status.conditions[?(@.type=="Ready")].status'
```

You should see:

```console
NAME      READY
vitess    True
```

Start a kubectl proxy:

```sh
kubectl proxy --port=8001
```

Then visit:

```
http://localhost:8001/api/v1/namespaces/default/services/vitess-global-vtctld:web/proxy/app/
```
