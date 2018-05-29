---
title: ControllerRevision
classes: wide
---
ControllerRevision is an internal API used by Metacontroller to implement
declarative rolling updates.

Users of Metacontroller normally shouldn't need to know about this API,
but it is documented here for Metacontroller [contributors](/contrib/),
as well as for [troubleshooting](/guide/troubleshooting/).

Note that this is different from the ControllerRevision in `apps/v1`,
although it serves a similar purpose.
You will likely need to use a fully-qualified resource name to inspect
Metacontroller's ControllerRevisions:

```sh
kubectl get controllerrevisions.metacontroller.k8s.io
```

Each ControllerRevision's name is a combination of the name and API group
(excluding the version suffix) of the resource that it's a revision of,
as well as a hash that is deterministic yet unique (used only for idempotent
creation, not for lookup).

By default, ControllerRevisions belonging to a particular parent instance
will get garbage-collected if the parent is deleted.
However, it is possible to orphan ControllerRevisions during parent
deletion, and then create a replacement parent to adopt them.
ControllerRevisions are adopted based on the parent's label selector,
the same way controllers like ReplicaSet adopt Pods.

## Example

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: ControllerRevision
metadata:
  name: catsets.ctl.enisoc.com-5463ba99b804a121d35d14a5ab74546d1e8ba953
  labels:
    app: nginx
    component: backend
    metacontroller.k8s.io/apiGroup: ctl.enisoc.com
    metacontroller.k8s.io/resource: catsets
parentPatch:
  spec:
    template:
      [...]
children:
- apiGroup: ""
  kind: Pod
  names:
  - nginx-backend-0
  - nginx-backend-1
  - nginx-backend-2
```

## Parent Patch

The `parentPatch` field stores a partial representation of the parent object
at a given revision, containing only those fields listed by the lambda controller
author as participating in rolling updates.

For example, if a CompositeController's [revision history][] specifies
a `fieldPaths` list of `["spec.template"]`, the parent patch will contain
only `spec.template` and any subfields nested within it.

This mirrors the selective behavior of rolling updates in built-in APIs
like Deployment and StatefulSet.
Any fields that aren't part of the parent patch take effect immediately,
rather than rolling out gradually.

[revision history]: /api/compositecontroller/#revision-history

## Children

The `children` field stores a list of child objects that "belong" to this
particular revision of the parent.

This is how Metacontroller keeps track of the current desired revision of
a given child.
For example, if a Pod that hasn't been updated yet gets deleted by a Node
drain, it should be replaced at the revision it was on before it got deleted,
not at the latest revision.

When Metacontroller decides it's time to update a given child to another
revision, it first records this intention by updating the relevant
ControllerRevision objects.
After committing these records, it then begins updating that child according
to the configured [child update strategy](/api/compositecontroller/#child-update-strategy).
This ensures that the intermediate progress of the rollout is persisted
in the API server so it survives process restarts.

Children are grouped by API Group (excluding the version suffix) and Kind.
For each Group-Kind, we store a list of object names.
Note that parent and children must be in the same namespace,
and ControllerRevisions for a given parent also live in that
parent's namespace.