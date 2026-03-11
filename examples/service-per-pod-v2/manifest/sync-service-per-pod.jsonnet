function(request) {
  local statefulset = request.object,
  local labelKey = statefulset.metadata.annotations["service-per-pod-label"],
  local ports = statefulset.metadata.annotations["service-per-pod-ports"],
  local ns = statefulset.metadata.namespace,

  // Check if we can see our existing attachments via UniformObjectMap (v2) naming.
  // In v2, keys for namespaced resources are always "namespace/name".
  local firstSvcKey = ns + "/" + statefulset.metadata.name + "-0",
  local services = if std.objectHas(request.attachments, 'Service.v1') then request.attachments['Service.v1'] else {},
  
  // We add an annotation to the parent to signal that we verified v2 naming.
  annotations: {
    // If the service exists, we should be able to find it by namespace/name key
    "v2-naming-verified": if std.objectHas(services, firstSvcKey) then "true" else "false"
  },

  // Create a service for each Pod, with a selector on the given label key.
  attachments: [
    {
      apiVersion: "v1",
      kind: "Service",
      metadata: {
        name: statefulset.metadata.name + "-" + index,
        labels: {app: "service-per-pod"}
      },
      spec: {
        selector: {
          [labelKey]: statefulset.metadata.name + "-" + index
        },
        ports: [
          {
            local parts = std.split(portnums, ":"),
            name: "port-" + std.parseInt(parts[0]),
            port: std.parseInt(parts[0]),
            targetPort: std.parseInt(parts[1]),
          }
          for portnums in std.split(ports, ",")
        ]
      }
    }
    for index in std.range(0, statefulset.spec.replicas - 1)
  ]
}
