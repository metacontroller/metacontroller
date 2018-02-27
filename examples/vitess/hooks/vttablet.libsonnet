local k8s = import "k8s.libsonnet";
local vitess = import "vitess.libsonnet";
local metacontroller = import "metacontroller.libsonnet";

{
  local vttablet = self,

  // Filter for vttablet-related objects among other children of VitessShard.
  matchLabels(obj)::
    k8s.matchLabels(obj, {"vitess.io/component": "vttablet"}),

  // Collections of vttablet objects.
  pods(observed, specs)::
    metacontroller.collection(observed, specs, "v1", "Pod", vttablet.pod)
      + metacontroller.collectionFilter(vttablet.matchLabels),
  volumes(observed, specs)::
    metacontroller.collection(observed, specs, "v1", "PersistentVolumeClaim", vttablet.volume)
      + metacontroller.collectionFilter(vttablet.matchLabels),
  services(observed, specs)::
    metacontroller.collection(observed, specs, "v1", "Service", vttablet.service)
      + metacontroller.collectionFilter(vttablet.matchLabels),

  // Create/update a Pod for a tablet spec within a VitessShard parent.
  pod(observed, spec):: {
    local podName = observed.parent.spec.cluster + "-vttablet-" + spec.alias,

    // A shell expression for a running tablet to find its own hostname.
    local hostnameExpr = "$(hostname -s)." + spec.subdomain,

    local defaultVttabletFlags = {
      service_map: "grpc-queryservice,grpc-tabletmanager,grpc-updatestream",
      "tablet-path": spec.alias,
      tablet_hostname: hostnameExpr,
      init_keyspace: spec.keyspace,
      init_shard: spec.shard,
      init_tablet_type: spec.type,
      health_check_interval: "5s",
      mysqlctl_socket: "$VTDATAROOT/mysqlctl.sock",
      "db-config-app-uname": "vt_app",
      "db-config-app-dbname": "vt_" + spec.keyspace,
      "db-config-app-charset": "utf8",
      "db-config-dba-uname": "vt_dba",
      "db-config-dba-dbname": "vt_" + spec.keyspace,
      "db-config-dba-charset": "utf8",
      "db-config-repl-uname": "vt_repl",
      "db-config-repl-dbname": "vt_" + spec.keyspace,
      "db-config-repl-charset": "utf8",
      "db-config-filtered-uname": "vt_filtered",
      "db-config-filtered-dbname": "vt_" + spec.keyspace,
      "db-config-filtered-charset": "utf8",
      enable_semi_sync: true,
      enable_replication_reporter: true,
      orc_api_url: "http://%s-global-orchestrator/api" % spec.cluster,
      orc_discover_interval: "5m",
      restore_from_backup: "backup_storage_implementation" in observed.parent.spec.backupFlags,
    },
    local vttabletFlags = vitess.serverFlags
      + vitess.topoFlags(observed.parent.spec.cluster)
      + defaultVttabletFlags
      + observed.parent.spec.backupFlags
      + (if "flags" in spec.vttablet then spec.vttablet.flags else {}),

    local mysqlctldFlags = vitess.baseFlags + {
      tablet_uid: spec.uid,
      socket_file: "$VTDATAROOT/mysqlctl.sock",
      "db-config-dba-uname": "vt_dba",
      "db-config-dba-charset": "utf8",
      init_db_sql_file: "$VTROOT/config/init_db.sql",
    } + (if "flags" in spec.mysql then spec.mysql.flags else {}),

    // TODO(enisoc): Allow customizing my.cnf somehow.
    local extraMyCnf = [
      "/vt/config/mycnf/master_mysql56.cnf",
      "/vt/vtdataroot/init/report-host.cnf",
    ],

    local env = [
      {name: "EXTRA_MY_CNF", value: std.join(":", extraMyCnf)},
    ],

    apiVersion: "v1",
    kind: "Pod",
    metadata: {
      name: podName,
      labels: observed.parent.spec.template.metadata.labels + {
        "vitess.io/component": "vttablet",
        "vitess.io/tablet-uid": spec.uidString,
      },
    },
    spec: {
      hostname: spec.alias,
      subdomain: spec.subdomain,
      securityContext: {runAsUser: 999, fsGroup: 999},
      initContainers: [
        {
          name: "init-vtdataroot",
          image: spec.image,
          command: ["bash", "-c",
            "set -ex; mkdir -p $VTDATAROOT/init;
            echo report-host=%s > $VTDATAROOT/init/report-host.cnf"
              % hostnameExpr],
          volumeMounts: [
            {name: "vtdataroot", mountPath: "/vt/vtdataroot"},
          ],
        },
      ],
      containers: [
        {
          name: "vttablet",
          image: spec.image,
          livenessProbe: {
            httpGet: {path: "/debug/vars", port: 15000},
            initialDelaySeconds: 60,
            timeoutSeconds: 10,
          },
          volumeMounts: [
            {name: "vtdataroot", mountPath: "/vt/vtdataroot"},
            // TODO(enisoc): Remove certs volume after switching to new vitess image.
            {name: "certs", readOnly: true, mountPath: "/etc/ssl/certs/ca-certificates.crt"},
          ],
          resources: spec.vttablet.resources,
          ports: [
            {name: "web", containerPort: 15000},
            {name: "grpc", containerPort: 15999},
          ],
          command: ["bash", "-c",
             "set -ex; exec /vt/bin/vttablet " +
             vitess.formatFlags(vttabletFlags)],
          env: env,
        },
        {
          name: "mysql",
          image: spec.image,
          volumeMounts: [
            {name: "vtdataroot", mountPath: "/vt/vtdataroot"},
          ],
          resources: spec.mysql.resources,
          command: ["bash", "-c",
             "set -ex; exec /vt/bin/mysqlctld " +
             vitess.formatFlags(mysqlctldFlags)],
          env: env,
        },
      ],
      volumes: [
        {name: "vtdataroot", persistentVolumeClaim: {claimName: podName}},
        // TODO(enisoc): Remove certs volume after switching to new vitess image.
        {name: "certs", hostPath: {path: "/etc/ssl/certs/ca-certificates.crt"}},
      ],
    },
  },

  // Create/update a PVC for a tablet spec.
  volume(observed, spec):: {
    apiVersion: "v1",
    kind: "PersistentVolumeClaim",
    metadata: {
      name: observed.parent.spec.cluster + "-vttablet-" + spec.alias,
      labels: observed.parent.spec.template.metadata.labels + {
        "vitess.io/component": "vttablet",
        "vitess.io/tablet-uid": spec.uidString,
      },
    },
    spec: spec.volumeClaim,
  },

  // Create/update a vttablet headless Service for a VitessCell spec.
  service(observed, spec):: {
    apiVersion: "v1",
    kind: "Service",
    metadata: {
      name: observed.parent.spec.cluster + "-vttablet-" + spec.name,
      labels: observed.parent.spec.template.metadata.labels + {
        "vitess.io/component": "vttablet",
      },
      annotations: {
        "service.alpha.kubernetes.io/tolerate-unready-endpoints": "true",
      },
    },
    spec: {
      selector: observed.parent.spec.selector.matchLabels + {
        "vitess.io/component": "vttablet",
      },
      ports: [
        {name: "web",  port: 15000},
        {name: "grpc", port: 15999},
      ],
      clusterIP: "None",
      publishNotReadyAddresses: true,
    },
  },

  getUid(tablet)::
    // Unlike the other CRDs, we can't put our own fields in the Pod spec,
    // so we put the UID in a label, which can also be used to select
    // individual tablets (if combined with cluster and cell labels).
    k8s.getLabel(tablet, "vitess.io/tablet-uid"),
}
