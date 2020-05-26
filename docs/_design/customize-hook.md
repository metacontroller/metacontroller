---
title: Customize Hook
---
### Controller changes

The composite and decorator controller `sync` and `finalize` hooks will contain a new field, `related`.

This field is in the same format as `children` / `ChildMap`.

### Customize Hook

If the `customize` hook is defined, Metacontroller will ask for which related
objects, or classes of objects that your `sync` and `finalize` hooks need to
know about.

This is useful for mapping across many objects. One example would be a
controller that lets you specify ConfigMaps to be placed in every Namespace.

Another use-case is being able to reference other objects, e.g. the `env`
section from a core `Pod` object.

If you don't define a `customize` hook, then the related section of the hooks will
be empty.

The `customize` hook will not provide any information about the current state of
the cluster. Thus, the set of related objects may only depend on the state of
the parent object.

This hook may also accept other fields in future, for other customizations.

#### Customize Hook Request

A separate request will be sent for each parent object,
so your hook only needs to think about one parent at a time.

The body of the request (a POST in the case of a [webhook][])
will be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `controller` | The whole CompositeController object, like what you might get from `kubectl get compositecontroller <name> -o json`. |
| `parent` | The parent object, like what you might get from `kubectl get <parent-resource> <parent-name> -o json`. |

#### Customize Hook Response

The body of your response should be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `relatedResources` | A list of JSON objects representing all the desired related resource label selectors. |

The `relatedResources` field should contain a flat list of objects,
not an associative array.

Each resource rule object should be a JSON object with the following fields:
| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `<group>/<version>` of the parent resource, or just `<version>` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the parent resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| `labelSelector` | A `v1.LabelSelector` object. |
| `namespace` | Optional. The Namespace to select in |
| `names` | Optional. A list of strings, representing individual objects to return |

If the parent resource is cluster scoped and the related resource is namespaced,
the namespace may be used to restrict which objects to look at. If the parent
resource is namespaced, the related resources must come from the same namespace.
Specifying the namespace is optional, but if specified must match.

Note that your webhook handler must return a response with a status code of `200`
to be considered successful. Metacontroller will wait for a response for up to the
amount defined in the [Webhook spec](/api/hook/#webhook).
