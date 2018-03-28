---
title: Create a Controller
classes: wide
toc: false
---
This tutorial walks through a simple example of creating a controller in Python
with Metacontroller.

## Prerequisites

* Kubernetes v1.8+ is recommended for the improved CRD support, especially
  garbage collection on custom resources.
* You should have `kubectl` available and configured to talk to the desired cluster.
* You should have already [installed Metacontroller](/guide/install/).

## Hello, World!

In this example, we'll create a useless controller that runs a single Pod
that prints a greeting to its standard output.
Once you're familiar with the general process, you can look through the
[examples](/examples/) page to find concepts that actually do something useful.

To make cleanup easier, first create a new Namespace called `hello`:

```sh
kubectl create namespace hello
```

We'll put all our Namespace-scoped objects there by adding `-n hello` to the
`kubectl` commands.

### Define a custom resource

Our example controller will implement the behavior for a new API represented
as a [custom resource][].

First, let's use the built-in [CustomResourceDefinition][CRD] API to set up a storage location
(a *helloworlds* [resource][]) for objects of our custom type (HelloWorld).

Save the following to a file called `crd.yaml`:

```yaml
apiVersion: apiextensions.k8s.io/v1beta1
kind: CustomResourceDefinition
metadata:
  name: helloworlds.example.com
spec:
  group: example.com
  version: v1
  names:
    kind: HelloWorld
    plural: helloworlds
    singular: helloworld
```

Then apply it to your cluster:

```sh
kubectl apply -f crd.yaml
```

[custom resource]: /concepts/#custom-resource
[CRD]: https://kubernetes.io/docs/concepts/api-extension/custom-resources/#customresourcedefinitions
[resource]: /concepts/#resource

### Define a custom controller

For each HelloWorld object, we're going to create a Pod as a child object,
so we'll use the [CompositeController][] API to implement a controller that
defines this parent-child relationship.

Save the following to a file called `controller.yaml`:

```yaml
apiVersion: metacontroller.k8s.io/v1alpha1
kind: CompositeController
metadata:
  name: hello-controller
spec:
  generateSelector: true
  parentResource:
    apiVersion: example.com/v1
    resource: helloworlds
  childResources:
  - apiVersion: v1
    resource: pods
    updateStrategy:
      method: Recreate
  hooks:
    sync:
      webhook:
        url: http://hello-controller.hello/sync
```

Then apply it to your cluster:

```sh
kubectl apply -f controller.yaml
```

This tells Metacontroller to start a [reconciling control loop][controller]
for you, running inside the Metacontroller server.
The parameters under `spec:` let you tune the behavior of the controller
[declaratively][declarative reconciliation].

In this case:

* We set [`generateSelector`][generateSelector] to `true` to mimic the built-in
  [Job][job selector] API since we're running a Pod to completion and don't want
  to share Pods across invocations.
* The [`parentResource`][parentResource] is our custom resource called `helloworlds`.
* The idea of CompositeController is that the parent resource represents
  objects that are composed of other objects.
  A HelloWorld is composed of just a Pod, so we have only one entry in the
  [`childResources`][childResources] list.
* For each child resource, we can optionally set an
  [`updateStrategy`][updateStrategy] to specify what to do if a child object
  needs to be updated.
  Since Pods are effectively immutable, we use the `Recreate` method,
  which means, "delete the outdated object and create a new one".
