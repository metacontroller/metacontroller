# Customize Hook

If the customize hook is defined, Metacontroller will ask for which related objects, or classes of objects that your sync and finalize hooks need to know about.
This is useful for mapping across many objects. One example would be a controller that lets you specify ConfigMaps to be placed in every Namespace.
Another use-case is being able to reference other objects, e.g. the env section from a core Pod object.
If you don't define a customize hook, then the related section of the hooks will be empty.

The `customize` hook will not provide any information about the current state of
the cluster. Thus, the set of related objects may only depend on the state of
the parent object.

This hook may also accept other fields in future, for other customizations.

[[_TOC_]]

## Customize Hook Request

A separate request will be sent for each parent object,
so your hook only needs to think about one parent at a time.

The body of the request (a POST in the case of a [webhook](../api/hook.md#webhook))
will be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `controller` | The whole controller object (CompositeController or DecoratorController). |
| `parent` | The parent object (or target object for DecoratorController). |

Metacontroller supports both `v1` and `v2` hook versions. 
The version can be specified in the controller's `hooks.customize.version` field.
The primary difference is the naming convention used for objects in the `sync` 
and `finalize` hooks' `related` field (see 
[CompositeController](./compositecontroller.md#hook-version-v2-uniformobjectmap) 
and [DecoratorController](./decoratorcontroller.md#hook-version-v2-uniformobjectmap) 
API references for details on `v2` naming).


## Customize Hook Response

The body of your response should be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `relatedResources` | A list of JSON objects (`ResourceRules`) representing all the desired related resource descriptions (). |

The `relatedResources` field should contain a flat list of objects,
not an associative array.

Each `ResourceRule` object should be a JSON object with the following fields:

| Field | Description |
| ----- | ----------- |
| `apiVersion` | The API `<group>/<version>` of the parent resource, or just `<version>` for core APIs. (e.g. `v1`, `apps/v1`, `batch/v1`) |
| `resource`   | The canonical, lowercase, plural name of the parent resource. (e.g. `deployments`, `replicasets`, `statefulsets`) |
| `labelSelector` | A `v1.LabelSelector` object. Filters objects by their labels. |
| `namespaceSelector` | A `v1.LabelSelector` object. Filters namespaces by their labels. If omitted and `namespace` is also omitted, searching is performed across all namespaces. **Warning:** Using `namespaceSelector` without a `labelSelector` will select **ALL** objects of the specified type in the matching namespaces, which can have a significant performance impact in large clusters. Additionally, selecting a large number of namespaces can be expensive as it requires separate list operations for each matching namespace. |
| `namespace` | Optional. The specific Namespace to select in. |
| `names` | Optional. A list of strings, representing individual objects to return. |


**Combined usage rules**

*   `names` (explicit list) **cannot** be combined with any selector (`labelSelector` or `namespaceSelector`).
*   `namespace` (explicit name) **cannot** be combined with `namespaceSelector`.
*   `namespace` **can** be combined with `labelSelector` to find specific objects within one namespace.
*   `namespaceSelector` **can** be combined with `labelSelector` to find specific objects across multiple namespaces. **It is highly recommended to provide a `labelSelector` when using `namespaceSelector` to avoid accidentally selecting all objects in the target namespaces.**
*   If both `namespace` and `namespaceSelector` are omitted, `labelSelector` will search across **all** namespaces.

If the parent resource is cluster scoped and the related resource is namespaced,
the namespace may be used to restrict which objects to look at.

If the parent resource is namespaced, the related resources must come from the
same namespace **unless hook version `v2` is used**.

In `v2`, a namespaced parent can access:
- **Cluster-scoped** related objects.
- **Namespaced** related objects from **any namespace**.

Specifying the namespace is optional. In `v1`, if specified, it must match the 
parent's namespace. In `v2`, it can be any namespace.

Note that your webhook handler must return a response with a status code of `200`
to be considered successful. Metacontroller will wait for a response for up to the
amount defined in the [Webhook spec](../api/hook.md#webhook).

## Example

Let's take a look at [Global Config Map example](../examples.md#global-config-map) custom resource object:
```yaml
---
apiVersion: examples.metacontroller.io/v1alpha1
kind: GlobalConfigMap
metadata:
  name: globalsettings
spec:
  sourceName: globalsettings
  sourceNamespace: global
```
it tells that we would like to have `globalsettings` ConfigMap from `global` namespace
present in each namespace.

The customize hook request will looks like :
```json
{
    'controller': '...',
    'parent': '...'
}
```

and we need to extract information identyfying source ConfigMap.

Controller returns :
```json
[
    {
        'apiVersion': 'v1',
        'resource': 'configmaps',
        'namespace': ${parent['spec']['sourceNamespace']},
        'names': [${parent['spec']['sourceName']}]
    }, {
        'apiVersion': 'v1',
        'resource': 'namespaces',
        'labelSelector': {}
    }
]
```

The first `RelatedRule` describes that given configmap should be returned (it will be used as souce for our propagation).

The second `RelatedRule` describes that we want to recieve also all namespaces in the cluster (`'labelSelector': {}` means - select all objects).

With those rules, call to the `sync` hook will have non empty `related` field (if resources exists in the cluster), in which all objects matching given criteria will be present.
