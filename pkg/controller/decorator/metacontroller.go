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

	DecoratorControllers map[string]*decoratorController

	numWorkers int

	logger logr.Logger
}

func NewMetacontroller(controllerContext common.ControllerContext, numWorkers int) *Metacontroller {
	mc := &Metacontroller{
		k8sClient:     controllerContext.K8sClient,
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		DecoratorControllers: make(map[string]*decoratorController),

		numWorkers: numWorkers,

		logger: logging.Logger.WithName("decorator"),
	}

	return mc
}

func (mc *Metacontroller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	decoratorControllerName := request.Name
	mc.logger.V(4).Info("Sync DecoratorController", "name", decoratorControllerName)

	dc := v1alpha1.DecoratorController{}
	err := mc.k8sClient.Get(ctx, request.NamespacedName, &dc)
	if apierrors.IsNotFound(err) {
		mc.logger.V(4).Info("DecoratorController has been deleted", "name", decoratorControllerName)
		// Stop and remove the controller if it exists.
		if c, ok := mc.DecoratorControllers[decoratorControllerName]; ok {
			c.Stop()
			defer c.eventRecorder.Eventf(
				c.dc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", c.dc.Name)
			delete(mc.DecoratorControllers, decoratorControllerName)
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
	reconcileErr := mc.reconcileDecoratorController(&dc)
	return reconcile.Result{}, reconcileErr
}

func (mc *Metacontroller) reconcileDecoratorController(dc *v1alpha1.DecoratorController) error {
	if c, ok := mc.DecoratorControllers[dc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(dc.Spec, c.dc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		c.Stop()
		mc.eventRecorder.Eventf(
			dc,
			v1.EventTypeNormal,
			events.ReasonStopped,
			"Stopped controller: %s", dc.Name)
		delete(mc.DecoratorControllers, dc.Name)
	}

	c, err := newDecoratorController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		dc,
		mc.numWorkers,
		mc.logger,
	)
	if err != nil {
		mc.eventRecorder.Eventf(
			dc,
			v1.EventTypeWarning,
			events.ReasonCreateError,
			"Cannot create new controller: %s", err.Error())
		return err
	}
	c.Start()
	mc.eventRecorder.Eventf(
		dc,
		v1.EventTypeNormal,
		events.ReasonStarted,
		"Started controller: %s", dc.Name)
	mc.DecoratorControllers[dc.Name] = c
	return nil
}
