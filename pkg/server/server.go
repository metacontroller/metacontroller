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
	"time"

	"sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/source"

	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/discovery"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/decorator"
	"metacontroller/pkg/logging"
	"metacontroller/pkg/options"

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
		Metrics:                    server.Options{BindAddress: configuration.MetricsEndpoint},
		HealthProbeBindAddress:     configuration.HealthProbeBindAddress,
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

	// If configuration has a TargetLabelSelector, then read the values
	// and create the labels.Selector object.
	targetLabelSelector := labels.Everything()
	logging.Logger.Info("Initializing target label selector", "label_selector", configuration.TargetLabelSelector)
	if configuration.TargetLabelSelector != "" {
		targetLabelSelector, err = labels.Parse(configuration.TargetLabelSelector)
		if err != nil {
			return nil, err
		}
	}

	// Filter function using the labelSelector to match against the objects.
	byLabelSelectorFilter := func(labelSelector labels.Selector) func(object client.Object) bool {
		return func(object client.Object) bool {
			return labelSelector.Matches(labels.Set(object.GetLabels()))
		}
	}

	// Predicate filter to apply the byLabelSelectorFilter using our targetLabelSelector.
	predicateFuncs := predicate.NewTypedPredicateFuncs[client.Object](byLabelSelectorFilter(targetLabelSelector))

	// Set the Kubernetes client to the one created by the manager.
	// In this way we can take advantage of the underlying caching
	// mechanism for reads instead of hitting the API directly.
	controllerContext.K8sClient = mgr.GetClient()

	var strategy common.ApplyStrategy
	switch configuration.ApplyStrategy {
	case "":
		fallthrough
	case "dynamic-apply": // default
		strategy = common.ApplyStrategyDynamicApply
	case "server-side-apply":
		strategy = common.ApplyStrategyServerSideApply
	default:
		return nil, fmt.Errorf("unknown apply strategy: %s", configuration.ApplyStrategy)
	}

	compositeReconciler := composite.NewMetacontroller(*controllerContext, mcClient, configuration.Workers, &common.ApplyOptions{
		FieldManager: configuration.SsaFieldManager,
		Strategy:     strategy,
	})
	compositeCtrl, err := controller.New("composite-metacontroller", mgr, controller.Options{
		Reconciler: compositeReconciler,
	})
	if err != nil {
		return nil, err
	}

	err = compositeCtrl.Watch(source.Kind[client.Object](mgr.GetCache(), &v1alpha1.CompositeController{}, &handler.TypedEnqueueRequestForObject[client.Object]{}, predicateFuncs))
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
	err = decoratorCtrl.Watch(source.Kind[client.Object](mgr.GetCache(), &v1alpha1.DecoratorController{}, &handler.TypedEnqueueRequestForObject[client.Object]{}, predicateFuncs))
	if err != nil {
		return nil, err
	}

	// We need to call Start after initializing the controllers
	// to make sure all the needed informers are already created
	controllerContext.Start()

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
