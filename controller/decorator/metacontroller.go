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
	ctx "context"

	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"

	"k8s.io/client-go/tools/record"

	v1 "k8s.io/api/core/v1"
	"metacontroller.io/events"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"metacontroller.io/apis/metacontroller/v1alpha1"
	"metacontroller.io/controller/common"
)

type Metacontroller struct {
	// k8sClient is a client used to interact with the Kubernetes API
	k8sClient    client.Client
	resources    *dynamicdiscovery.ResourceMap
	dynClient    *dynamicclientset.Clientset
	dynInformers *dynamicinformer.SharedInformerFactory

	eventRecorder record.EventRecorder

	decoratorControllers map[string]*decoratorController

	stopCh, doneCh chan struct{}

	numWorkers int
}

func NewMetacontroller(controllerContext common.ControllerContext, numWorkers int) *Metacontroller {
	mc := &Metacontroller{
		k8sClient:     controllerContext.K8sClient,
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		decoratorControllers: make(map[string]*decoratorController),

		numWorkers: numWorkers,
	}

	return mc
}

func (mc *Metacontroller) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	decoratorControllerName := request.Name
	klog.V(4).InfoS("Sync DecoratorController", "name", decoratorControllerName)

	dc := v1alpha1.DecoratorController{}
	err := mc.k8sClient.Get(ctx.Background(), request.NamespacedName, &dc)
	if apierrors.IsNotFound(err) {
		klog.V(4).InfoS("DecoratorController has been deleted", "name", decoratorControllerName)
		// Stop and remove the controller if it exists.
		if c, ok := mc.decoratorControllers[decoratorControllerName]; ok {
			c.Stop()
			defer c.eventRecorder.Eventf(
				c.dc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", c.dc.Name)
			delete(mc.decoratorControllers, decoratorControllerName)
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
	if c, ok := mc.decoratorControllers[dc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(dc.Spec, c.dc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		c.Stop()
		mc.eventRecorder.Eventf(dc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", dc.Name)
		delete(mc.decoratorControllers, dc.Name)
	}

	c, err := newDecoratorController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		dc,
		mc.numWorkers,
	)
	if err != nil {
		return err
	}
	c.Start()
	mc.eventRecorder.Eventf(dc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", dc.Name)
	mc.decoratorControllers[dc.Name] = c
	return nil
}
