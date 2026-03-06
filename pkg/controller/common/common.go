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
	"metacontroller/pkg/options"
	"strings"
	"sync"

	v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"

	"metacontroller/pkg/events"

	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	mcinformers "metacontroller/pkg/client/generated/informer/externalversions"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/tools/cache"

	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"
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
	// K8sClient is a client used to interact with the Kubernetes API
	K8sClient         client.Client
	Resources         *dynamicdiscovery.ResourceMap
	DynClient         *dynamicclientset.Clientset
	DynInformers      *dynamicinformer.SharedInformerFactory
	McInformerFactory mcinformers.SharedInformerFactory
	McClient          mcclientset.Interface
	EventRecorder     record.EventRecorder
	Broadcaster       record.EventBroadcaster
	configuration     options.Configuration
}

// NewControllerContext creates a new ControllerContext using given Configuration and metacontroller client
func NewControllerContext(configuration options.Configuration, mcClient *mcclientset.Clientset) (*ControllerContext, error) {
	// Periodically refresh discovery to pick up newly-installed resources.
	dc := discovery.NewDiscoveryClientForConfigOrDie(configuration.RestConfig)
	resources := dynamicdiscovery.NewResourceMap(dc)

	mcInformerFactory := mcinformers.NewSharedInformerFactory(mcClient, configuration.InformerRelist)

	// Create dynamic clientset (factory for dynamic clients).
	dynClient, err := dynamicclientset.New(configuration.RestConfig, resources)
	if err != nil {
		return nil, err
	}
	// Create dynamic informer factory (for sharing dynamic informers).
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, configuration.InformerRelist)

	// Start metacontrollers (controllers that spawn controllers).
	// Each one requests the informers it needs from the factory.
	broadcaster, err := events.NewBroadcaster(configuration.RestConfig, configuration.CorrelatorOptions)
	if err != nil {
		return nil, err
	}
	recorder := broadcaster.NewRecorder(scheme, corev1.EventSource{Component: "metacontroller"})

	return &ControllerContext{
		Resources:         resources,
		DynClient:         dynClient,
		DynInformers:      dynInformers,
		McInformerFactory: mcInformerFactory,
		EventRecorder:     recorder,
		Broadcaster:       broadcaster,
		configuration:     configuration,
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

// SyncMap is a generic wrapper around sync.Map that provides type safety.
type SyncMap[K comparable, V any] struct {
	m sync.Map
}

// NewSyncMap returns a new SyncMap.
func NewSyncMap[K comparable, V any]() *SyncMap[K, V] {
	return &SyncMap[K, V]{}
}

// Load returns the value stored in the map for a key, or nil if no value is present.
// The ok result indicates whether value was found in the map.
func (m *SyncMap[K, V]) Load(key K) (V, bool) {
	v, ok := m.m.Load(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *SyncMap[K, V]) LoadOrStore(key K, val V) (actual V, loaded bool) {
	v, ok := m.m.LoadOrStore(key, val)
	return v.(V), ok
}

// LoadAndDelete deletes the value for a key, returning the previous value if any.
// The loaded result reports whether the key was present.
func (m *SyncMap[K, V]) LoadAndDelete(key K) (value V, loaded bool) {
	v, ok := m.m.LoadAndDelete(key)
	if !ok {
		var zero V
		return zero, false
	}
	return v.(V), true
}

// Store sets the value for a key.
func (m *SyncMap[K, V]) Store(key K, val V) {
	m.m.Store(key, val)
}

// Delete deletes the value for a key.
func (m *SyncMap[K, V]) Delete(key K) {
	m.m.Delete(key)
}

// Range calls f sequentially for each key and value present in the map.
// If f returns false, range stops the iteration.
func (m *SyncMap[K, V]) Range(f func(key K, value V) bool) {
	m.m.Range(func(key, value interface{}) bool {
		return f(key.(K), value.(V))
	})
}

func NewGroupKindMap() *GroupKindMap {
	return &GroupKindMap{
		m: *NewSyncMap[schema.GroupKind, *dynamicdiscovery.APIResource](),
	}
}

type GroupKindMap struct {
	m SyncMap[schema.GroupKind, *dynamicdiscovery.APIResource]
}

func (m *GroupKindMap) Set(gk schema.GroupKind, resource *dynamicdiscovery.APIResource) {
	if m == nil {
		return
	}
	m.m.Store(gk, resource)
}

func (m *GroupKindMap) Get(gk schema.GroupKind) *dynamicdiscovery.APIResource {
	if m == nil {
		return nil
	}
	val, ok := m.m.Load(gk)
	if !ok {
		return nil
	}
	return val
}

func (m *GroupKindMap) Len() int {
	if m == nil {
		return 0
	}
	length := 0
	m.m.Range(func(_ schema.GroupKind, _ *dynamicdiscovery.APIResource) bool {
		length++
		return true
	})
	return length
}

func (m *GroupKindMap) Range(f func(gk schema.GroupKind, resource *dynamicdiscovery.APIResource)) {
	if m == nil {
		return
	}
	m.m.Range(func(key schema.GroupKind, value *dynamicdiscovery.APIResource) bool {
		f(key, value)
		return true
	})
}

func NewInformerMap() *InformerMap {
	return &InformerMap{
		m: *NewSyncMap[schema.GroupVersionResource, *dynamicinformer.ResourceInformer](),
	}
}

type InformerMap struct {
	m SyncMap[schema.GroupVersionResource, *dynamicinformer.ResourceInformer]
}

func (m *InformerMap) Set(gvr schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer) {
	if m == nil {
		return
	}
	m.m.Store(gvr, informer)
}

func (m *InformerMap) Get(gvr schema.GroupVersionResource) *dynamicinformer.ResourceInformer {
	if m == nil {
		return nil
	}
	val, ok := m.m.Load(gvr)
	if !ok {
		return nil
	}
	return val
}

func (m *InformerMap) GetOrCreate(gvr schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer) (*dynamicinformer.ResourceInformer, bool) {
	if m == nil {
		return nil, false
	}
	return m.m.LoadOrStore(gvr, informer)
}

func (m *InformerMap) Len() int {
	if m == nil {
		return 0
	}
	length := 0
	m.m.Range(func(_ schema.GroupVersionResource, _ *dynamicinformer.ResourceInformer) bool {
		length++
		return true
	})
	return length
}

func (m *InformerMap) Range(f func(gvr schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer)) {
	if m == nil {
		return
	}
	m.m.Range(func(key schema.GroupVersionResource, value *dynamicinformer.ResourceInformer) bool {
		f(key, value)
		return true
	})
}

// GetObject return object via Lister from given informer, namespaced or not.
func GetObject(informer *dynamicinformer.ResourceInformer, namespace, name string) (*unstructured.Unstructured, error) {
	if namespace == "" {
		return informer.Lister().Get(name)
	}
	return informer.Lister().Namespace(namespace).Get(name)
}

func HasStatusSubresource(crd *v1.CustomResourceDefinition, version string) bool {
	for _, crdVersion := range crd.Spec.Versions {
		if crdVersion.Name == version {
			// check subresource for matching version
			if crdVersion.Subresources != nil && crdVersion.Subresources.Status != nil {
				return true
			}
		}
	}
	return false
}

type ApplyOptions struct {
	FieldManager string
	Strategy     ApplyStrategy
}

type ApplyStrategy string

const (
	ApplyStrategyServerSideApply ApplyStrategy = "server-side-apply"
	ApplyStrategyDynamicApply    ApplyStrategy = "dynamic-apply"
)
