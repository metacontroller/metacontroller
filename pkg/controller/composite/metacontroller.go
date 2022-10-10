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
	"metacontroller/pkg/logging"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-logr/logr"

	"metacontroller/pkg/controller/common"

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

	ParentControllers map[string]*parentController

	numWorkers int
	logger     logr.Logger
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

		ParentControllers: make(map[string]*parentController),

		numWorkers: numWorkers,
		logger:     logging.Logger.WithName("composite"),
	}

	return mc
}

func (mc *Metacontroller) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	compositeControllerName := request.Name
	mc.logger.Info("Sync CompositeController", "name", compositeControllerName)

	cc := v1alpha1.CompositeController{}
	err := mc.k8sClient.Get(ctx, request.NamespacedName, &cc)
	if apierrors.IsNotFound(err) {
		mc.logger.Info("CompositeController has been deleted", "name", compositeControllerName)
		// Stop and remove the controller if it exists.
		if pc, ok := mc.ParentControllers[compositeControllerName]; ok {
			pc.Stop()
			defer pc.eventRecorder.Eventf(
				pc.cc,
				v1.EventTypeNormal,
				events.ReasonStopped,
				"Stopped controller: %s", pc.cc.Name)
			delete(mc.ParentControllers, compositeControllerName)
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
	groupVersion, err := schema.ParseGroupVersion(cc.Spec.ParentResource.APIVersion)
	if err != nil {
		return reconcile.Result{}, err
	}
	parentCRD := &apiextensionsv1.CustomResourceDefinition{}
	err = mc.k8sClient.Get(ctx,
		client.ObjectKey{
			Name: cc.Spec.ParentResource.Resource + "." + groupVersion.Group,
		},
		parentCRD)
	if err != nil {
		return reconcile.Result{}, err
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
		return reconcile.Result{}, nil
	}
	reconcileErr := mc.reconcileCompositeController(&cc)
	return reconcile.Result{}, reconcileErr
}

func (mc *Metacontroller) reconcileCompositeController(cc *v1alpha1.CompositeController) error {
	if pc, ok := mc.ParentControllers[cc.Name]; ok {
		// The controller was already started.
		if apiequality.Semantic.DeepEqual(cc.Spec, pc.cc.Spec) {
			// Nothing has changed.
			return nil
		}
		// Stop and remove the controller so it can be recreated.
		pc.Stop()
		mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStopped, "Stopped controller: %s", cc.Name)
		delete(mc.ParentControllers, cc.Name)
	}

	pc, err := newParentController(
		mc.resources,
		mc.dynClient,
		mc.dynInformers,
		mc.eventRecorder,
		mc.mcClient,
		mc.revisionLister,
		cc,
		mc.numWorkers,
		mc.logger)
	if err != nil {
		mc.eventRecorder.Eventf(
			cc,
			v1.EventTypeWarning,
			events.ReasonCreateError,
			"Cannot create new controller: %s", err.Error())
		return err
	}
	pc.Start()
	mc.eventRecorder.Eventf(cc, v1.EventTypeNormal, events.ReasonStarted, "Started controller: %s", cc.Name)
	mc.ParentControllers[cc.Name] = pc
	return nil
}
