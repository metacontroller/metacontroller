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
        timeout: 10s
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
| [`sync`](#sync-hook) | Specifies how to call your sync hook, if any. |
| [`finalize`](#finalize-hook) | Specifies how to call your finalize hook, if any. |

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
| `finalizing` | This is always `false` for the `sync` hook. See the [`finalize` hook](#finalize-hook) for details. |

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

Within each attachment type (e.g. in `attachments['Pod.v1']`), there is another
associative array that maps from the attachment's path relative to the parent to
the JSON representation, like what you might get from
`kubectl get <attachment-resource> <attachment-name> -o json`.

If the parent and attachment are of the same scope - both cluster or both namespace -
then the key is only the object's `.metadata.name`. If the parent is
cluster scoped and the attachment is namespace scoped, then the key will be of the
form `{.metadata.namespace}/{.metadata.name}`. This is to disambiguate between
two attachments with the same name in different namespaces. A parent may never
be namespace scoped while an attachment is cluster scoped.

For example, a Pod named `my-pod` in the `my-namespace` namespace could be
accessed as follows if the parent is also in `my-namespace`:

```js
request.attachments['Pod.v1']['my-pod']
```

Alternatively, if the parent resource is cluster scoped, the Pod could be
accessed as:

```js
request.attachments['Pod.v1']['my-namespace/my-pod']
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
| `status` | A JSON object that will completely replace the `status` field within the target object. Leave unspecified or `null` to avoid changing `status`. |
| `attachments` | A list of JSON objects representing all the desired attachments for this target object. |
| `resyncAfterSeconds` | Set the delay (in seconds, as a float) before an optional, one-time, per-object resync. |

By convention, the controller for a given resource should not
modify its own spec, so your decorator can't mutate the target's spec.

As a result, decorators currently cannot modify the target object except
to optionally set labels, annotations, and status on it.
Note that if the target resource already has its own controller,
that controller might ignore and overwrite any status updates you make.

The `attachments` field should contain a flat list of objects,
not an associative array.
Metacontroller groups the objects it sends you by type and name as a
convenience to simplify your scripts, but it's actually redundant
since each object contains its own `apiVersion`, `kind`, and `metadata.name`.

It's important to include the `apiVersion` and `kind` in objects
you return, and also to ensure that you list every type of
[attachment resource](#attachments) you plan to create in the
DecoratorController spec.

If the parent resource is cluster scoped and the child resource is namespaced,
it's important to include the `.metadata.namespace` since the namespace cannot
be inferred from the parent's namespace.

Any objects sent as attachments in the request that you decline to return
in your response list **will be deleted**.
However, you shouldn't directly copy attachments from the request into the
response because they're in different forms.

Instead, you should think of each entry in the list of `attachments` as being
sent to [`kubectl apply`][kubectl apply].
That is, you should [set only the fields that you care about](/api/apply/).

You can optionally set `resyncAfterSeconds` to a value greater than 0 to request
that the `sync` hook be called again with this particular parent object after
some delay (specified in seconds, with decimal fractions allowed).
Unlike the controller-wide [`resyncPeriodSeconds`](#resync-period), this is a
one-time request (not a request to start periodic resyncs), although you can
always return another `resyncAfterSeconds` value from subsequent `sync` calls.
Also unlike the controller-wide setting, this request only applies to the
particular parent object that this `sync` call sent, so you can request
different delays (or omit the request) depending on the state of each object.

Note that your webhook handler must return a response with a status code of `200`
to be considered successful. Metacontroller will wait for a response for up to the
amount defined in the [Webhook spec](/api/hook/#webhook).

### Finalize Hook

If the `finalize` hook is defined, Metacontroller will add a finalizer to the
parent object, which will prevent it from being deleted until your hook has had
a chance to run and the response indicates that you're done cleaning up.

This is useful for doing ordered teardown of attachments, or for cleaning up
resources you may have created in an external system.
If you don't define a `finalize` hook, then when a parent object is deleted,
the garbage collector will delete all your attachments immediately,
and no hooks will be called.

In addition to finalizing when an object is deleted, Metacontroller will also
call your `finalize` hook on objects that were previously sent to `sync`
but now no longer match the DecoratorController's label and annotation selectors.
This allows you to clean up after yourself when the object has been updated to
opt out of the functionality added by your decorator, even if the object is not
being deleted.
If you don't define a `finalize` hook, then when the object opts out, any
attachments you added will remain until the object is deleted,
and no hooks will be called.

The semantics of the `finalize` hook are mostly equivalent to those of
the [`sync` hook](#sync-hook).
Metacontroller will attempt to reconcile the desired states you return in the
`attachments` field, and will set labels and annotations as requested.
The main difference is that `finalize` will be called instead of `sync` when
it's time to clean up because the parent object is pending deletion.

Note that, just like `sync`, your `finalize` handler must be idempotent.
Metacontroller might call your hook multiple times as the observed state
changes, possibly even after you first indicate that you're done finalizing.
Your handler should know how to check what still needs to be done
and report success if there's nothing left to do.

Both `sync` and `finalize` have a request field called `finalizing` that
indicates which hook was actually called.
This lets you implement `finalize` either as a separate handler or as a check
within your `sync` handler, depending on how much logic they share.
To use the same handler for both, just define a `finalize` hook and set it to
the same value as your `sync` hook.

#### Finalize Hook Request

The `finalize` hook request has all the same fields as the
[`sync` hook request](#sync-hook-request), with the following changes:

| Field | Description |
| ----- | ----------- |
| `finalizing` | This is always `true` for the `finalize` hook. See the [`finalize` hook](#finalize-hook) for details. |

If you share the same handler for both `sync` and `finalize`, you can use the
`finalizing` field to tell whether it's time to clean up or whether it's a
normal sync.
If you define a separate handler just for `finalize`, there's no need to check
the `finalizing` field since it will always be `true`.

#### Finalize Hook Response

The `finalize` hook response has all the same fields as the
[`sync` hook response](#sync-hook-response), with the following additions:

| Field | Description |
| ----- | ----------- |
| `finalized` | A boolean indicating whether you are done finalizing. |

To perform ordered teardown, you can generate attachments just like you would for
`sync`, but omit some attachments from the desired state depending on the observed
set of attachments that are left.
For example, if you observe `[A,B,C]`, generate only `[A,B]` as your desired
state; if you observe `[A,B]`, generate only `[A]`; if you observe `[A]`,
return an empty desired list `[]`.

Once the observed state passed in with the `finalize` request meets all your
criteria (e.g. no more attachments were observed), and you have checked all
other criteria (e.g. no corresponding external resource exists), return `true`
for the `finalized` field in your response.

Note that you should *not* return `finalized: true` the first time you return
a desired state that you consider "final", since there's no guarantee that your
desired state will be reached immediately.
Instead, you should wait until the *observed* state matches what you want.

If the observed state passed in with the request doesn't meet your criteria,
you can return a successful response (HTTP code 200) with `finalized: false`,
and Metacontroller will call your hook again automatically if anything changes
in the observed state.

If the only thing you're still waiting for is a state change in an external
system, and you don't need to assert any new desired state for your children,
returning success from the `finalize` hook may mean that Metacontroller doesn't
call your hook again until the next [periodic resync](#resync-period).
To reduce the delay, you can request a one-time, per-object resync by setting
`resyncAfterSeconds` in your [hook response](#sync-hook-response), giving you
a chance to recheck the external state without holding up a slot in the work
queue.
