# CONTROLLERS (`pkg/controller/`)

There are two controller kinds, composite and decorator. They share the code in
`common/`.

## STRUCTURE
```
pkg/controller/
├── common/      # shared context, child management, apply, customize hook, finalizer
├── composite/   # CompositeController reconcile; api/{common,v1,v2}
└── decorator/   # DecoratorController reconcile; api/{common,v1,v2}
```

Each kind has the same shape:

| File | Role |
|------|------|
| `metacontroller.go` | lifecycle: `Metacontroller`, `NewMetacontroller`, start and stop of one controller for each controller object |
| `controller.go` | the sync loop |
| `hooks.go` | hook plumbing |
| `api/v1`, `api/v2` | hook request and response types |

## WHERE TO LOOK
| Task | Location | Notes |
|------|----------|-------|
| Composite sync loop | `composite/controller.go` | large file; do not make it longer |
| Rollouts and revisions | `composite/rolling_update.go`, `composite/controller_revision.go` | uses the ControllerRevision CRD |
| Decorator sync loop | `decorator/controller.go` | near-duplicate of the composite loop |
| Decorator object matching | `decorator/selector.go` | `decoratorSelector.Matches` |
| Child create, update, apply | `common/manage_children.go` | `ManageChildren`, `ChildUpdateStrategy`, `Applier`; the two appliers are `ServerSideApply` and `DynamicApply` |
| Controller wiring | `common/common.go` | `ControllerContext`, `NewControllerContext`, `ParseAPIVersion` |
| Related objects (customize hook) | `common/customize/manager.go` | `GetRelatedObjects`; returns `ErrRelatedInformerNotSynced` while informers start |
| Object maps | `common/api/` | `v1` `RelativeObjectMap`, `v2` `UniformObjectMap` |
| v1 to v2 conversion | `common/api/v2/` | `MakeUniformObjectMap`, `MakeRelativeObjectMap` |
| Finalizers | `common/finalizer/` | `ShouldFinalize` decides if a sync is a finalize sync |
| Diff and equality | `common/diff.go`, `common/equality.go` | shared update primitives; use them, do not write new ones |

## RULES
- The composite and decorator sync loops are near-duplicates. When you fix one,
  examine the other. Apply the fix to both, or say why not.
- New hook API work goes to v2 (`UniformObjectMap`).
- The apply strategy (server-side apply or dynamic apply) comes from
  `pkg/options` `Configuration`. CI runs the e2e tests with the two strategies
  (`manifests/dev` and `manifests/dev-ssa`). A change in child management must
  work with both.
- These files are already longer than 500 lines: `composite/controller.go`,
  `decorator/controller.go`, `composite/controller_revision.go`,
  `common/manage_children.go`, `common/customize/manager.go`, and
  `common/customize/manager_test.go`. Put new logic in a new file.
- A controller that lives for the full process uses the root context, not the
  context of one reconcile. See the `//nolint:contextcheck` comments in the two
  `metacontroller.go` files.

## KNOWN GAPS (TODO COMMENTS IN CODE)
- v2 sync hooks and v2 customize hooks do not report Kubernetes events from
  the hook response. Do not assume that hook activity shows on the object.
- `composite/controller.go`: a parent with `generateSelector: true` cannot be
  found through the UID lookup.
- `composite/controller_revision.go`: open issue 194.
- `decorator/controller.go`: the controller syncs again after its own updates.

Search for `TODO` in the file to find the exact place.
