local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

{
  local vtctld = self,

  // Filter for vtctld objects among other children of VitessCell.
  matchName(obj)::
    std.endsWith(obj.metadata.name, "-vtctld"),

  // Collections of vtctld objects.
  services(observed, specs)::
    metacontroller.collection(observed, specs, "v1", "Service", vtctld.service)
      + metacontroller.collectionFilter(vtctld.matchName),
  configMaps(observed, specs)::
    metacontroller.collection(observed, specs, "v1", "ConfigMap", vtctld.configMap)
      + metacontroller.collectionFilter(vtctld.matchName),
  deployments(observed, specs)::
    metacontroller.collection(observed, specs, "apps/v1beta2", "Deployment", vtctld.deployment)
      + metacontroller.collectionFilter(vtctld.matchName),

  // Create/update a Service for vtctld.
  service(observed, spec):: {
    apiVersion: "v1",
    kind: "Service",
    metadata: {
      name: observed.parent.metadata.name + "-vtctld",
      labels: observed.parent.spec.template.metadata.labels,
    },
    spec: {
      selector: observed.parent.spec.selector.matchLabels + {
        "vitess.io/component": "vtctld",
      },
      ports: [
        {name: "web",  port: 15000},
        {name: "grpc", port: 15999},
      ],
    },
  },

  // Create/update a Deployment for vtctld.
  deployment(observed, spec):: {
    local vtctldFlags = {
      cell: observed.parent.spec.name,
      web_dir: "/vt/web/vtctld",
      web_dir2: "/vt/web/vtctld2/app",
      workflow_manager_init: true,
      workflow_manager_use_election: true,
      service_map: "grpc-vtctl",
    },
    local flags = vitess.serverFlags
      + vitess.topoFlags(observed.parent.spec.cluster)
      + vtctldFlags
      + observed.parent.spec.backupFlags
      + (if "flags" in spec then spec.flags else {}),

    apiVersion: "apps/v1beta2",
    kind: "Deployment",
    metadata: {
      name: observed.parent.metadata.name + "-vtctld",
      labels: observed.parent.spec.template.metadata.labels,
    },
    spec: {
      replicas: spec.replicas,
      selector: observed.parent.spec.selector + {
        matchLabels+: {
          "vitess.io/component": "vtctld",
        },
      },
      template: {
        metadata: {
          labels: observed.parent.spec.template.metadata.labels + {
            "vitess.io/component": "vtctld",
          },
        },
        spec: {
          securityContext: {runAsUser: 999, fsGroup: 999},
          initContainers: [
            {
              name: "init-vtctld",
              image: spec.image,
              command: ["bash", "-c", |||
                set -ex
                rm -rf /vt/web/*
                cp -R $VTTOP/web/* vt/web/
                cp /mnt/config/config.js /vt/web/vtctld/
              |||],
              volumeMounts: [
                {name: "config", mountPath: "/mnt/config"},
                {name: "web", mountPath: "/vt/web"},
              ],
            },
          ],
          containers: [
            {
              name: "vtctld",
              image: spec.image,
              livenessProbe: {
                httpGet: {path: "/debug/vars", port: 15000},
                initialDelaySeconds: 30,
                timeoutSeconds: 5,
              },
              volumeMounts: [
                {name: "vtdataroot", mountPath: "/vt/vtdataroot"},
                {name: "web", mountPath: "/vt/web"},
              ],
              resources: spec.resources,
              command: ["bash", "-c",
                "set -ex; exec /vt/bin/vtctld " +
                vitess.formatFlags(flags)],
            },
          ],
          volumes: [
            {name: "vtdataroot", emptyDir: {}},
            {name: "web", emptyDir: {}},
            {
              name: "config",
              configMap: {
                name: observed.parent.metadata.name + "-vtctld",
              },
            },
          ],
        },
      },
    },
  },

  configMap(observed, spec):: {
    apiVersion: "v1",
    kind: "ConfigMap",
    metadata: {
      name: observed.parent.metadata.name + "-vtctld",
      labels: observed.parent.spec.template.metadata.labels,
    },
    data: {
      // Customize the vtctld web UI for Kubernetes.
      "config.js": |||
        vtconfig = {
          k8s_proxy_re: /(\/api\/v1\/namespaces\/.*)\/services\//,
          tabletLinks: function(tablet) {
            status_href = 'http://'+tablet.hostname+':'+tablet.port_map.vt+'/debug/status'

            // If we're going through the Kubernetes API server proxy,
            // route tablet links through the proxy as well.
            var match = window.location.pathname.match(vtconfig.k8s_proxy_re);
            if (match) {
              var hostname = tablet.hostname.split('.');
              var alias = hostname[0];
              var subdomain = hostname[1];
              status_href = match[1] + '/pods/' + subdomain + '-' + alias.split('-')[1] + ':' + tablet.port_map.vt + '/proxy/debug/status';
            }

            return [
              {
                title: 'Status',
                href: status_href
              }
            ];
          }
        };
      |||,
    },
  },
}
