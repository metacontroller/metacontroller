local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

// Library for running vtctlclient commands as Jobs.
{
  local vtctlclient = self,

  image: "vitess/vtctlclient",

  // Filter for vtctlclient Jobs among other child Jobs.
  matchJob(obj)::
    k8s.matchLabels(obj, {"vitess.io/component": "vtctlclient"}),

  jobs(observed, specs)::
    metacontroller.collection(observed, specs, "batch/v1", "Job", vtctlclient.job)
      + metacontroller.collectionFilter(vtctlclient.matchJob)
      + metacontroller.collectionImmutable
      + {
        isComplete(specName)::
          local name = observed.parent.metadata.name + "-" + specName;
          k8s.conditionStatus(super.getObserved(name), "Complete") == "True",
      },

  // Create/update a Job.
  job(observed, spec):: {
    // The metadata.name of the Job.
    local labels = observed.parent.spec.template.metadata.labels + {
      "vitess.io/component": "vtctlclient",
    },
    local vtctldAddr = "%s-global-vtctld:%d" %
      [observed.parent.spec.cluster, vitess.serverFlags.grpc_port],

    apiVersion: "batch/v1",
    kind: "Job",
    metadata: {
      name: observed.parent.metadata.name + "-" + spec.name,
      labels: labels,
    },
    spec: {
      activeDeadlineSeconds: 60,
      backoffLimit: 10,
      template: {
        metadata: {
          labels: labels,
        },
        spec: {
          restartPolicy: "OnFailure",
          containers: [
            {
              name: "vtctlclient",
              image: vtctlclient.image,
              command: ["vtctlclient", "-server", vtctldAddr] + spec.command,
            },
          ],

        },
      },
    },
  },
}
