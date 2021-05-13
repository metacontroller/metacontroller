/*
Copyright 2019 Google Inc.

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

package server

import (
	"fmt"

	"metacontroller.io/pkg/controller/common"

	"metacontroller.io/pkg/controller/decorator"
	"metacontroller.io/pkg/options"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"metacontroller.io/pkg/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller.io/pkg/client/generated/clientset/internalclientset"
	"metacontroller.io/pkg/controller/composite"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// New returns a new controller manager and a function which can be used
// to release resources after the manager is stopped.
func New(configuration options.Configuration) (controllerruntime.Manager, func(), error) {
	// Create informer factory for metacontroller API objects.
	mcClient, err := mcclientset.NewForConfig(configuration.RestConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("can't create client for api %s: %v", v1alpha1.SchemeGroupVersion, err)
	}

	controllerContext, err := common.NewControllerContext(configuration, mcClient)
	if err != nil {
		return nil, nil, err
	}

	mgr, err := controllerruntime.NewManager(configuration.RestConfig, manager.Options{
		// Disables serving built-in metrics.
		// We already start a standalone metrics server in parallel to the manager.
		MetricsBindAddress: "0",
		EventBroadcaster:   controllerContext.Broadcaster,
	})
	if err != nil {
		return nil, nil, err
	}

	err = v1alpha1.AddToScheme(mgr.GetScheme())
	if err != nil {
		return nil, nil, err
	}

	// Set the Kubernetes client to the one created by the manager.
	// In this way we can take advantage of the underlying caching
	// mechanism for reads instead of hitting the API directly.
	controllerContext.K8sClient = mgr.GetClient()

	compositeReconciler := composite.NewMetacontroller(*controllerContext, mcClient, configuration.Workers)
	compositeCtrl, err := controller.New("composite-metacontroller", mgr, controller.Options{
		Reconciler: compositeReconciler,
	})
	if err != nil {
		return nil, nil, err
	}
	err = compositeCtrl.Watch(&source.Kind{Type: &v1alpha1.CompositeController{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, nil, err
	}

	decoratorReconciler := decorator.NewMetacontroller(*controllerContext, configuration.Workers)
	decoratorCtrl, err := controller.New("decorator-metacontroller", mgr, controller.Options{
		Reconciler: decoratorReconciler,
	})
	if err != nil {
		return nil, nil, err
	}
	err = decoratorCtrl.Watch(&source.Kind{Type: &v1alpha1.DecoratorController{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, nil, err
	}

	// We need to call Start after initializing the controllers
	// to make sure all the needed informers are already created
	controllerContext.Start()

	stopFunc := func() {
		controllerContext.Stop()
	}

	return mgr, stopFunc, nil
}
