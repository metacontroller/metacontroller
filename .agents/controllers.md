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
| `metacontroller.go` | Starts and stops one controller for each controller object. Contains `Metacontroller` and `NewMetacontroller`. The controller map contains `runningController` values. Unit tests replace `newController` with a stub. |
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
| Cleanup when a controller object is deleted | `common/finalizer/cleanup.go`, `{composite,decorator}/controller_finalizer.go` | `ControllerCleaner.Finalize` removes the parent finalizer from all parents; `ControllerFinalizer` keeps the controller object until then |
| Cleanup after a change of parent resources | `common/finalizer/cleanup_state.go`, the two `metacontroller.go` files | `RecordActiveParents` and `CleanupRetiredParents`. Read "PARENT FINALIZER CLEANUP". |
| Diff and equality | `common/diff.go`, `common/equality.go` | shared update primitives; use them, do not write new ones |

## PARENT FINALIZER CLEANUP
This section applies to `common/finalizer/cleanup_state.go` and to the two
`metacontroller.go` files.

Terms:
- Record: the annotation `metacontroller.io/parent-finalizer-cleanup-state`
  on the controller object. The record has two lists of parent resources:
  `active` and `removeParentFinalizerFrom`.
- Entry: one parent resource in a list of the record.
- Retired resource: a parent resource that the spec contained before and does
  not contain now.
- Discovery: the list of the APIs that the API server serves.
- Requeue: Metacontroller puts the controller object back in the work queue
  after a delay (`RequeueAfter`).

How the code works:
1. `RecordActiveParents` writes the record. It moves each retired resource to
   `removeParentFinalizerFrom`.
2. `CleanupRetiredParents` removes the parent finalizer from the objects of
   each entry in `removeParentFinalizerFrom`.
3. The cleanup runs on each reconcile. It also runs when the spec does not
   change.

Rules:
- Compare parent resources by `GroupResource`. Do not compare them by
  `APIVersion`.
- Run the two functions only when the controller object has
  `ControllerFinalizer`.
- A finalize hook gives `ControllerFinalizer` to the controller object. The
  object keeps this finalizer when you remove the hook.

An entry that discovery does not list:

| Condition | Action |
|-----------|--------|
| The CRD of the resource exists. | Keep the entry. |
| No CRD. Discovery lists the `apiVersion`. | Remove the entry. |
| No CRD. The APIService of the `apiVersion` exists. | Keep the entry. |
| No CRD. No APIService. | Remove the entry. |

When the cleaner removes an entry, it records a `Warning` event. When it keeps
an entry, it tries the entry again after 30 seconds.

Errors and requeue:
- Do not let a cleanup error stop the start of the controller.
- controller-runtime ignores `RequeueAfter` when `Reconcile` returns an
  error.
- The cleaner records an entry error as an event. Then it returns
  `RequeueAfter` of 30 seconds and a nil error.
- A gate error is an error from a check before the controller starts. An
  example is the check of the parent CRD.
- While the cleanup has a requeue, send a gate error or a start error through
  `finalizer.KeepRequeue`. Record the event for the error before you call
  `KeepRequeue`.

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
