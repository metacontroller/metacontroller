# Apply Semantics

This page describes how Metacontroller applies changes to managed resources. Historically, Metacontroller has used a dynamic apply approach, which emulates [`kubectl apply`](https://kubernetes.io/docs/reference/kubectl/generated/kubectl_apply/), particularly for working with CRDs.

Now, Metacontroller also supports Kubernetes server-side apply (SSA), which is the recommended approach for declarative resource management in Kubernetes. SSA enables better field ownership tracking and is the future of Kubernetes resource application.

Below, we explain the motivation behind Metacontroller's apply mechanisms and describe both dynamic apply and server-side apply, including their use cases and trade-offs.

[[_TOC_]]

## Motivation

This section explains why Metacontroller uses `apply` semantics.

As an example, suppose you create a simple Pod like this
with `kubectl apply -f`:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  labels:
    app: my-app
spec:
  containers:
    - name: nginx
      image: nginx
```

If you then read back the Pod you created with `kubectl get pod my-pod -o yaml`,
you'll see a lot of extra fields filled in that you never set:

```yaml
apiVersion: v1
kind: Pod
metadata:
  annotations:
    kubernetes.io/limit-ranger: 'LimitRanger plugin set: cpu request for container
      nginx'
  creationTimestamp: 2018-04-13T00:46:51Z
  labels:
    app: my-app
  name: my-pod
  namespace: default
  resourceVersion: "28573496"
  selfLink: /api/v1/namespaces/default/pods/my-pod
  uid: 27f1b2e1-3eb4-11e8-88d2-42010a800051
spec:
  containers:
  - image: nginx
    imagePullPolicy: Always
    name: nginx
    resources:
      requests:
        cpu: 100m
    terminationMessagePath: /dev/termination-log
    terminationMessagePolicy: File
[...]
```

These fields may represent materialized default values and other metadata
set by the API server, values set by built-in admission control or
external admission plugins, or even values set by other controllers.

Rather than sifting through all that to find the fields you care about,
`kubectl apply` lets you go back to your original, simple file,
and make a change:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  labels:
    app: my-app
    role: staging # added a label
spec:
  containers:
    - name: nginx
      image: nginx
```

If you try to `kubectl create -f` your updated file, it will fail because
you can't create something that already exists.
If you try to `kubectl replace -f` your updated file, it will fail because
it thinks you're trying to unset all those extra fields.

However, if you use `kubectl apply -f` with your updated file,
it will update only the part you changed (adding a label),
and leave all those extra fields untouched.

Metacontroller treats the desired objects you return from your
hook in much the same way (but with [some differences](#dynamic-apply),
such as support for strategic merge inside CRDs).
As a result, you should always return the short form containing
only the fields you care about, not the long form containing
all the extra fields.

This generally means you should use the same code path to update things
as you do to create them.
Just generate a full JSON object from scratch every time,
containing all the fields you care about,
and only the fields you care about.

Metacontroller will figure out whether the object needs to be created
or updated, and which fields it should and shouldn't touch in the case
of an update.

## Dynamic Apply

The biggest difference between kubectl's implementation of apply
and Metacontroller's is that Metacontroller can emulate strategic
merge inside CRDs.

For example, suppose you have a CRD with an embedded Pod template:

```yaml
apiVersion: ctl.enisoc.com/v1
kind: CatSet # this resource is served via CRD
metadata:
  name: my-catset
spec:
  template: # embedded Pod template in CRD
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx
          ports:
            - containerPort: 80
              name: web
```

You create this with apply:

```sh
kubectl apply -f catset.yaml
```

The promise of `apply` is that it will "apply the changes you’ve made, without overwriting any automated changes to properties you haven’t specified".

As an example, suppose some other automation decides to edit your Pod template
and add a sidecar container:

```yaml
apiVersion: ctl.enisoc.com/v1
kind: CatSet # this resource is served via CRD
metadata:
  name: my-catset
spec:
  template: # embedded Pod template in CRD
    metadata:
      labels:
        app: nginx
    spec:
      containers:
        - name: nginx
          image: nginx
          ports:
            - containerPort: 80
              name: web
        - name: sidecar
          image: log-uploader # fake sidecar example
```

Now suppose you change something in your local file and reapply it:

```sh
kubectl apply -f catset.yaml
```

Because `kubectl apply` doesn't support strategic merge inside CRDs,
this will completely replace the `containers` list with yours,
removing the `sidecar` container.
By contrast, if this had been a Deployment or StatefulSet,
`kubectl apply` would have preserved the `sidecar` container.

As a result, if a controller uses kubectl's apply implementation
with CRDs, that controller will fight against automation that tries
to add sidecar containers or makes other changes to lists of objects
that Kubernetes expects to be treated like associative arrays
(ports, volumes, etc.).

To avoid this fighting, and to make the experience of using CRDs beter match
that of native resources, Metacontroller uses an alternative implementation
of apply logic that's based on convention instead of configuration.

### Conventions

The main convention that Metacontroller enforces on apply semantics
is how to detect and handle "associative lists".

In Kubernetes API conventions, an associative list is a list of objects
or scalars that should be treated as if it were a map (associative array),
but because of limitations in JSON/YAML it looks the same as an ordered list
when serialized.

For native resources, `kubectl apply` determines which lists are associative
lists by configuration: it must have compiled-in knowledge of all the resources,
and metadata about how each of their fields should be treated.
There is currently no mechanism for CRDs to specify this metadata,
which is why `kubectl apply` falls back to assuming all lists are "atomic",
and should never be merged (only replaced entirely).

Even if there were a mechanism for CRDs to specify metadata for every field
(e.g. through extensions to OpenAPI),
it's not clear that it makes sense to _require_ every CRD author to do so
in order for their resources to behave correctly when used with `kubecl apply`.
One alternative that has been considered for such "schemaless CRDs" is to
establish a convention -- as long as your CRD follows the convention, you
don't need to provide configuration.

Metacontroller implements one such convention that empirically handles
many common cases encountered when embedding Pod templates in CRDs
(although it has [limitations](#limitations)),
developed by surveying the use of associative lists across the resources
built into Kubernetes:

- A list is detected as an associative list if and only if all of the
  following conditions are met:
  - All items in the list are JSON objects
    (not scalars, nor other lists).
  - All objects in the list have some field name in common,
    where that field name is one of the conventional merge keys
    (most commonly `name`).
- If a list is detected as an associative list, the conventional
  field name that all objects have in common (e.g. `name`) is used
  as the merge key.
  - If more than one conventional merge key might work,
    pick only one according to a fixed order.

This allows Metacontroller to "do the right thing" in the majority of cases,
without requiring advance knowledge about the resources it's working with --
knowledge that's not available anywhere in the case of CRDs.

### Limitations

A convention-based approach is necessarily more limiting than
the native apply implementation, which supports arbitrary per-field
configuration.
The trade-off is that conventions reduce boilerplate and lower the
barrier to entry for simple use cases.

This section lists some examples of configurations that the native
apply allows, but are currently not supported in Metacontroller's
convention-based apply.
If any of these are blockers for you,
please [file an issue](https://www.github.com/metacontroller/metacontroller/issues) describing your
use case.

- Atomic object lists
  - A list of objects that share one of the conventional keys,
    but should nevertheless be treated atomically (replaced rather than merged).
- Unconventional associative list keys
  - An associative list that doesn't use one of the conventional keys.
- Multi-field associative list keys
  - A key that's composed of two or more fields (e.g. both `port` and `protocol`).
- Scalar-valued associative lists
  - A list of scalars (not objects) that should be merged as if the
    scalar values were field names in an object.

## Server-Side Apply

Server-side apply (SSA) is a Kubernetes-native declarative update mechanism that allows clients (e.g., controllers) to send a full object definition to the API server, which then manages field ownership and performs merges. Since SSA is a new feature for Metacontroller, it's advisable to use it with caution - especially in production environments - until you fully understand its implications and field ownership model.

SSA provides several advantages over client-side apply:

- **Field Ownership Tracking**: The Kubernetes API server tracks which controller or user modified each field, preventing unintended overwrites.
- **Strategic Merging**: Unlike dynamic apply, SSA applies **strategic merges** even inside CRDs, similar to native Kubernetes resources.
- **Better Handling of Concurrent Updates**: SSA provides better conflict resolution when multiple controllers modify the same resource.

### How Metacontroller Uses Server-Side Apply

When enabled, Metacontroller will:

- Use the `apply` verb with `server-side=true` when updating managed objects.
- Allow multiple controllers to modify different fields of the same resource without conflicts.
- Automatically merge updates to associative lists like `containers`, `volumes`, and `ports` without deleting unexpected changes.

### Example of Server-Side Apply

Instead of performing a full replacement, SSA updates only the fields specified:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: my-pod
  labels:
    app: my-app
    role: staging # added a label
spec:
  containers:
    - name: nginx
      image: nginx
```

When applied using SSA, Kubernetes ensures that fields like `resourceVersion`, `creationTimestamp`, and dynamically added containers (e.g., sidecars) **remain untouched**, unlike dynamic apply which would overwrite lists.

### Comparison: Dynamic Apply vs. Server-Side Apply

| Feature               | Dynamic Apply                       | Server-Side Apply (SSA)           |
| --------------------- | ----------------------------------- | --------------------------------- |
| Merging of CRD fields | Uses conventions (e.g., `name` key) | Full strategic merge support      |
| Field ownership       | Not explicitly tracked              | Kubernetes tracks field ownership |
| Concurrent updates    | Risk of overwriting fields          | Controlled conflict resolution    |
| Associative lists     | Convention-based merging            | Kubernetes-native merging         |
| Performance           | Fast (no API tracking)              | Slightly higher API overhead      |

### Enabling Server-Side Apply

To enable SSA in Metacontroller, configure the controller with:

```sh
--apply-strategy=server-side-apply
```

This setting ensures Metacontroller applies resources using Kubernetes-native [`server-side-apply`](https://kubernetes.io/docs/reference/using-api/server-side-apply) rather than dynamic apply.

## Future Direction

Previously, Metacontroller relied solely on a custom **dynamic apply** implementation to handle strategic merges within CRDs. However, with the introduction of Kubernetes **server-side apply (SSA)**, Metacontroller now supports SSA as a preferred alternative.

While **dynamic apply** remains available for compatibility, SSA is the recommended method for most use cases because it provides **native field ownership tracking**, **strategic merging**, and **better concurrency handling**.
