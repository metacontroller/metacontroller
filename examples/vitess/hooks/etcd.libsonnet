local k8s = import "k8s.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

{
  local etcd = self,

  apiVersion: "etcd.database.coreos.com/v1beta2",

  // EtcdClusters
  clusters(observed, specs)::
    metacontroller.collection(observed, specs, etcd.apiVersion, "EtcdCluster", etcd.cluster),

  // Create/update an EtcdCluster child for a VitessCell parent.
  cluster(observed, spec):: {
    apiVersion: etcd.apiVersion,
    kind: "EtcdCluster",
    metadata: {
      name: observed.parent.metadata.name + "-etcd",
      labels: observed.parent.spec.template.metadata.labels,
    },
    spec: {
      version: spec.version,
      size: spec.size,
    }
  },
}
