---
title: CompositeController
classes: wide
---
CompositeController is an API provided by Metacontroller, designed to facilitate
custom controllers whose primary purpose is to manage a set of child objects
based on the desired state specified in a parent object.

Workload controllers like [Deployment][] and [StatefulSet][] are examples of
existing controllers that fit this pattern.

This page is a detailed reference of all the features available in this API.
See the [Create a Controller](/guide/create/) guide for a step-by-step walkthrough.

[Deployment]: https://kubernetes.io/docs/concepts/workloads/controllers/deployment/
[StatefulSet]: https://kubernetes.io/docs/concepts/workloads/controllers/statefulset/

## Example

This [example CompositeController](/examples/#compositecontroller)
defines a controller that behaves like StatefulSet.

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: CompositeController
metadata:
  name: catset-controller
spec:
  parentResource:
    apiVersion: ctl.enisoc.com/v1
    resource: catsets
    revisionHistory:
      fieldPaths:
      - spec.template
  childResources:
  - apiVersion: v1
    resource: pods
    updateStrategy:
      method: RollingRecreate
      statusChecks:
        conditions:
        - type: Ready
          status: "True"
  - apiVersion: v1
    resource: persistentvolumeclaims
  hooks:
    sync:
      webhook:
        url: http://catset-controller.metacontroller/sync
        timeout: 10s
```

## Spec

[spec]: #spec

A CompositeController `spec` has the following fields:

| Field | Description |
| ----- | ----------- |
| [`parentResource`](#parent-resource) | A single resource rule specifying the parent resource. |
| [`childResources`](#child-resources) | A list of resource rules specifying the child resources. |
| [`resyncPeriodSeconds`](#resync-period) | How often, in seconds, you want every parent object to be resynced (sent to your hook), even if no changes are detected. |
| [`generateSelector`](#generate-selector) | If `true`, ignore the selector in each parent object and instead generate a unique selector that prevents overlap with other objects. |
| [`hooks`](#hooks) | A set of lambda hooks for defining your controller's behavior. |

## Parent Resource

[parent resource]: #parent-resource

The parent resource is the "entry point" for the CompositeController.
It should contain the information your controller needs to create
children, such as a Pod template if your controller creates Pods.
This is often a custom resource that you define (e.g. with CRD),
and for which you are now implementing a custom controller.

CompositeController expects to have full control over this resource.
That is, you shouldn't define a CompositeController with a parent
resource that already has its own controller.
See [DecoratorController](/api/decoratorcontroller/) for an API that's
better suited for adding behavior to existing resources.

The `parentResource` rule has the following fields:

| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `<group>/<version>` of the parent resource, or just `<version>` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the parent resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| [`revisionHistory`](#revision-history) | If any [child resources][] use rolling updates, this field specifies how parent revisions are tracked. |

### Label Selector

Kubernetes APIs use [labels and selectors][labels] to define subsets of
objects, such as the Pods managed by a given ReplicaSet.

The parent resource of a CompositeController is assumed to have a
`spec.selector` that matches the form of `spec.selector` in built-in resources
like Deployment and StatefulSet (with `matchLabels` and/or `matchExpressions`).

If the parent object doesn't have this field, or it can't be parsed in the
expected label selector format, the [sync hook](#sync-hook) for
that parent will fail, unless you are using [selector generation](#generate-selector).

The parent's label selector determines which child objects a given parent
will try to manage, according to the [ControllerRef rules][controller-ref].
Metacontroller automatically handles orphaning and adoption for you,
and will only send you the observed states of children you own.

These rules imply:

* **Children you create must have labels that satisfy the parent's selector**,
  or else they will be immediately orphaned and you'll never see them again.
* If other controllers or users create orphaned objects that match the parent's
  selector, Metacontroller will try to adopt them for you.
* If Metacontroller adopts an object, and you subsequently decline to list that
  object in your [desired list of children](#sync-hook-response),
  it will get deleted (because you now own it, but said you don't want it).

To avoid confusion, it's therefore important that users of your custom
controller specify a `spec.selector` (on each parent object) that is
sufficiently precise to discriminate its child objects from those of other
parents in the same namespace.

[labels]: https://kubernetes.io/docs/concepts/overview/working-with-objects/labels/
[controller-ref]: https://github.com/kubernetes/community/blob/master/contributors/design-proposals/api-machinery/controller-ref.md#behavior

### Revision History

Within the `parentResource` rule, the `revisionHistory` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| `fieldPaths` | A list of field path strings (e.g. `spec.template`) specifying which parent fields trigger rolling updates of children (for any [child resources][] that use rolling updates). Changes to other parent fields (e.g. `spec.replicas`) apply immediately. Defaults to `["spec"]`, meaning any change in the parent's `spec` triggers a rolling update. |

## Child Resources

[child resources]: #child-resources

This list should contain a rule for every type of child resource
that your controller creates on behalf of each parent.

Each entry in the `childResources` list has the following fields:

| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `group/version` of the child resource, or just `version` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the child resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| [`updateStrategy`](#child-update-strategy) | An optional field that specifies how to update children when they already exist but don't match your desired state. **If no update strategy is specified, children of that type will never be updated if they already exist.** |

### Child Update Strategy

Within each rule in the `childResources` list, the `updateStrategy` field
has the following subfields:

| Field | Description |
| ----- | ----------- |
| [`method`](#child-update-methods) | A string indicating the overall method that should be used for updating this type of child resource. **The default is `OnDelete`, which means don't try to update children that already exist.** |
| [`statusChecks`](#child-update-status-checks) | If any rolling update method is selected, children that have already been updated must pass these status checks before the rollout will continue. |

### Child Update Methods

Within each child resource's `updateStrategy`, the `method` field can have
these values:

| Method | Description |
| ------ | ----------- |
| `OnDelete` | Don't update existing children unless they get deleted by some other agent. |
| `Recreate` | Immediately delete any children that differ from the desired state, and recreate them in the desired state. |
| `InPlace` | Immediately update any children that differ from the desired state. |
| `RollingRecreate` | Delete each child that differs from the desired state, one at a time, and recreate each child before moving on to the next one. Pause the rollout if at any time one of the children that have already been updated fails one or more [status checks](#child-update-status-checks). |
| `RollingInPlace` | Update each child that differs from the desired state, one at a time. Pause the rollout if at any time one of the children that have already been updated fails one or more [status checks](#child-update-status-checks). |

### Child Update Status Checks

Within each `updateStrategy`, the `statusChecks` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| [`conditions`](#status-condition-check) | A list of status condition checks that must all pass on already-updated children for the rollout to continue. |

### Status Condition Check

Within a set of `statusChecks`, each item in the `conditions` list has the following subfields:

| Field | Description |
| ----- | ----------- |
| `type` | A string specifying the status condition `type` to check. |
| `status` | A string specifying the required `status` of the given status condition. If none is specified, the condition's `status` is not checked. |
| `reason` | A string specifying the required `reason` of the given status condition. If none is specified, the condition's `reason` is not checked. |

## Resync Period

By default, your [sync hook](#sync-hook) will only be called when
something changes in one of the resources you're watching,
or when the [local cache is flushed](/guide/install/#configuration).

Sometimes you may want to sync periodically even if nothing has
changed in the Kubernetes API objects, either to simply observe the passage
of time, or because your hook takes external state into account.
For example, CronJob uses a periodic resync to check whether it's time
to start a new Job.

The `resyncPeriodSeconds` value specifies how often to do this.
Each time it triggers, Metacontroller will send sync hook requests for
all objects of the parent resource type, with the latest observed
values of all the necessary objects.

Note that these objects will be retrieved from Metacontroller's local
cache (kept up-to-date through watches), so adding a resync shouldn't
add more load on the API server, unless you actually change objects.
For example, it's relatively cheap to use this setting to poll until
it's time to trigger some change, as long as most sync calls result in
a no-op (no CRUD operations needed to achieve desired state).

## Generate Selector

Usually, each parent object managed by a CompositeController must have its own
user-specified [label selector](#label-selector), just like each
Deployment has its own label selector in `spec.selector`.
However, sometimes it makes more sense to let the user of your API pretend there
are no labels or label selectors.

For example, the built-in [Job][] API doesn't make you specify labels for your
Pods, and you can leave `spec.selector` unset.
Because each Job object represents a unique invocation at a point in time,
you wouldn't expect a newly-created Job to be satisfied by finding a
pre-existing Pod that just happens to have the right labels.
On the other hand, a ReplicaSet assumes all Pods that match its selector are
interchangeable, so it would be happy to have one less replica it has to create.

If you set `spec.generateSelector` to `true` in your CompositeController
definition, Metacontroller will do the following:

* When creating children for you, Metacontroller will automatically add a label
  that points to the parent object's unique ID (`metadata.uid`).
* Metacontroller will *not* expect each parent object to contain a
  `spec.selector`, and will ignore the value even if one is set.
* Metacontroller will manage children as if each parent object had an
  "imaginary" label selector that points to the unique ID label that
  Metacontroller added to all your children.

The end result is that you and the users of your API don't have to think about
labels or selectors, similar to the Job API.
The downside is that your API won't support all the same capabilities as
built-in APIs.
For example, with ReplicaSet or StatefulSet, you can delete the controller with
`kubectl delete --cascade=false` to keep the Pods around, and later create a new
controller with the same selector to adopt those existing Pods instead of making
new ones from scratch.

[Job]: https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/

## Hooks

Within the CompositeController `spec`, the `hooks` field has the following subfields:

| Field | Description |
| ----- | ----------- |
| [`sync`](#sync-hook) | Specifies how to call your sync hook, if any. |
| [`finalize`](#finalize-hook) | Specifies how to call your finalize hook, if any. |

Each field of `hooks` contains [subfields][hook] that specify how to invoke
that hook, such as by sending a request to a [webhook][].

[hook]: /api/hook/
[webhook]: /api/hook/#webhook

### Sync Hook

The `sync` hook is how you specify which children to create/maintain
for a given parent -- in other words, your desired state.

Based on the CompositeController [spec][], Metacontroller gathers up
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

A separate request will be sent for each parent object,
so your hook only needs to think about one parent at a time.

The body of the request (a POST in the case of a [webhook][])
will be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `controller` | The whole CompositeController object, like what you might get from `kubectl get compositecontroller <name> -o json`. |
| `parent` | The parent object, like what you might get from `kubectl get <parent-resource> <parent-name> -o json`. |
| `children` | An associative array of child objects that already exist. |
| `finalizing` | This is always `false` for the `sync` hook. See the [`finalize` hook](#finalize-hook) for details. |

Each field of the `children` object represents one of the types of [child resources][]
you specified in your CompositeController [spec][].
The field name for each child type is `<Kind>.<apiVersion>`,
where `<apiVersion>` could be just `<version>` (for a core resource)
or `<group>/<version>`, just like you'd write in a YAML file.

For example, the field name for Pods would be `Pod.v1`,
while the field name for StatefulSets might be `StatefulSet.apps/v1`.

For resources that exist in multiple versions, the `apiVersion` you specify
in the [child resource rule][child resources] is the one you'll be sent.
Metacontroller requires you to be explicit about the version you expect
because it does conversion for you as needed, so your hook doesn't need
to know how to convert between different versions of a given resource.

Within each child type (e.g. in `children['Pod.v1']`), there is another
associative array that maps from the child's path relative to the parent
to the JSON representation, like what you might get from
`kubectl get <child-resource> <child-name> -o json`.

If the parent and child are of the same scope - both cluster or both namespace -
then the key is only the child's `.metadata.name`. If the parent is
cluster scoped and the child is namespace scoped, then the key will be of the
form `{.metadata.namespace}/{.metadata.name}`. This is to disambiguate between
two children with the same name in different namespaces. A parent may never be
namespace scoped while a child is cluster scoped.

For example, a Pod named `my-pod` in the `my-namespace` namespace could be
accessed as follows if the parent is also in `my-namespace`:

```js
request.children['Pod.v1']['my-pod']
```

Alternatively, if the parent resource is cluster scoped, the Pod could be
accessed as:

```js
request.children['Pod.v1']['my-namespace/my-pod']
```

Note that you will only be sent children that you "own" according to the
[ControllerRef rules][controller-ref].
That means, for a given parent object, **you will only see children whose
labels match the [parent's label selector](#label-selector), *and* that
don't belong to any other parent**.

There will always be an entry in `children` for every [child resource rule][child resources],
even if no children of that type were observed at the time of the sync.
For example, if you listed Pods as a child resource rule, but no existing Pods
matched the parent's selector, you will receive:

```json
{
  "children": {
    "Pod.v1": {}
  }
}
```

as opposed to:

```json
{
  "children": {}
}
```

#### Sync Hook Response

The body of your response should be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `status` | A JSON object that will completely replace the `status` field within the parent object. |
| `children` | A list of JSON objects representing all the desired children for this parent object. |

What you put in `status` is up to you, but usually it's best to follow
conventions established by controllers like Deployment.
You should compute `status` based only on the children that existed
when your hook was called; **status represents a report on the last
observed state, not the new desired state**.

The `children` field should contain a flat list of objects,
not an associative array.
Metacontroller groups the objects it sends you by type and name as a
convenience to simplify your scripts, but it's actually redundant
since each object contains its own `apiVersion`, `kind`, and `metadata.name`.

It's important to include the `apiVersion` and `kind` in objects
you return, and also to ensure that you list every type of
[child resource][child resources] you plan to create in the
CompositeController spec.

If the parent resource is cluster scoped and the child resource is namespaced,
it's important to include the `.metadata.namespace` since the namespace cannot
be inferred from the parent's namespace.

Any objects sent as children in the request that you decline to return
in your response list **will be deleted**.
However, you shouldn't directly copy children from the request into the
response because they're in different forms.

Instead, you should think of each entry in the list of `children` as being
sent to [`kubectl apply`][kubectl apply].
That is, you should [set only the fields that you care about](/api/apply/).

Note that your webhook handler must return a response with a status code of `200`
to be considered successful. Metacontroller will wait for a response for up to the
amount defined in the [Webhook spec](/api/hook/#webhook).

### Finalize Hook

If the `finalize` hook is defined, Metacontroller will add a finalizer to the
parent object, which will prevent it from being deleted until your hook has had
a chance to run and the response indicates that you're done cleaning up.

This is useful for doing ordered teardown of children, or for cleaning up
resources you may have created in an external system.
If you don't define a `finalize` hook, then when a parent object is deleted,
the garbage collector will delete all your children immediately,
and no hooks will be called.

The semantics of the `finalize` hook are mostly equivalent to those of
the [`sync` hook](#sync-hook).
Metacontroller will attempt to reconcile the desired states you return in the
`children` field, and will set `status` on the parent.
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

To perform ordered teardown, you can generate children just like you would for
`sync`, but omit some children from the desired state depending on the observed
set of children that are left.
For example, if you observe `[A,B,C]`, generate only `[A,B]` as your desired
state; if you observe `[A,B]`, generate only `[A]`; if you observe `[A]`,
return an empty desired list `[]`.

Once the observed state passed in with the `finalize` request meets all your
criteria (e.g. no more children were observed), and you have checked all
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
consider returning an error from your `finalize` hook instead of success.
This will cause Metacontroller to requeue and retry the hook later, giving you
a chance to recheck the external state without holding up a slot in the work
queue.
If you return success, Metacontroller won't call your hook again until the next
[periodic resync](#resync-period).
