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
	"context"
	"fmt"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/syncServer"
	"time"

	"k8s.io/client-go/discovery"

	"metacontroller/pkg/controller/common"

	"metacontroller/pkg/controller/decorator"
	"metacontroller/pkg/options"

	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	"metacontroller/pkg/controller/composite"

	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"
)

// New returns a new controller manager and a function which can be used
// to release resources after the manager is stopped.
func New(configuration options.Configuration) (controllerruntime.Manager, error) {
	// Create informer factory for metacontroller API objects.
	mcClient, err := mcclientset.NewForConfig(configuration.RestConfig)
	if err != nil {
		return nil, fmt.Errorf("can't create client for api %s: %w", v1alpha1.SchemeGroupVersion, err)
	}

	// Check if metacontroller can successfully communicate with the K8s API server
	// If metacontroller is in a service mesh, this serves as a check that the sidecar is healthy
	err = k8sCommunicationCheck(mcClient.DiscoveryClient)
	if err != nil {
		return nil, err
	}

	controllerContext, err := common.NewControllerContext(configuration, mcClient)
	if err != nil {
		return nil, err
	}

	mgr, err := controllerruntime.NewManager(configuration.RestConfig, manager.Options{
		// Disables serving built-in metrics.
		// We already start a standalone metrics server in parallel to the manager.
		MetricsBindAddress:         configuration.MetricsEndpoint,
		EventBroadcaster:           controllerContext.Broadcaster,
		LeaderElection:             configuration.LeaderElectionOptions.LeaderElection,
		LeaderElectionResourceLock: configuration.LeaderElectionOptions.LeaderElectionResourceLock,
		LeaderElectionNamespace:    configuration.LeaderElectionOptions.LeaderElectionNamespace,
		LeaderElectionID:           configuration.LeaderElectionOptions.LeaderElectionID,
	})
	if err != nil {
		return nil, err
	}

	err = v1alpha1.AddToScheme(mgr.GetScheme())
	if err != nil {
		return nil, err
	}
	// crds api is in apiextensionsv1 package
	err = apiextensionsv1.AddToScheme(mgr.GetScheme())
	if err != nil {
		return nil, err
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
		return nil, err
	}
	err = compositeCtrl.Watch(&source.Kind{Type: &v1alpha1.CompositeController{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	decoratorReconciler := decorator.NewMetacontroller(*controllerContext, configuration.Workers)
	decoratorCtrl, err := controller.New("decorator-metacontroller", mgr, controller.Options{
		Reconciler: decoratorReconciler,
	})
	if err != nil {
		return nil, err
	}
	err = decoratorCtrl.Watch(&source.Kind{Type: &v1alpha1.DecoratorController{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return nil, err
	}

	// We need to call Start after initializing the controllers
	// to make sure all the needed informers are already created
	controllerContext.Start()

	// Trigger http server
	if configuration.Api {
		syncSrv := syncServer.New(compositeReconciler, decoratorReconciler, &configuration)
		syncSrv.Start()
	}

	return mgr, nil
}

func k8sCommunicationCheck(client *discovery.DiscoveryClient) (err error) {
	// retry 6 times with a delay of 5 seconds each retry
	// the retry and sleep intervals were observed anecdotally that service mesh sidecars
	// take up to ~20 seconds to allow requests to successfully communicate with the api server
	for range [6]int{} {
		_, err = client.RESTClient().Get().AbsPath("/api").DoRaw(context.TODO())
		if err == nil {
			logging.Logger.Info("Communication with K8s API server successful")
			break
		}

		logging.Logger.Info("Communication with K8s API server failed. Retrying...")
		time.Sleep(5 * time.Second)
	}
	return err
}
