local k8s = import "k8s.libsonnet";
local etcd = import "etcd.libsonnet";
local vitess = import "vitess.libsonnet";
local vtctld = import "vtctld.libsonnet";
local vtgate = import "vtgate.libsonnet";
local vttablet = import "vttablet.libsonnet";
local vtctlclient = import "vtctlclient.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Sync hook for VitessCell.
function(request) {
  // Wrap the raw request object to add functions.
  local observed = metacontroller.observed(request),

  // Whether a component is enabled in this cell.
  local enabled = function(name) name in observed.parent.spec,

  // VitessKeyspaces live within VitessCells because a given keyspace
  // doesn't have to be deployed across all cells.
  // VitessCluster passes the relevant VitessKeyspaces to each VitessCell.
  local keyspaces = vitess.keyspaces(observed, observed.parent.spec.keyspaces),

  // Vitess needs its own etcd cluster for internal coordination.
  // This requires etcd-operator to be installed and running in the
  // same namespace as your VitessCluster.
  local etcdClusters = etcd.clusters(observed,
    if enabled("etcd") then [observed.parent.spec.etcd]),

  // Vitess administrative server (vtctld).
  local vtctldSpecs =
    if enabled("vtctld") then [observed.parent.spec.vtctld],
  local vtctldServices = vtctld.services(observed, vtctldSpecs),
  local vtctldConfigMaps = vtctld.configMaps(observed, vtctldSpecs),
  local vtctldDeployments = vtctld.deployments(observed, vtctldSpecs),

  // Vitess query routers (vtgate).
  local vtgateSpecs =
    if enabled("vtgate") then [observed.parent.spec.vtgate],
  local vtgateServices = vtgate.services(observed, vtgateSpecs),
  local vtgateDeployments = vtgate.deployments(observed, vtgateSpecs),

  // Vitess database instances (vttablet).
  local vttabletServiceSpecs =
    if observed.parent.spec.name == "global" then [] else [observed.parent.spec],
  local vttabletServices = vttablet.services(observed, vttabletServiceSpecs),

  // vtctlclient Jobs
  local topoAddr = vitess.topoFlags(observed.parent.spec.cluster).topo_global_server_address,
  local cellName = observed.parent.spec.name,
  local vtctlclientSpecs = [] + (
    // For now, just share the global etcd since we're in a single k8s cluster.
    if cellName != "global" then [{
      name: "cell-info",
      command: ["UpdateCellInfo", "-server_address", topoAddr,
        "-root", "/vitess/" + cellName, cellName],
    }] else []
  ),
  local vtctlclientJobs = vtctlclient.jobs(observed, vtctlclientSpecs),

  // Aggregate status for a cell.
  status: {
    local status = self,
    local specKeyspaceNames = [spec.name for spec in keyspaces.specs],

    keyspaces:
      std.sort([ks.spec.name for ks in keyspaces.observed]),
    readyKeyspaces:
      std.sort([ks.spec.name for ks in k8s.filterReady(keyspaces.observed)]),

    etcd: {
      clusters: std.length(etcdClusters.observed),
      availableClusters: std.length(k8s.filterAvailable(etcdClusters.observed)),
    },
    vtctld: {
      services: std.length(vtctldServices.observed),
      configMaps: std.length(vtctldConfigMaps.observed),
      deployments: std.length(vtctldDeployments.observed),
      availableDeployments:
        std.length(k8s.filterAvailable(vtctldDeployments.observed)),
    },
    vtgate: {
      services: std.length(vtgateServices.observed),
      deployments: std.length(vtgateDeployments.observed),
      availableDeployments:
        std.length(k8s.filterAvailable(vtgateDeployments.observed)),
    },
    vttablet: {
      services: std.length(vttabletServices.observed),
    },
    cellInfoRegistered:
      if cellName == "global" then
        // Global is implicitly registered.
        true
      else
        vtctlclientJobs.isComplete("cell-info"),
    conditions: [
      k8s.condition("Ready",
        local expected = {
          [comp]: if enabled(comp) then 1 else 0
            for comp in ["etcd", "vtctld", "vtgate"]
        };

        status.readyKeyspaces == std.sort(specKeyspaceNames) &&
        status.etcd.availableClusters >= expected.etcd &&
        status.vtctld.services >= expected.vtctld &&
        status.vtctld.configMaps >= expected.vtctld &&
        status.vtctld.availableDeployments >= expected.vtctld &&
        status.vtgate.services >= expected.vtgate &&
        status.vtgate.availableDeployments >= expected.vtgate &&
        status.vttablet.services == std.length(vttabletServiceSpecs) &&
        status.cellInfoRegistered
      ),
    ],
  },

  // Child objects for this cell.
  children:
    keyspaces.desired +

    etcdClusters.desired +

    vtctldServices.desired +
    vtctldConfigMaps.desired +
    vtctldDeployments.desired +

    vtgateServices.desired +
    vtgateDeployments.desired +

    vttabletServices.desired +

    vtctlclientJobs.desired,
}
