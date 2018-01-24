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
    updateStrategy:
      if "updateStrategy" in observed.controller.spec then
        observed.controller.spec.updateStrategy
      else
        null,
  },

  // Mix-in for collection that filters observed objects.
  // This may be needed if a given parent has multiple collections of children
  // of the same Kind.
  collectionFilter(filter):: {
    observed: std.filter(filter, super.observed),
  },

  // Mix-in for collection that causes the child objects to be treated as
  // "immutable". That is, they will be created if they don't exist, but left
  // untouched if they do.
  //
  // Note that in the case of the "Apply" strategy, this means we return the
  // last applied config. Metacontroller might still attempt an update in
  // response, if a third party has mutated fields to which we've previously
  // applied values. In other words, it continues to actively maintain the last
  // applied config; it doesn't become totally passive.
  collectionImmutable:: {
    desired: [
      local observed = super.getObserved(desired.metadata.name);
      if observed == null then
        desired
      else (
        if super.updateStrategy == "Apply" then
          metacontroller.getLastApplied(observed)
        else
          observed
      )
      for desired in super.desired
    ],
  },

  // Unmarshal the Metacontroller last applied config annotation from an object.
  // If the annotation doesn't exist, it returns an "empty" last applied config.
  // Returning an empty config when using the "Apply" strategy will tell
  // Metacontroller, "I want this to exist, but I don't care what's in it."
  getLastApplied(obj)::
    local lastApplied = k8s.getAnnotation(obj, "metacontroller.k8s.io/last-applied-configuration");
    if lastApplied != null then
      metacontroller.jsonUnmarshal(lastApplied)
    else {
      apiVersion: obj.apiVersion,
      kind: obj.kind,
      metadata: {name: obj.metadata.name},
    },

  // Unmarshal JSON into a Jsonnet value.
  // This function is defined as a native extension in jsonnetd.
  jsonUnmarshal(jsonString):: std.native("jsonUnmarshal")(jsonString),

  // Convert an integer string in the given base to "int" (actually double).
  // Should be precise up to 2^53.
  // This function is defined as a native extension in jsonnetd.
  parseInt(intStr, base):: std.native("parseInt")(intStr, base),
}