* Finally, we tell Metacontroller how to invoke the `sync` webhook,
  which is where we'll define the business logic of our controller.
  The example relies on in-cluster DNS to resolve the address of the
  `hello-controller` Service (which we'll define below)
  within the `hello` Namespace.

[CompositeController]: /api/compositecontroller/
[controller]: /concepts/#controller
[declarative reconciliation]: /features/#declarative-reconciliation
[generateSelector]: /api/compositecontroller/#generate-selector
[job selector]: https://kubernetes.io/docs/concepts/workloads/controllers/jobs-run-to-completion/#specifying-your-own-pod-selector
[parentResource]: /api/compositecontroller/#parent-resource
[childResources]: /api/compositecontroller/#child-resources
[updateStrategy]: /api/compositecontroller/#child-update-strategy

### Write a webhook

Metacontroller will handle the [controllery bits](/features/#controller-best-practices)
for us, but we still need to tell it what our controller actually does.

To define our business logic, we write a webhook that generates child objects
based on the parent spec, which is provided as JSON in the webhook request.
The [sync hook request][] contains additional information as well,
but the parent spec is all we need for this example.

You can write Metacontroller hooks in any language, but Python is particularly
nice because its *dictionary* type is convenient for programmatically building
JSON objects (like the Pod object below).

If you have a preferred Functions-as-a-Service framework, you can use that to
write your webhook, but we'll keep this example self-contained by relying on
the basic HTTP server module in the Python standard library.
The `do_POST()` method handles decoding and encoding the request and response
as JSON.

The real hook logic is in the `sync()` method, and consists primarily of
building a Pod object.
Because Metacontroller uses [apply semantics][], you can simply return the
Pod object as if you were creating it, every time.
If the Pod already exists, Metacontroller will take care of updates according
to your [update strategy][updateStrategy].

In this case, we set the update method to `Recreate`, so an existing Pod
would be deleted and replaced if it doesn't match the desired state returned
by your hook.
Notice, however, that the hook code below doesn't need to mention any of that
because it's only responsible for computing the desired state;
the Metacontroller server takes care of
[reconciling with the observed state][declarative reconciliation].

[sync hook request]: /api/compositecontroller/#sync-hook-request
[apply semantics]: /api/apply/

Save the following to a file called `sync.py`:

```python
from BaseHTTPServer import BaseHTTPRequestHandler, HTTPServer
import json

class Controller(BaseHTTPRequestHandler):
  def sync(self, parent, children):
    # Compute status based on observed state.
    desired_status = {
      "pods": len(children["Pod.v1"])
    }

    # Generate the desired child object(s).
    who = parent.get("spec", {}).get("who", "World")
    desired_pods = [
      {
        "apiVersion": "v1",
        "kind": "Pod",
        "metadata": {
          "name": parent["metadata"]["name"]
        },
        "spec": {
          "restartPolicy": "OnFailure",
          "containers": [
            {
              "name": "hello",
              "image": "busybox",
              "command": ["echo", "Hello, %s!" % who]
            }
          ]
        }
      }
    ]

    return {"status": desired_status, "children": desired_pods}

  def do_POST(self):
    # Serve the sync() function as a JSON webhook.
    observed = json.loads(self.rfile.read(int(self.headers.getheader("content-length"))))
    desired = self.sync(observed["parent"], observed["children"])

    self.send_response(200)
    self.send_header("Content-type", "application/json")
    self.end_headers()
    self.wfile.write(json.dumps(desired))

HTTPServer(("", 80), Controller).serve_forever()
```

Then load it into your cluster as a ConfigMap:

```sh
kubectl -n hello create configmap hello-controller --from-file=sync.py
```

*Note*: The `-n hello` flag is important to put the ConfigMap in the
`hello` namespace we created for the tutorial.

### Deploy the webhook

Finally, since we wrote our hook as a self-contained Python web server,
we need to deploy it somewhere that Metacontroller can reach.
Luckily, we have this thing called Kubernetes which is great at hosting
stateless web services.

Since our hook consists of only a small Python script, we'll use a generic
Python container image and mount the script from the ConfigMap we created.

Save the following to a file called `webhook.yaml`:

```yaml
apiVersion: apps/v1beta1
kind: Deployment
metadata:
  name: hello-controller
spec:
  replicas: 1
  selector:
    matchLabels:
      app: hello-controller
  template:
    metadata:
      labels:
        app: hello-controller
    spec:
      containers:
      - name: controller
        image: python:2.7
        command: ["python", "/hooks/sync.py"]
        volumeMounts:
        - name: hooks
          mountPath: /hooks
      volumes:
      - name: hooks
        configMap:
          name: hello-controller
---
apiVersion: v1
kind: Service
metadata:
  name: hello-controller
spec:
  selector:
    app: hello-controller
  ports:
  - port: 80
```

Then apply it to your cluster:

```sh
kubectl -n hello apply -f webhook.yaml
```

### Try it out

Now we can create HelloWorld objects and see what they do.

Save the following to a file called `hello.yaml`:

```yaml
apiVersion: example.com/v1
kind: HelloWorld
metadata:
  name: your-name
spec:
  who: Your Name
```

Then apply it to your cluster:

```sh
kubectl -n hello apply -f hello.yaml
```

Our controller should see this and create a Pod that prints a greeting
and then exits.
If you list all Pods (with the `-a` flag) in the `hello` namespace:

```sh
kubectl -n hello get pods -a
```

You should see something like this:

```console
NAME                                READY     STATUS      RESTARTS   AGE
hello-controller-746fc7c4dc-rzslh   1/1       Running     0          2m
your-name                           0/1       Completed   0          15s
```

Then you can check the logs on the *Completed* Pod:

```sh
kubectl -n hello logs your-name
```

Which should look like this:

```console
Hello, Your Name!
```

Now let's look at what happens when you update the parent object,
for example to change the name:

```sh
kubectl -n hello patch helloworld your-name --type=merge -p '{"spec":{"who":"My Name"}}'
```

If you now check the Pod logs again:

```sh
kubectl -n hello logs your-name
```

You should see that the Pod was updated (actually deleted and recreated)
to print a greeting to the new name, even though the hook code doesn't
mention anything about updates.

```console
Hello, My Name!
```

### Clean up

Another thing Metacontroller does for you by default is set up links
so that child objects are removed by the garbage collector when the parent
goes away (assuming your cluster is version 1.8+).

You can check this by deleting the parent:

```sh
kubectl -n hello delete helloworld your-name
```

And then checking for the child Pod:

```sh
kubectl -n hello get pods -a
```

You should see that the child Pod was cleaned up automatically,
so only the webhook Pod remains:

```console
NAME                                READY     STATUS      RESTARTS   AGE
hello-controller-746fc7c4dc-rzslh   1/1       Running     0          3m
```

When you're done with the tutorial, you should remove the controller,
CRD, and Namespace as follows:

```sh
kubectl delete compositecontroller hello-controller
kubectl delete crd helloworlds.example.com
kubectl delete ns hello
```

## Next Steps

* Explore other [example controllers](/examples/).
* Read about [best practices](/guide/best-practices/) for writing controllers.
* Learn how to [troubleshoot controllers](/guide/troubleshooting/).
* Dive into the details of all the [available Metacontroller APIs](/api/).
