# DYNAMIC CLIENT (`pkg/dynamic/`)

Metacontroller works with resource types that it does not know at compile
time. This package gives access to them: discovery, clients, informers,
apply, and owner references.

## STRUCTURE
```
pkg/dynamic/
├── discovery/      # API resource cache; refreshed at the --discovery-interval
├── clientset/      # ResourceClient: get, list, update for one resource type
├── informer/       # ResourceInformer and the informer factory
├── apply/          # merge logic for dynamic apply
├── controllerref/  # owner references: adopt and release children
└── object/         # generic object helpers (status.go: HasStatusSubresource)
```

## WHERE TO LOOK
| Task | File |
|------|------|
| How metacontroller talks to a custom resource | `clientset/clientset.go` |
| Discovery cache | `discovery/discovery.go` |
| Watch setup for a resource | `informer/factory.go` |
| Merge behavior of dynamic apply | `apply/apply.go` |
| Status subresource check | `object/status.go` |

## RULES
- Other packages import these packages with fixed aliases: `dynamicclientset`,
  `dynamicdiscovery`, `dynamicinformer`. Keep those names.
- A new resource type goes through discovery, then clientset, then informer.
  The informer factory depends on the discovery result.
- The apply strategy (server-side apply or dynamic apply) comes from
  `pkg/options` `Configuration`. The appliers are in
  `pkg/controller/common/manage_children.go`.
- `apply/apply.go` has an open `TODO` about cases where a merge is wanted. Read
  it before you change the default apply behavior.
- Library updates of `client-go` and `controller-runtime` frequently need
  changes in `informer/`. See [dependencies.md](dependencies.md).

## INFORMER WRAPPER (`informer/informer.go`)
The shared informer is a `client-go` dynamic informer. The local code adds the
factory with a reference count, the `ownerUID` and `orphan` indexes, and the
handler tracking in `informerWrapper`.

- Add handlers in a start path. Remove them with `RemoveEventHandlers` in a
  stop path, before `Close`. Do not remove a handler from a handler callback:
  `client-go` waits for the handler and the call does not return.
- `client-go` gives each handler its own goroutines and its own notification
  buffer. The buffer grows to the cache size during a replay and does not
  become smaller. Measure the memory before you add more handlers for each
  controller.
- A new handler gets one `OnAdd` call for each cached object, with
  `isInInitialList` set to true. `SkipInitialList` drops these calls. Use it
  only when the sync reads the same objects from the cache.
- Do not pass a resync period to `client-go`. It changes the period of the
  shared informer. A local timer does the short resync for one handler.

## CHECK AFTER A `client-go` UPDATE
The wrapper depends on these facts of `client-go`. The tests in
`informer/informer_test.go` guard the first two. Check each one in the new
`tools/cache/shared_informer.go`:

- `isInInitialList` is true only for the first list and for the replay to a
  new handler. A later relist after a watch error must give false. If this
  changes, `SkipInitialList` drops objects that were created during a watch
  gap.
- `cache.ShutDownEventHandler` waits until the handler stops. It returns
  immediately when the informer did not start or has stopped.
- A resync period shorter than the informer period is raised to the informer
  period after start. This is the reason for the local timer.
- The registration (`cache.ResourceEventHandlerRegistration`) is a key in the
  map `informerWrapper.handlers`. Its type must stay comparable. Today it is
  a pointer.
