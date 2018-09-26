---
title: Best Practices
---
This is a collection of recommendations for writing controllers with Metacontroller.

If you have something to add to the collection, please send a pull request against
[this document]({{ site.repo_file }}/docs/_guide/best-practices.md).

## Lambda Hooks

### Apply Semantics

Because Metacontroller uses [apply semantics](/api/apply/), you don't have to
think about whether a given object needs to be created (because it doesn't exist)
or patched (because it exists and some fields don't match your desired state).
In either case, you should generate a fresh object from scratch with only the
fields you care about filled in.

For example, suppose you create an object like this:

```yaml
apiVersion: example.com/v1
kind: Foo
metadata:
  name: my-foo
spec:
  importantField: 1
```

Then later you decide to change the value of `importantField` to 2.

Since Kubernetes API objects can be edited by the API server, users, and other
controllers to collaboratively produce emergent behavior, the object you observe
might now look like this:

```yaml
apiVersion: example.com/v1
kind: Foo
metadata:
  name: my-foo
  stuffFilledByAPIServer: blah
spec:
  importantField: 1
  otherField: 5
```

To avoid overwriting the parts of the object you don't care about, you would
ordinarily need to either build a patch or use a retry loop to send
concurrency-safe updates.
With apply semantics, you instead just call your "generate object" function
again with the new values you want, and return this (as JSON):

```yaml
apiVersion: example.com/v1
kind: Foo
metadata:
  name: my-foo
spec:
  importantField: 2
```

Metacontroller will take care of merging your change to `importantField` while
preserving the fields you don't care about that were set by others.

### Side Effects

Your hook code should generally be free of side effects whenever possible.
Ideally, you should interpret a call to your hook as asking,
"Hypothetically, if the observed state of the world were like this, what would
your desired state be?"

In particular, Metacontroller may ask you about such hypothetical scenarios
during rolling updates, when your object is undergoing a slow transition between
two desired states.
If your hook has to produce side effects to work, you should avoid enabling
rolling updates on that controller.

### Status

If your object uses the Spec/Status convention, keep in mind that the Status
returned from your hook should ideally reflect a judgement on only the observed
objects that were sent to you.
The Status you compute should not yet account for your desired state, because
the actual state of the world may not match what you want yet.

For example, if you observe 2 Pods, but you return a desired list of 3 Pods,
you should return a Status that reflects only the observed Pods
(e.g. `replicas: 2`).
This is important so that Status reflects present reality, not future desires.
