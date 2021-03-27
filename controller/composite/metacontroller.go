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
	ctx "context"

	"k8s.io/client-go/tools/record"
	"metacontroller.io/controller/common"

	dynamicclientset "metacontroller.io/dynamic/clientset"
	dynamicdiscovery "metacontroller.io/dynamic/discovery"
	dynamicinformer "metacontroller.io/dynamic/informer"

	v1 "k8s.io/api/core/v1"
	"metacontroller.io/events"

	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	apiequality "k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/tools/cache"
	"metacontroller.io/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/client/generated/clientset/internalclientset"
	mclisters "metacontroller.io/client/generated/lister/metacontroller/v1alpha1"
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

	parentControllers map[string]*parentController

	stopCh, doneCh chan struct{}

	numWorkers int
}

func NewMetacontroller(controllerContext common.ControllerContext, mcClient mcclientset.Interface, numWorkers int) *Metacontroller {
	mc := &Metacontroller{
		k8sClient:     controllerContext.K8sClient,
		resources:     controllerContext.Resources,
		dynClient:     controllerContext.DynClient,
		dynInformers:  controllerContext.DynInformers,
		eventRecorder: controllerContext.EventRecorder,

		mcClient: mcClient,

		revisionLister:   controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Lister(),
		revisionInformer: controllerContext.McInformerFactory.Metacontroller().V1alpha1().ControllerRevisions().Informer(),

		parentControllers: make(map[string]*parentController),

		numWorkers: numWorkers,
	}

	return mc
}

func (mc *Metacontroller) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	compositeControllerName := request.Name
	klog.V(4).InfoS("Sync CompositeController", "name", compositeControllerName)

	cc := v1alpha1.CompositeController{}
	err := mc.k8sClient.Get(ctx.Background(), request.NamespacedName, &cc)
	if apierrors.IsNotFound(err) {
		klog.V(4).InfoS("CompositeController has been deleted", "name", compositeControllerName)
		// Stop and remove the controller if it exists.
		if pc, ok := mc.parentControllers[compositeControllerName]; ok {
			pc.Stop()
			defer pc.eventRecorder.Eventf(
				pc.cc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", pc.cc.Name)
			delete(mc.parentControllers, compositeControllerName)
		}
		return reconcile.Result{}, nil
	}

	if err != nil {
		mc.eventRecorder.Eventf(
			&cc, v1.EventTypeNormal,
			events.ReasonSyncError,
			"[%s] Sync error - %s", cc.Name, err)
		return reconcile.Result{}, err
	}
	parentClient, err := mc.dynClient.Resource(cc.Spec.ParentResource.APIVersion, cc.Spec.ParentResource.Resource)
	if err != nil {
		return reconcile.Result{}, err
	}
	if found := parentClient.APIResource.HasSubresource("status"); !found {
		mc.eventRecorder.Eventf(
			&cc,
			v1.EventTypeWarning,
			events.ReasonSyncError,
			"[%s] Sync error - ignoring, parent resource %s does not have subresource 'Status' enabled",
			cc.Name,
			parentClient.GroupVersionKind())
		klog.InfoS("Ignoring CompositeController",
			"name", compositeControllerName,
			"reason", "subresource 'Status' not enabled",
			"groupVersionKind", parentClient.GroupVersionKind())
		// returning, as we cannot do anything until 'Status' subresource is added to parent resource
		return reconcile.Result{}, nil
	}
	reconcileErr := mc.reconcileCompositeController(&cc)
	return reconcile.Result{}, reconcileErr
}

func (mc *Metacontroller) reconcileCompositeController(cc *v1alpha1.CompositeController) error {
	if pc, ok := mc.parentControllers[cc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(cc.Spec, pc.cc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		pc.Stop()
		mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", cc.Name)
		delete(mc.parentControllers, cc.Name)
	}

	pc, err := newParentController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		mc.mcClient,
		mc.revisionLister,
		cc,
		mc.numWorkers)
	if err != nil {
		return err
	}
	pc.Start()
	mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", cc.Name)
	mc.parentControllers[cc.Name] = pc
	return nil
}
