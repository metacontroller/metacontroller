/*
Copyright 2018 Google Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package composite

import (
	"context"
	"errors"
	"time"

	"metacontroller/pkg/logging"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"

	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/finalizer"

	"k8s.io/client-go/tools/record"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"metacontroller/pkg/events"

	v1 "k8s.io/api/core/v1"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	mclisters "metacontroller/pkg/client/generated/lister/metacontroller/v1alpha1"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
)

type Metacontroller struct {
	// k8sClient is a client used to interact with the Kubernetes API
	k8sClient     client.Client
	resources     *dynamicdiscovery.ResourceMap
	dynClient     *dynamicclientset.Clientset
	dynInformers  *dynamicinformer.SharedInformerFactory
	eventRecorder record.EventRecorder

	mcClient mcclientset.Interface

	revisionLister   mclisters.ControllerRevisionLister
	revisionInformer cache.SharedIndexInformer

	parentControllers common.SyncMap[string, runningController]
	// newController creates the controller of one CompositeController.
	newController func(cc *v1alpha1.CompositeController) (runningController, error)
	// cleanupCursors keeps the position of each finalizer cleanup.
	cleanupCursors finalizer.Cursors
	// cleanupBudget is the finalizer.ControllerCleaner Budget.
	cleanupBudget time.Duration

	numWorkers int
	ssaOptions *common.ApplyOptions
	logger     logr.Logger
	ctx        context.Context
}

func NewMetacontroller(ctx context.Context, controllerContext common.ControllerContext, mcClient mcclientset.Interface, numWorkers int, ssaOptions *common.ApplyOptions) *Metacontroller {
	mc := &Metacontroller{
		k8sClient:     controllerContext.K8sClient,
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		mcClient: mcClient,

		revisionLister:   controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Lister(),
		revisionInformer: controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Informer(),

		numWorkers: numWorkers,
		ssaOptions: ssaOptions,
		logger:     logging.Logger.WithName("composite"),
		ctx:        ctx,
	}
	mc.newController = mc.createController

	return mc
}

// runningController is the controller of one CompositeController.
type runningController interface {
	Start()
	Stop()
	// object returns the CompositeController that the controller was created from.
	object() *v1alpha1.CompositeController
}

func (pc *parentController) object() *v1alpha1.CompositeController {
	return pc.cc
}

func (mc *Metacontroller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	compositeControllerName := request.Name
	mc.logger.Info("Sync CompositeController", "name", compositeControllerName)

	cc := v1alpha1.CompositeController{}
	err := mc.k8sClient.Get(ctx, request.NamespacedName, &cc)
	if apierrors.IsNotFound(err) {
		mc.logger.Info("CompositeController has been deleted", "name", compositeControllerName)
		// The object can be gone before Finalize completed, for example when
		// a person removed the controller finalizer. Remove its cursor.
		mc.cleanupCursors.Delete(request.NamespacedName)
		// Stop and remove the controller if it exists.
		if pc, ok := mc.parentControllers.LoadAndDelete(compositeControllerName); ok {
			pc.Stop()
			mc.eventRecorder.Eventf(
				pc.object(),
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", pc.object().Name)
		}
		return reconcile.Result{}, nil
	}

	if err != nil {
		mc.logger.Error(err, "Sync error")
		mc.eventRecorder.Eventf(
			&cc, v1.EventTypeNormal,
			events.ReasonSyncError,
			"[%s] Sync error - %s", cc.Name, err)
		return reconcile.Result{}, err
	}

	// Do this before the checks of the parent CRD. The parent CRD can be
	// deleted before the CompositeController.
	if cc.GetDeletionTimestamp() != nil {
		return mc.finalizeCompositeController(ctx, &cc)
	}

	if err := mc.ensureControllerFinalizer(ctx, &cc); err != nil {
		mc.eventRecorder.Eventf(
			&cc,
			v1.EventTypeWarning,
			events.ReasonSyncError,
			"[%s] can't add finalizer - %s", cc.Name, err)
		return reconcile.Result{}, err
	}

	// Clean the retired resources. A cleanup error must not stop the
	// controller of the current spec, so the reconcile continues. The cleaner
	// records the event.
	var cleanupResult reconcile.Result
	var cleanupErr error
	if hasControllerFinalizer(&cc) {
		cleanupResult, cleanupErr = mc.cleanupRetiredParents(ctx, &cc)
	}

	// The gates and the start do not stop the requeue of the cleanup. See
	// finalizer.KeepRequeue.
	groupVersion, err := schema.ParseGroupVersion(cc.Spec.ParentResource.APIVersion)
	if err != nil {
		mc.recordGateError(&cc, err)
		return finalizer.KeepRequeue(mc.logger, cleanupResult, errors.Join(cleanupErr, err))
	}
	parentCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = mc.k8sClient.Get(ctx,
		client.ObjectKey{
			Name: cc.Spec.ParentResource.Resource + "." + groupVersion.Group,
		},
		parentCRD)
	if err != nil {
		mc.recordGateError(&cc, err)
		return finalizer.KeepRequeue(mc.logger, cleanupResult, errors.Join(cleanupErr, err))
	}
	if !common.HasStatusSubresource(parentCRD, groupVersion.Version) {
		mc.eventRecorder.Eventf(
			&cc,
			v1.EventTypeWarning,
			events.ReasonSyncError,
			"[%s] Sync error - ignoring, CRD [%s] does not have subresource 'Status' enabled",
			cc.Name,
			parentCRD.GroupVersionKind())
		mc.logger.Info("Ignoring CompositeController",
			"name", compositeControllerName,
			"reason", "subresource 'Status' not enabled",
			"groupVersionKind", parentCRD.GroupVersionKind())
		// returning, as we cannot do anything until 'Status' subresource is added to parent parentCRD
		return finalizer.KeepRequeue(mc.logger, cleanupResult, cleanupErr)
	}
	reconcileErr := mc.reconcileCompositeController(ctx, &cc)
	return finalizer.KeepRequeue(mc.logger, cleanupResult, errors.Join(cleanupErr, reconcileErr))
}

// recordGateError records an error of a check of the parent resource.
func (mc *Metacontroller) recordGateError(cc *v1alpha1.CompositeController, err error) {
	mc.eventRecorder.Eventf(cc, v1.EventTypeWarning, events.ReasonSyncError, "[%s] Sync error - %s", cc.Name, err)
}

func (mc *Metacontroller) reconcileCompositeController(ctx context.Context, cc *v1alpha1.CompositeController) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	running, ok := mc.parentControllers.Load(cc.Name)
	if ok && apiequality.Semantic.DeepEqual(cc.Spec, running.object().Spec) {
		// The spec is unchanged. If the record is missing, write it again.
		// A later spec change then finds the old resource.
		if hasControllerFinalizer(cc) && !finalizer.HasCleanupRecord(cc) {
			return mc.recordActiveParents(ctx, cc, running)
		}
		return nil
	}
	// Write the record first. It then lists the resource of the old
	// controller.
	var recordErr error
	if hasControllerFinalizer(cc) {
		recordErr = mc.recordActiveParents(ctx, cc, running)
		if recordErr != nil && (ok || !errors.Is(recordErr, finalizer.ErrInvalidCleanupRecord)) {
			// Keep the old controller. The next reconcile tries again.
			return recordErr
		}
		// recordErr is not nil here only if no controller runs and the
		// record is not valid. A wait does not help, because the old
		// resources stay unknown until a person repairs the record. Start
		// the controller.
	}
	// Stop and remove the controller so it can be recreated.
	if pc, ok := mc.parentControllers.LoadAndDelete(cc.Name); ok {
		pc.Stop()
		mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", cc.Name)
	}

	pc, err := mc.newController(cc)
	if err != nil {
		mc.eventRecorder.Eventf(
			cc,
			v1.EventTypeWarning,
			events.ReasonCreateError,
			"Cannot create new controller: %s", err.Error())
		return errors.Join(recordErr, err)
	}
	pc.Start()
	mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", cc.Name)
	mc.parentControllers.Store(cc.Name, pc)
	return recordErr
}

// createController creates the controller of one CompositeController.
func (mc *Metacontroller) createController(cc *v1alpha1.CompositeController) (runningController, error) {
	return newParentController( //nolint:contextcheck // long-lived controller must use root context, not per-reconcile ctx
		mc.ctx,
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		mc.mcClient,
		mc.revisionLister,
		cc,
		mc.numWorkers,
		mc.ssaOptions,
		mc.logger,
		mc.k8sClient)
}

// cleanupRetiredParents cleans one slice. It skips the resource of the spec
// and the resource of the running controller. The running controller can
// differ from the spec, because a gate can stop the reconcile before the new
// controller starts.
func (mc *Metacontroller) cleanupRetiredParents(ctx context.Context, cc *v1alpha1.CompositeController) (reconcile.Result, error) {
	exclude := activeParentResources(cc)
	if pc, ok := mc.parentControllers.Load(cc.Name); ok {
		exclude = append(exclude, activeParentResources(pc.object())...)
	}
	return mc.newCleaner().CleanupRetiredParents(ctx, cc, parentFinalizerName(cc.Name), exclude)
}

// recordActiveParents writes the cleanup record for the spec. If the
// annotation is absent, it takes the old resources from the running
// controller.
func (mc *Metacontroller) recordActiveParents(ctx context.Context, cc *v1alpha1.CompositeController, running runningController) error {
	var runningParents []finalizer.ParentResource
	if running != nil {
		runningParents = activeParentResources(running.object())
	}
	return mc.newCleaner().RecordActiveParents(ctx, cc, runningParents, activeParentResources(cc))
}
