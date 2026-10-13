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
