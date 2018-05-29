---
title: Apply Semantics
classes: wide
---
This page describes how Metacontroller emulates [`kubectl apply`][kubectl apply].

In most cases, you should be able to think of Metacontroller's apply semantics
as being the same as `kubectl apply`, but there are some differences.

[kubectl apply]: https://kubernetes.io/docs/concepts/cluster-administration/manage-deployment/#kubectl-apply

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
it's not clear that it makes sense to *require* every CRD author to do so
in order for their resources to behave correctly when used with `kubecl apply`.
One alternative that has been considered for such "schemaless CRDs" is to
establish a convention -- as long as your CRD follows the convention, you
don't need to provide configuration.

Metacontroller implements one such convention that empirically handles
many common cases encountered when embedding Pod templates in CRDs
(although it has [limitations](#limitations)),
developed by surveying the use of associative lists across the resources
built into Kubernetes:

* A list is detected as an associative list if and only if all of the
  following conditions are met:
  * All items in the list are JSON objects
    (not scalars, nor other lists).
  * All objects in the list have some field name in common,
    where that field name is one of the conventional merge keys
    (most commonly `name`).
* If a list is detected as an associative list, the conventional
  field name that all objects have in common (e.g. `name`) is used
  as the merge key.
  * If more than one conventional merge key might work,
    pick only one according to a fixed order.

This allows Metacontroller to "do the right thing" in the majority of cases,
without requiring advance knowledge about the resources it's working with --
knowledge that's not available anywhere in the case of CRDs.

In the future, Metacontroller will likely switch from this custom apply
implementation to [server-side apply][], which is trying to solve the
broader problem for all components that interact with the Kubernetes API.
However, it's not yet clear whether that proposal will embrace schemaless
CRDs and support apply semantics on them.

[server-side apply]: https://github.com/kubernetes/features/issues/555

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
please [file an issue]({{ site.repo_url }}/issues) describing your
use case.

* Atomic object lists
  * A list of objects that share one of the conventional keys,
    but should nevertheless be treated atomically (replaced rather than merged).
* Unconventional associative list keys
  * An associative list that doesn't use one of the conventional keys.
* Multi-field associative list keys
  * A key that's composed of two or more fields (e.g. both `port` and `protocol`).
* Scalar-valued associative lists
  * A list of scalars (not objects) that should be merged as if the
    scalar values were field names in an object.