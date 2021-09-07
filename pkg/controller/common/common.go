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

package common

import (
	"fmt"
	"metacontroller/pkg/logging"
	"strings"

	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/restmapper"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/options"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"

	"k8s.io/client-go/dynamic/dynamicinformer"

	"metacontroller/pkg/events"

	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	mcinformers "metacontroller/pkg/client/generated/informer/externalversions"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
)

var (
	KeyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
	scheme  = runtime.NewScheme()
)

func init() {
	if err := v1alpha1.AddToScheme(scheme); err != nil {
		logging.Logger.Error(err, "failed adding v1alpha1 to scheme")
	}
}

type HookType string
type ControllerType string

const (
	FinalizeHook        HookType       = "finalize"
	CustomizeHook       HookType       = "customize"
	SyncHook            HookType       = "sync"
	CompositeController ControllerType = "CompositeController"
	DecoratorController ControllerType = "DecoratorController"
)

func (h HookType) String() string {
	return string(h)
}

func (c ControllerType) String() string {
	return string(c)
}

// ControllerContext holds various object related to interacting with kubernetes cluster
type ControllerContext struct {
	Broadcaster   record.EventBroadcaster
	configuration options.Configuration
	DynClient     *dynamicclientset.Clientset
	DynInformers  dynamicinformer.DynamicSharedInformerFactory
	EventRecorder record.EventRecorder
	// K8sClient is a client used to interact with the Kubernetes API
	K8sClient         client.Client
	McInformerFactory mcinformers.SharedInformerFactory
	McClient          mcclientset.Interface
	Resources         *dynamicdiscovery.ResourceMap
	RESTMapper        *restmapper.DeferredDiscoveryRESTMapper
}

// NewControllerContext creates a new ControllerContext using given Configuration and metacontroller client
func NewControllerContext(configuration options.Configuration, mcClient *mcclientset.Clientset) (*ControllerContext, error) {
	// Periodically refresh discovery to pick up newly-installed resources.
	discoveryClient := discovery.NewDiscoveryClientForConfigOrDie(configuration.RestConfig)
	resources := dynamicdiscovery.NewResourceMap(discoveryClient)
	cacheClient := memory.NewMemCacheClient(discoveryClient)
	restMapper := restmapper.NewDeferredDiscoveryRESTMapper(cacheClient)

	mcInformerFactory := mcinformers.NewSharedInformerFactory(mcClient, configuration.InformerRelist)

	// Create dynamic clientset (factory for dynamic clients).
	dynClient, err := dynamicclientset.New(configuration.RestConfig, resources)
	if err != nil {
		return nil, err
	}
	dynamicClient := dynamic.NewForConfigOrDie(configuration.RestConfig)
	dynamicSharedInformerFactory := dynamicinformer.NewDynamicSharedInformerFactory(dynamicClient, configuration.InformerRelist)

	// Start metacontrollers (controllers that spawn controllers).
	// Each one requests the informers it needs from the factory.
	broadcaster, err := events.NewBroadcaster(configuration.RestConfig, configuration.CorrelatorOptions)
	if err != nil {
		return nil, err
	}
	recorder := broadcaster.NewRecorder(scheme, corev1.EventSource{Component: "metacontroller"})

	return &ControllerContext{
		Broadcaster:       broadcaster,
		configuration:     configuration,
		DynClient:         dynClient,
		DynInformers:      dynamicSharedInformerFactory,
		EventRecorder:     recorder,
		McInformerFactory: mcInformerFactory,
		Resources:         resources,
		RESTMapper:        restMapper,
	}, nil
}

// Start starts all informers created up to that point.
// Informers created after Start is called will not be automatically started
func (controllerContext ControllerContext) Start() {
	// We don't care about stopping this cleanly since it has no external effects.
	controllerContext.Resources.Start(controllerContext.configuration.DiscoveryInterval)
	// Start all requested informers.
	// We don't care about stopping this cleanly since it has no external effects.
	controllerContext.McInformerFactory.Start(nil)
}

// describeObject returns a human-readable string to identify a given object.
func describeObject(obj *unstructured.Unstructured) string {
	if ns := obj.GetNamespace(); ns != "" {
		return fmt.Sprintf("%s %s/%s", obj.GetKind(), ns, obj.GetName())
	}
	return fmt.Sprintf("%s %s", obj.GetKind(), obj.GetName())
}

func ParseAPIVersion(apiVersion string) (group, version string) {
	parts := strings.SplitN(apiVersion, "/", 2)
	if len(parts) == 1 {
		// It's a core version.
		return "", parts[0]
	}
	return parts[0], parts[1]
}

type GroupKindMap map[schema.GroupKind]*dynamicdiscovery.APIResource

func (m GroupKindMap) Set(gk schema.GroupKind, resource *dynamicdiscovery.APIResource) {
	m[gk] = resource
}

func (m GroupKindMap) Get(gk schema.GroupKind) *dynamicdiscovery.APIResource {
	return m[gk]
}

type InformerMap map[schema.GroupVersionResource]informers.GenericInformer

func (m InformerMap) Set(gvr schema.GroupVersionResource, informer informers.GenericInformer) {
	m[gvr] = informer
}

func (m InformerMap) Get(gvr schema.GroupVersionResource) informers.GenericInformer {
	return m[gvr]
}

// GetObject return object via Lister from given informer, namespaced or not.
func GetObject(inf informers.GenericInformer, namespace, name string) (runtime.Object, error) {
	if namespace == "" {
		return inf.Lister().Get(name)
	}
	return inf.Lister().ByNamespace(namespace).Get(name)
}

func HasStatusSubresource(crd *v1.CustomResourceDefinition, version string) bool {
	for _, crdVersion := range crd.Spec.Versions {
		if crdVersion.Name == version {
			// check subresource for matching verison
			if crdVersion.Subresources != nil && crdVersion.Subresources.Status != nil {
				return true
			}
		}
	}
	return false
}
