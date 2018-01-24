local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Sync hook for VitessKeyspace.
function(request) {
  // Wrap the raw request object to add functions.
  local observed = metacontroller.observed(request),

  local shards = vitess.shards(observed,
    if "shards" in observed.parent.spec then
      observed.parent.spec.shards
    else
      vitess.unsharded),

  // Aggregate status of shards in the keyspace.
  status: {
    local status = self,
    local specShardNames = [spec.name for spec in shards.specs],

    shards:
      std.sort([s.spec.name for s in shards.observed]),
    readyShards:
      std.sort([s.spec.name for s in k8s.filterReady(shards.observed)]),

    conditions: [
      k8s.condition("Ready",
        status.readyShards == std.sort(specShardNames)
      ),
    ],
  },

  children: shards.desired,
}
