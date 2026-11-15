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

package decorator

import (
	"context"
	"errors"
	"time"

	"metacontroller/pkg/logging"

	"github.com/go-logr/logr"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"k8s.io/client-go/tools/record"

	"metacontroller/pkg/events"

	v1 "k8s.io/api/core/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/finalizer"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

type Metacontroller struct {
	// k8sClient is a client used to interact with the Kubernetes API
	k8sClient    client.Client
	resources    *dynamicdiscovery.ResourceMap
	dynClient    *dynamicclientset.Clientset
	dynInformers *dynamicinformer.SharedInformerFactory

	eventRecorder record.EventRecorder

	decoratorControllers common.SyncMap[string, runningController]
	// newController creates the controller of one DecoratorController.
	newController func(dc *v1alpha1.DecoratorController) (runningController, error)
	// cleanupCursors keeps the position of each finalizer cleanup.
	cleanupCursors finalizer.Cursors
	// cleanupBudget is the finalizer.ControllerCleaner Budget.
	cleanupBudget time.Duration

	numWorkers int

	ssaOptions *common.ApplyOptions

	logger logr.Logger
	ctx    context.Context
}

func NewMetacontroller(ctx context.Context, controllerContext common.ControllerContext, numWorkers int, ssaOptions *common.ApplyOptions) *Metacontroller {
	mc := &Metacontroller{
		k8sClient:     controllerContext.K8sClient,
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,
		ssaOptions:    ssaOptions,

		numWorkers: numWorkers,

		logger: logging.Logger.WithName("decorator"),
		ctx:    ctx,
	}
	mc.newController = mc.createController

	return mc
}

// runningController is the controller of one DecoratorController.
type runningController interface {
	Start()
	Stop()
	// object returns the DecoratorController that the controller was created from.
	object() *v1alpha1.DecoratorController
}

func (c *decoratorController) object() *v1alpha1.DecoratorController {
	return c.dc
}

func (mc *Metacontroller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	decoratorControllerName := request.Name
	mc.logger.V(4).Info("Sync DecoratorController", "name", decoratorControllerName)

	dc := v1alpha1.DecoratorController{}
	err := mc.k8sClient.Get(ctx, request.NamespacedName, &dc)
	if apierrors.IsNotFound(err) {
		mc.logger.V(4).Info("DecoratorController has been deleted", "name", decoratorControllerName)
		// The object can be gone before Finalize completed, for example when
		// a person removed the controller finalizer. Remove its cursor.
		mc.cleanupCursors.Delete(request.NamespacedName)
		// Stop and remove the controller if it exists.
		if c, ok := mc.decoratorControllers.LoadAndDelete(decoratorControllerName); ok {
			c.Stop()
			mc.eventRecorder.Eventf(
				c.object(),
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", c.object().Name)
		}
		return reconcile.Result{}, nil
	}
	if err != nil {
		mc.eventRecorder.Eventf(
			&dc,
			v1.EventTypeNormal,
			events.ReasonSyncError,
			"[%s] sync error - %s", dc.Name, err)
		return reconcile.Result{}, err
	}

	if dc.GetDeletionTimestamp() != nil {
		return mc.finalizeDecoratorController(ctx, &dc)
	}
	if err := mc.ensureControllerFinalizer(ctx, &dc); err != nil {
		mc.eventRecorder.Eventf(
			&dc,
			v1.EventTypeWarning,
			events.ReasonSyncError,
			"[%s] can't add finalizer - %s", dc.Name, err)
		return reconcile.Result{}, err
	}

	// Clean the retired resources. A cleanup error must not stop the
	// controller of the current spec, so the reconcile continues. The cleaner
	// records the event.
	var cleanupResult reconcile.Result
	var cleanupErr error
	if hasControllerFinalizer(&dc) {
		cleanupResult, cleanupErr = mc.cleanupRetiredParents(ctx, &dc)
	}

	// A start error does not stop the requeue of the cleanup. See
	// finalizer.KeepRequeue.
	reconcileErr := mc.reconcileDecoratorController(ctx, &dc)
	return finalizer.KeepRequeue(mc.logger, cleanupResult, errors.Join(cleanupErr, reconcileErr))
}

func (mc *Metacontroller) reconcileDecoratorController(ctx context.Context, dc *v1alpha1.DecoratorController) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	running, ok := mc.decoratorControllers.Load(dc.Name)
	if ok && apiequality.Semantic.DeepEqual(dc.Spec, running.object().Spec) {
		// The spec is unchanged. If the record is missing, write it again.
		// A later spec change then finds the old resources.
		if hasControllerFinalizer(dc) && !finalizer.HasCleanupRecord(dc) {
			return mc.recordActiveParents(ctx, dc, running)
		}
		return nil
	}
	// Write the record first. It then lists the resources of the old
	// controller.
	var recordErr error
	if hasControllerFinalizer(dc) {
		recordErr = mc.recordActiveParents(ctx, dc, running)
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
	if c, ok := mc.decoratorControllers.LoadAndDelete(dc.Name); ok {
		c.Stop()
		mc.eventRecorder.Eventf(
			dc,
			v1.EventTypeNormal,
			events.ReasonStopped,
			"Stopped controller: %s", dc.Name)
	}

	c, err := mc.newController(dc)
	if err != nil {
		mc.eventRecorder.Eventf(
			dc,
			v1.EventTypeWarning,
			events.ReasonCreateError,
			"Cannot create new controller: %s", err.Error())
		return errors.Join(recordErr, err)
	}
	c.Start()
	mc.eventRecorder.Eventf(
		dc,
		v1.EventTypeNormal,
		events.ReasonStarted,
		"Started controller: %s", dc.Name)
	mc.decoratorControllers.Store(dc.Name, c)
	return recordErr
}

// createController creates the controller of one DecoratorController.
func (mc *Metacontroller) createController(dc *v1alpha1.DecoratorController) (runningController, error) {
	return newDecoratorController( //nolint:contextcheck // long-lived controller must use root context, not per-reconcile ctx
		mc.ctx,
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		dc,
		mc.numWorkers,
		mc.ssaOptions,
		mc.logger,
		mc.k8sClient,
	)
}

// cleanupRetiredParents cleans one slice. It skips the resources of the spec
// and of the running controller.
func (mc *Metacontroller) cleanupRetiredParents(ctx context.Context, dc *v1alpha1.DecoratorController) (reconcile.Result, error) {
	exclude := activeParentResources(dc)
	if c, ok := mc.decoratorControllers.Load(dc.Name); ok {
		exclude = append(exclude, activeParentResources(c.object())...)
	}
	return mc.newCleaner().CleanupRetiredParents(ctx, dc, parentFinalizerName(dc.Name), exclude)
}

// recordActiveParents writes the cleanup record for the spec. If the
// annotation is absent, it takes the old resources from the running
// controller.
func (mc *Metacontroller) recordActiveParents(ctx context.Context, dc *v1alpha1.DecoratorController, running runningController) error {
	var runningParents []finalizer.ParentResource
	if running != nil {
		runningParents = activeParentResources(running.object())
	}
	return mc.newCleaner().RecordActiveParents(ctx, dc, runningParents, activeParentResources(dc))
}
