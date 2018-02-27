local k8s = import "k8s.libsonnet";

// Library for working with kube-metacontroller.
{
  local metacontroller = self,

  // Extend a metacontroller request object with extra fields and functions.
  observed(request):: request + {
    children+: {
      // Get a map of children of a given kind, by child name.
      getMap(apiVersion, kind)::
        self[kind + "." + apiVersion],

      // Get a list of children of a given kind.
      getList(apiVersion, kind)::
        local map = self.getMap(apiVersion, kind);
        [map[key] for key in std.objectFields(map)],

      // Get a child object of a given kind and name.
      get(apiVersion, kind, name)::
        local map = self.getMap(apiVersion, kind);
        if name in map then map[name] else null,
    },
  },

  // Helpers for managing spec, observed, and desired states
  // for a collection of objects of a given Kind.
  collection(observed, specs, apiVersion, kind, desired):: {
    specs: if specs != null then specs else [],

    observed: observed.children.getList(apiVersion, kind),
    desired: [
      {apiVersion: apiVersion, kind: kind} + desired(observed, spec)
        for spec in self.specs
    ],

    getObserved(name): observed.children.get(apiVersion, kind, name),
  },

  // Mix-in for collection that filters observed objects.
  // This may be needed if a given parent has multiple collections of children
  // of the same Kind.
  collectionFilter(filter):: {
    observed: std.filter(filter, super.observed),
  },

  // Convert an integer string in the given base to "int" (actually double).
  // Should be precise up to 2^53.
  // This function is defined as a native extension in jsonnetd.
  parseInt(intStr, base):: std.native("parseInt")(intStr, base),
}
