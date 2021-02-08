function(request) {
  local statefulset = request.object,
  local labelKey = statefulset.metadata.annotations["service-per-pod-label"],
  local ports = statefulset.metadata.annotations["service-per-pod-ports"],

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
