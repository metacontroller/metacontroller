// Library for working with Kubernetes objects.
{
  local k8s = self,

  // Fill in a conventional status condition object.
  condition(type, status):: {
    type: type,
    status:
      if std.type(status) == "string" then (
        status
      ) else if std.type(status) == "boolean" then (
        if status then "True" else "False"
      ) else (
        "Unknown"
      ),
  },

  // Extract the status of a given condition type.
  // Returns null if the condition doesn't exist.
  conditionStatus(obj, type)::
    if obj != null && "status" in obj && "conditions" in obj.status then
      // Filter conditions with matching "type" field.
      local matches = [
        cond.status for cond in obj.status.conditions if cond.type == type
      ];
      // Take the first one, if any.
      if std.length(matches) > 0 then matches[0] else ""
    else
      null,

  // Returns only the objects from a given list that have the
  // "Ready" condition set to "True".
  filterReady(list)::
    std.filter(function(x) self.conditionStatus(x, "Ready") == "True", list),

  // Returns only the objects from a given list that have the
  // "Available" condition set to "True".
  filterAvailable(list)::
    std.filter(function(x) self.conditionStatus(x, "Available") == "True", list),

  // Returns whether the object matches the given label values.
  matchLabels(obj, labels)::
    local keys = std.objectFields(labels);

    "metadata" in obj && "labels" in obj.metadata &&
      [
        obj.metadata.labels[k]
          for k in keys if k in obj.metadata.labels
      ]
      ==
      [labels[k] for k in keys],

  // Get the value of a label from object metadata.
  // Returns null if the label doesn't exist.
  getLabel(obj, key)::
    if "metadata" in obj && "labels" in obj.metadata && key in obj.metadata.labels then
      obj.metadata.labels[key]
    else
      null,

  // Get the value of an annotation from object metadata.
  // Returns null if the annotation doesn't exist.
  getAnnotation(obj, key)::
    if "metadata" in obj && "annotations" in obj.metadata && key in obj.metadata.annotations then
      obj.metadata.annotations[key]
    else
      null,
}
