local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Sync hook for VitessCluster.
function(request) {
  // Wrap the raw request object to add functions.
  local observed = metacontroller.observed(request),

  // Everything lives in one of the VitessCells.
  local cells = vitess.cells(observed, observed.parent.spec.cells),

  // Aggregate status of all VitessCells.
  status: {
    local status = self,
    local specCellNames = [spec.name for spec in cells.specs],

    cells:
      std.sort([cell.spec.name for cell in cells.observed]),
    readyCells:
      std.sort([cell.spec.name for cell in k8s.filterReady(cells.observed)]),
    conditions: [
      k8s.condition("Ready", status.readyCells == std.sort(specCellNames)),
    ],
  },

  // Children of this VitessCluster.
  children:
    cells.desired,
}
