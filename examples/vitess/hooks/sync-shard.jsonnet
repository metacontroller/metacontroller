local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local vttablet = import "vttablet.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Sync hook for VitessShard.
function(request) {
  // Wrap the raw request object to add functions.
  local observed = metacontroller.observed(request),

  // Defaults that apply to all tablet types.
  local tabletDefaults =
    if "defaults" in observed.parent.spec.tablets then
      observed.parent.spec.tablets.defaults
    else {},

  // Generate individual tablet specs (not written by users explicitly).
  local tabletSpec = function(spec) spec + {
    cluster: observed.parent.spec.cluster,
    cell: observed.parent.spec.cell,
    keyspace: observed.parent.spec.keyspace,
    shard: observed.parent.spec.name,
    uid: vitess.tabletUid(self),
    uidString: "%010d" % self.uid,
    alias: self.cell + "-" + self.uidString,
    subdomain: "%s-vttablet-%s" % [self.cluster, self.cell],
  },
  local tabletSpecs = (
    // "replica" type tablets
    if "masterEligible" in observed.parent.spec.tablets then
      local spec = tabletDefaults + observed.parent.spec.tablets.masterEligible;
      [tabletSpec(spec + {type: "replica", index: i}) for i in std.range(0, spec.replicas - 1)]
    else []
  ) + (
    // "rdonly" type tablets
    if "batch" in observed.parent.spec.tablets then
      local spec = tabletDefaults + observed.parent.spec.tablets.batch;
      [tabletSpec(spec + {type: "rdonly", index: i}) for i in std.range(0, spec.replicas - 1)]
    else []
  ),

  // Collect observed and compute desired objects for tablets.
  local volumes = vttablet.volumes(observed, tabletSpecs),
  local pods = vttablet.pods(observed, tabletSpecs),

  // Aggregate status of tablets in the shard.
  status: {
    local status = self,
    local specTabletUids = [spec.uidString for spec in tabletSpecs],

    tablets:
      std.sort([vttablet.getUid(t) for t in pods.observed]),
    readyTablets:
      std.sort([vttablet.getUid(t) for t in k8s.filterReady(pods.observed)]),

    conditions: [
      k8s.condition("Ready",
        status.readyTablets == std.sort(specTabletUids)
      ),
    ],
  },

  children:
    volumes.desired +
    pods.desired,
}
