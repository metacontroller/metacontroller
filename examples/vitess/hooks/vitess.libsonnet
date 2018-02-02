local k8s = import "k8s.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Library for working with Vitess objects.
{
  local vitess = self,

  apiVersion: "vitess.io/v1alpha1",

  // Default flags shared by multiple Vitess components.
  baseFlags: {
    logtostderr: true,
  },
  serverFlags: vitess.baseFlags + {
    port: 15000,
    grpc_port: 15999,
  },
  topoFlags(cluster):: {
    topo_implementation: "etcd2",
    topo_global_root: "/vitess/global",
    topo_global_server_address:
      "http://%s-global-etcd-client:2379" % cluster,
  },

  // Collections of Vitess objects.
  cells(observed, specs)::
    metacontroller.collection(observed, specs, vitess.apiVersion, "VitessCell", vitess.cell),
  keyspaces(observed, specs)::
    metacontroller.collection(observed, specs, vitess.apiVersion, "VitessKeyspace", vitess.keyspace),
  shards(observed, specs)::
    local shardSpecs = [vitess.shardSpec(spec) for spec in specs];
    metacontroller.collection(observed, shardSpecs, vitess.apiVersion, "VitessShard", vitess.shard),

  // Create/update a VitessCell child for a VitessCluster parent.
  cell(observed, spec):: {
    apiVersion: vitess.apiVersion,
    kind: "VitessCell",
    metadata: {
      name: observed.parent.metadata.name + "-" + spec.name,
      labels: observed.parent.spec.template.metadata.labels,
    },
    // Each VitessCell spec starts from a VitessCluster.spec.cells item.
    spec: spec + {
      // Pass down info from the parent that children will need.
      cluster: observed.parent.metadata.name,
      backupFlags:
        if "backupFlags" in observed.parent.spec && observed.parent.spec.backupFlags != null then
          observed.parent.spec.backupFlags
        else {},

      // Propagate the selector and child template from the parent.
      // Add a "cell" label that will be applied to all child objects
      // of this VitessCell.
      selector: observed.parent.spec.selector + {
        matchLabels+: {
          "vitess.io/cell": spec.name,
        },
      },
      template: observed.parent.spec.template + {
        metadata+: {
          labels+: {
            "vitess.io/cell": spec.name,
          },
        },
      },

      // Add keyspaces that are enabled in this cell.
      // This way you don't have to duplicate the spec in every cell.
      keyspaces:
        std.filter(
          function(ks) std.count(ks.cells, spec.name) > 0, observed.parent.spec.keyspaces),
    },
  },

  // Create/update a VitessKeyspace child for a VitessCell parent.
  keyspace(observed, spec):: {
    apiVersion: vitess.apiVersion,
    kind: "VitessKeyspace",
    metadata: {
      name: observed.parent.metadata.name + "-" + spec.name,
      labels: observed.parent.spec.template.metadata.labels,
    },
    // Each VitessKeyspace spec starts from a VitessCell.spec.keyspaces item.
    spec: spec + {
      // Pass down info from the parent that children will need.
      cluster: observed.parent.spec.cluster,
      cell: observed.parent.spec.name,
      backupFlags: observed.parent.spec.backupFlags,

      // Propagate the selector and child template from the parent.
      // Add a "keyspace" label that will be applied to all child objects
      // of this VitessKeyspace.
      selector: observed.parent.spec.selector + {
        matchLabels+: {
          "vitess.io/keyspace": spec.name,
        },
      },
      template: observed.parent.spec.template + {
        metadata+: {
          labels+: {
            "vitess.io/keyspace": spec.name,
          },
        },
      },
    },
  },

  // Create/update a VitessShard child for a VitessKeyspace parent.
  shard(observed, spec):: {
    apiVersion: vitess.apiVersion,
    kind: "VitessShard",
    metadata: {
      name: observed.parent.metadata.name + "-" + spec.kname,
      labels: observed.parent.spec.template.metadata.labels,
    },
    // Each VitessShard spec starts from a VitessKeyspace.spec.shards item.
    spec: spec + {
      // Pass down info from the parent that children will need.
      cluster: observed.parent.spec.cluster,
      cell: observed.parent.spec.cell,
      keyspace: observed.parent.spec.name,
      backupFlags: observed.parent.spec.backupFlags,

      // Propagate the selector and child template from the parent.
      // Add a "shard" label that will be applied to all child objects
      // of this VitessShard.
      selector: observed.parent.spec.selector + {
        matchLabels+: {
          "vitess.io/shard": spec.kname,
        },
      },
      template: observed.parent.spec.template + {
        metadata+: {
          labels+: {
            "vitess.io/shard": spec.kname,
          },
        },
      },

      // Propagate tablet specs from the keyspace.
      // This way you don't have to duplicate the spec in every shard.
      tablets: observed.parent.spec.tablets,
    },
  },

  // Extend a shard spec.
  shardSpec(spec)::
    if "keyRange" in spec then
      // Range-based shard.
      spec + {
        // The Vitess shard name.
        name: self.keyRange.start + "-" + self.keyRange.end,

        // The Kubernetes-safe name (can't start or end with "-").
        kname::
          (if self.keyRange.start != "" then self.keyRange.start else "x") +
          "-" +
          (if self.keyRange.end != "" then self.keyRange.end else "x"),
      }
    else
      // Custom shard or unsharded.
      spec + {
        // Custom shard names should just be integers.
        kname:: self.name,
      },

  // Shard spec list for an "unsharded" keyspace.
  unsharded: [{
    name: "0",
  }],

  // Format key-value pairs (object fields) into
  // a flags string for a Vitess binary.
  formatFlags(flags)::
    std.join(" ", [
      "-%s=\"%s\"" % [key,flags[key]] for key in std.objectFields(flags)
    ]),

  // Compute a deterministic tablet UID (32-bit unsigned int) for a tablet spec.
  // Note that this is an arbitrary algorithm, not necessarily shared by any
  // other methods of deploying Vitess.
  tabletUid(spec)::
    // Make a string that's unique within a given cluster (even across cells).
    local str = std.join("/", [spec.cell, spec.keyspace, spec.shard, spec.type]);
    // Checksum the string, take the first 24 bits, convert to integer.
    local hash = metacontroller.parseInt(std.md5(str)[:6], 16);
    // Shift left 2 decimal digits, add index.
    hash * 100 + spec.index,
}
