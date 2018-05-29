---
title: DecoratorController
classes: wide
---
DecoratorController is an API provided by Metacontroller, designed to facilitate
adding new behavior to existing resources. You can define rules for which
resources to watch, as well as filters on labels and annotations.

This page is a detailed reference of all the features available in this API.
See the [Create a Controller](/guide/create/) guide for a step-by-step walkthrough.

## Example

This [example DecoratorController](/examples/#decoratorcontroller)
attaches a Service for each Pod belonging to a StatefulSet,
for any StatefulSet that requests this behavior through a set of
annotations.

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: DecoratorController
metadata:
  name: service-per-pod
spec:
  resources:
  - apiVersion: apps/v1beta1
    resource: statefulsets
    annotationSelector:
      matchExpressions:
      - {key: service-per-pod-label, operator: Exists}
      - {key: service-per-pod-ports, operator: Exists}
  attachments:
  - apiVersion: v1
    resource: services
  hooks:
    sync:
      webhook:
        url: http://service-per-pod.metacontroller/sync-service-per-pod
```

## Spec

[spec]: #spec

A DecoratorController `spec` has the following fields:

| Field | Description |
| ----- | ----------- |
| [`resources`](#resources) | A list of resource rules specifying which objects to target for decoration (adding behavior). |
| [`attachments`](#attachments) | A list of resource rules specifying what this decorator can attach to the target resources. |
| [`resyncPeriodSeconds`](#resync-period) | How often, in seconds, you want every target object to be resynced (sent to your hook), even if no changes are detected. |
| [`hooks`](#hooks) | A set of lambda hooks for defining your controller's behavior. |

## Resources

Each DecoratorController can target one or more types of resources.
For every object that matches one of these rules, Metacontroller will
call your [sync hook](#sync-hook) to ask for your desired state.

Each entry in the `resources` list has the following fields:

| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `<group>/<version>` of the target resource, or just `<version>` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the target resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| [`labelSelector`](#label-selector) | An optional label selector for narrowing down the objects to target. |
| [`annotationSelector`](#annotation-selector) | An optional annotation selector for narrowing down the objects to target. |

### Label Selector

The `labelSelector` field within a [resource rule](#resources) has the following subfields:

| Field | Description |
| ----- | ----------- |
| `matchLabels` | A map of key-value pairs representing labels that must exist and have the specified values in order for an object to satisfy the selector. |
| `matchExpressions` | A list of [set-based requirements] on labels in order for an object to satisfy the selector. |

This label selector has the same format and semantics as the selector in
built-in APIs like Deployment.

If a `labelSelector` is specified for a given resource type,
the DecoratorController will ignore any objects of that type
that don't satisfy the selector.

If a resource rule has both a `labelSelector` and an `annotationSelector`,
the DecoratorController will only target objects of that type that satisfy
*both* selectors.

[set-based requirements]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/#resources-that-support-set-based-requirements

### Annotation Selector

The `annotationSelector` field within a [resource rule](#resources) has the following subfields:

| Field | Description |
| ----- | ----------- |
| `matchAnnotations` | A map of key-value pairs representing annotations that must exist and have the specified values in order for an object to satisfy the selector. |
| `matchExpressions` | A list of [set-based requirements] on annotations in order for an object to satisfy the selector. |

The annotation selector has an analogous format and semantics to the
[label selector](#label-selector) (note the field name `matchAnnotations`
rather than `matchLabels`).

If an `annotationSelector` is specified for a given resource type,
the DecoratorController will ignore any objects of that type
that don't satisfy the selector.

If a resource rule has both a `labelSelector` and an `annotationSelector`,
the DecoratorController will only target objects of that type that satisfy
*both* selectors.

## Attachments

This list should contain a rule for every type of resource
your controller wants to attach to an object of one of the
[targeted resources](#resources).

Unlike [child resources in CompositeController](/api/compositecontroller/#child-resources),
attachments are *not* related to the target object through
labels and label selectors.
This allows you to attach arbitrary things (which may not have any labels)
to other arbitrary things (which may not even have a selector).

Instead, attachments are only connected to the target object
through owner references, meaning they will get cleaned up
if the target object is deleted.

Each entry in the `attachments` list has the following fields:

| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `group/version` of the attached resource, or just `version` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the attached resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| [`updateStrategy`](#attachment-update-strategy) | An optional field that specifies how to update attachments when they already exist but don't match your desired state. **If no update strategy is specified, attachments of that type will never be updated if they already exist.** |

### Attachment Update Strategy

Within each rule in the `attachments` list, the `updateStrategy` field
has the following subfields:

| Field | Description |
| ----- | ----------- |
| [`method`](#attachment-update-methods) | A string indicating the overall method that should be used for updating this type of attachment resource. **The default is `OnDelete`, which means don't try to update attachments that already exist.** |

### Attachment Update Methods

Within each attachment resource's `updateStrategy`, the `method` field can have
these values:

| Method | Description |
| ------ | ----------- |
| `OnDelete` | Don't update existing attachments unless they get deleted by some other agent. |
| `Recreate` | Immediately delete any attachments that differ from the desired state, and recreate them in the desired state. |
| `InPlace` | Immediately update any attachments that differ from the desired state. |

Note that DecoratorController doesn't directly support rolling update
of attachments because you can compose such behavior by attaching
a [CompositeController](/api/compositecontroller/)
(or any other API that supports declarative rolling update,
like Deployment or StatefulSet).

## Resync Period

The `resyncPeriodSeconds` field in DecoratorController's `spec`
works similarly to the same field in
[CompositeController](/api/compositecontroller/#resync-period).

## Hooks

Within the DecoratorController `spec`, the `hooks` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| [`sync`](#sync-hook) | Specifies how to call your sync hook. |

Each field of `hooks` contains [subfields][hook] that specify how to invoke
that hook, such as by sending a request to a [webhook][].

[hook]: /api/hook/
[webhook]: /api/hook/#webhook

### Sync Hook

The `sync` hook is how you specify which attachments to create/maintain
for a given target object -- in other words, your desired state.

Based on the DecoratorController [spec][], Metacontroller gathers up
all the resources you said you need to decide on the desired state,
and sends you their latest observed states.

After you return your desired state, Metacontroller begins to take action
to converge towards it -- creating, deleting, and updating objects as appropriate.

A simple way to think about your sync hook implementation is like a script
that generates JSON to be sent to [`kubectl apply`][kubectl apply].
However, unlike a one-off client-side generator, your script has access to
the latest observed state in the cluster, and will automatically get called
any time that observed state changes.

[kubectl apply]: https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/#kubectl-apply

#### Sync Hook Request

A separate request will be sent for each target object,
so your hook only needs to think about one target object at a time.

The body of the request (a POST in the case of a [webhook][])
will be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `controller` | The whole DecoratorController object, like what you might get from `kubectl get decoratorcontroller <name> -o json`. |
| `object` | The target object, like what you might get from `kubectl get <target-resource> <target-name> -o json`. |
| `attachments` | An associative array of attachments that already exist. |

Each field of the `attachments` object represents one of the types of
[attachment resources](#attachments) in your DecoratorController [spec][].
The field name for each attachment type is `<Kind>.<apiVersion>`,
where `<apiVersion>` could be just `<version>` (for a core resource)
or `<group>/<version>`, just like you'd write in a YAML file.

For example, the field name for Pods would be `Pod.v1`,
while the field name for StatefulSets might be `StatefulSet.apps/v1`.

For resources that exist in multiple versions, the `apiVersion` you specify
in the [attachment resource rule](#attachments) is the one you'll be sent.
Metacontroller requires you to be explicit about the version you expect
because it does conversion for you as needed, so your hook doesn't need
to know how to convert between different versions of a given resource.

Within each attachment type (e.g. in `attachments['Pod.v1']`),
there is another associative array that maps from the attachment's
`metadata.name` to the JSON representation, like what you might get
from `kubectl get <attachment-resource> <attachment-name> -o json`.

For example, a Pod named `my-pod` could be accessed as:

```js
request.attachments['Pod.v1']['my-pod']
```

Note that you will only be sent objects that are owned by the target
(i.e. objects you attached), not all objects of that resource type.

There will always be an entry in `attachments` for every [attachment resource rule](#attachments),
even if no attachments of that type were observed at the time of the sync.
For example, if you listed Pods as an attachment resource rule,
but no existing Pods have been attached, you will receive:

```json
{
  "attachments": {
    "Pod.v1": {}
  }
}
```

as opposed to:

```json
{
  "attachments": {}
}
```

#### Sync Hook Response

The body of your response should be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `labels` | A map of key-value pairs for labels to set on the target object. |
| `annotations` | A map of key-value pairs for annotations to set on the target object. |
| `attachments` | A list of JSON objects representing all the desired attachments for this target object. |

Unlike the parent of a [CompositeController](/api/compositecontroller/),
DecoratorController assumes that each target object already has its own
controller, so you can't mutate the target's status.

In addition, decorators are conceptually adding behavior to the target's
controller. By convention, the controller for a given resource should not
modify its own spec, so your decorator also can't mutate the target's spec.

As a result, decorators currently cannot modify the target object except
to optionally set labels and annotations on it.

The `attachments` field should contain a flat list of objects,
not an associative array.
Metacontroller groups the objects it sends you by type and name as a
convenience to simplify your scripts, but it's actually redundant
since each object contains its own `apiVersion`, `kind`, and `metadata.name`.

It's important to include the `apiVersion` and `kind` in objects
you return, and also to ensure that you list every type of
[attachment resource](#attachments) you plan to create in the
DecoratorController spec.

Any objects sent as attachments in the request that you decline to return
in your response list **will be deleted**.
However, you shouldn't directly copy attachments from the request into the
response because they're in different forms.

Instead, you should think of each entry in the list of `attachments` as being
sent to [`kubectl apply`][kubectl apply].
That is, you should [set only the fields that you care about](/api/apply/).
