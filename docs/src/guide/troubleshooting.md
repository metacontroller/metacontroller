# Troubleshooting

This is a collection of tips for debugging controllers written with Metacontroller.

If you have something to add to the collection, please send a pull request against
[this document](https://github.com/metacontroller/metacontroller/tree/master/docs/src/guide/troubleshooting.md).

[[_TOC_]]

## Events

As metacontroller emits kubernetes Events for internal actions, you might check events on parent object, like:
```shell
kubectl describe secretpropagations.examples.metacontroller.io <name>
```
where, at the end, you will see all events related with given parent:
```yaml
Name:         secret-propagation
Namespace:    
Labels:       <none>
Annotations:  <none>
API Version:  examples.metacontroller.io/v1alpha1
Kind:         SecretPropagation
Metadata:
  Creation Timestamp:  2021-07-14T20:25:09Z
...
Spec:
  Source Name:       shareable
  Source Namespace:  omega
  Target Namespace Label Selector:
    Match Labels:
      Propagate:  true
Status:
  Working:  fine
Events:
  Type     Reason     Age               From            Message
  ----     ------     ----              ----            -------
  Warning  SyncError  1s (x11 over 8s)  metacontroller  Sync error: sync hook failed for SecretPropagation /secret-propagation: sync hook failed: http error: Post "http://secret-propagation-controller.metacontroller/sync": dial tcp 10.96.138.14:80: connect: connection refused

```

You can access also events using `kubectl get events`, which return all events from given namespace. As metacontroller
CRD's are might be cluster wide, they can land in `default` namespace:
```shell
> kubectl get events -n default  
39m         Normal    Started                 compositecontroller/secret-propagation-controller      Started controller: secret-propagation-controller
39m         Normal    Starting                compositecontroller/secret-propagation-controller      Starting controller: secret-propagation-controller
39m         Normal    Stopping                compositecontroller/secret-propagation-controller      Stopping controller: secret-propagation-controller
39m         Normal    Stopped                 compositecontroller/secret-propagation-controller      Stopped controller: secret-propagation-controller
6m25s       Normal    Started                 compositecontroller/secret-propagation-controller      Started controller: secret-propagation-controller
6m25s       Normal    Starting                compositecontroller/secret-propagation-controller      Starting controller: secret-propagation-controller
2m27s       Normal    Stopping                compositecontroller/secret-propagation-controller      Stopping controller: secret-propagation-controller
2m27s       Normal    Stopped                 compositecontroller/secret-propagation-controller      Stopped controller: secret-propagation-controller

```

## Metacontroller Logs

Until Metacontroller [emits events](https://www.github.com/GoogleCloudPlatform/metacontroller/issues/7),
the first place to look when troubleshooting controller behavior is the logs for
the Metacontroller server itself.

For example, you can fetch the last 25 lines with a command like this:

```shell
kubectl -n metacontroller logs --tail=25 -l app=metacontroller
```

### Log Levels

You can customize the verbosity of the Metacontroller server's logs with the
`--zap-log-level` flag.

At all log levels, Metacontroller will log the progress of server startup and
shutdown, as well as major changes like starting and stopping hosted controllers.

At level 4 and above, Metacontroller will log actions (like create/update/delete)
on individual objects (like Pods) that it takes on behalf of hosted controllers.
It will also log when it decides to sync a given controller as well as events
that may trigger a sync.

At level 5 and above, Metacontroller will log the diffs between existing objects,
and the desired state of those objects returned by controller hooks.

At level 6 and above, Metacontroller will log every hook invocation as well as
the JSON request and response bodies.

### Common Log Messages

Since API discovery info is refreshed periodically, you may see log messages
like this when you start a controller that depends on a recently-installed CRD:

```plaintext
failed to sync CompositeController "my-controller": discovery: can't find resource <resource> in apiVersion <group>/<version>
```

Usually, this should fix itself within about 30s when the new CRD is discovered.
If this message continues indefinitely, check that the resource name and API
group/version are correct.

You may also notice periodic log messages like this:

```plaintext
Watch close - *unstructured.Unstructured total <X> items received
```

This comes from the underlying client-go library, and just indicates when the
shared caches are periodically flushed to place an upper bound on cache
inconsistency due to potential silent failures in long-running watches.

## Webhook Logs

If you return an HTTP error code (e.g., 500) from your webhook,
the Metacontroller server will log the text of the response body.

If you need more detail on what's happening inside your hook code, as opposed to
what Metacontroller does for you, you'll need to add log statements to your own
code and inspect the logs on your webhook server.
