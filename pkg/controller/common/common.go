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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/options"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"

	"metacontroller/pkg/events"

	mcclientset "metacontroller/pkg/client/generated/clientset/internalclientset"
	mcinformers "metacontroller/pkg/client/generated/informer/externalversions"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"

	"k8s.io/client-go/tools/record"

	"k8s.io/apimachinery/pkg/runtime/schema"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// GroupVersionKind is metacontroller wrapper around schema.GroupVersionKind
// implementing encoding.TextMarshaler and encoding.TextUnmarshaler
type GroupVersionKind struct {
	schema.GroupVersionKind
}

// MarshalText is implementation of  encoding.TextMarshaler
func (gvk GroupVersionKind) MarshalText() ([]byte, error) {
	var marshalledText string
	if gvk.Group == "" {
		marshalledText = fmt.Sprintf("%s.%s", gvk.Kind, gvk.Version)
	} else {
		marshalledText = fmt.Sprintf("%s.%s/%s", gvk.Kind, gvk.Group, gvk.Version)
	}
	return []byte(marshalledText), nil
}

// UnmarshalText is implementation of encoding.TextUnmarshaler
func (gvk *GroupVersionKind) UnmarshalText(text []byte) error {
	kindGroupVersionString := string(text)
	parts := strings.SplitN(kindGroupVersionString, ".", 2)
	if len(parts) < 2 {
		return fmt.Errorf("could not unmarshall [%s], expected string in 'kind.group/version' format", string(text))
	}
	groupVersion, err := schema.ParseGroupVersion(parts[1])
	if err != nil {
		return err
	}
	*gvk = GroupVersionKind{
		groupVersion.WithKind(parts[0]),
	}
	return nil
}

// RelativeObjectMap holds object related to given parent object.
// The structure is [GroupVersionKind] -> [common.relativeName] -> *unstructured.Unstructured
// where:
//   GroupVersionKind - identifies type stored in entry
//   relativeName() - return path to object in relation to parent
//   *unstructured.Unstructured - object to store
type RelativeObjectMap map[GroupVersionKind]map[string]*unstructured.Unstructured

// InitGroup initializes a map for given schema.GroupVersionKind if not yet initialized
func (m RelativeObjectMap) InitGroup(gvk schema.GroupVersionKind) {
	internalGvk := GroupVersionKind{gvk}
	if m[internalGvk] == nil {
		m[internalGvk] = make(map[string]*unstructured.Unstructured)
	}
}

// Insert inserts given obj to RelativeObjectMap regarding parent
func (m RelativeObjectMap) Insert(parent metav1.Object, obj *unstructured.Unstructured) {
	internalGvk := GroupVersionKind{obj.GroupVersionKind()}
	if m[internalGvk] == nil {
		m.InitGroup(obj.GroupVersionKind())
	}
	group := m[internalGvk]
	name := relativeName(parent, obj)
	group[name] = obj
}

// InsertAll inserts given slice of objects to RelativeObjectMap regarding parent
func (m RelativeObjectMap) InsertAll(parent metav1.Object, objects []*unstructured.Unstructured) {
	for _, object := range objects {
		m.Insert(parent, object)
	}
}

// FindGroupKindName search object by name in each schema.GroupKind (ignoring version part)
func (m RelativeObjectMap) FindGroupKindName(gk schema.GroupKind, name string) *unstructured.Unstructured {
	// The map is keyed by group-version-kind, but we don't know the version.
	// So, check inside any GVK that matches the group and kind, ignoring version.
	for key, objects := range m {
		if key.GroupKind() == gk {
			for n, object := range objects {
				if n == name {
					return object
				}
			}
		}
	}
	return nil
}

// relativeName returns the name of the object relative to the parent.
// If the parent is cluster scoped and the object namespaced scoped the
// name is of the format <namespace>/<name>. Otherwise the name of the object
// is returned.
func relativeName(parent metav1.Object, obj *unstructured.Unstructured) string {
	if parent.GetNamespace() == "" && obj.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	}
	return obj.GetName()
}

// describeObject returns a human-readable string to identify a given object.
func describeObject(obj *unstructured.Unstructured) string {
	if ns := obj.GetNamespace(); ns != "" {
		return fmt.Sprintf("%s %s/%s", obj.GetKind(), ns, obj.GetName())
	}
	return fmt.Sprintf("%s %s", obj.GetKind(), obj.GetName())
}

// ReplaceObject replaces the object with the same name & namespace as
// the given object with the contents of the given object. If no object exists
// in the existing map then no action is taken.
func (m RelativeObjectMap) ReplaceObject(parent metav1.Object, obj *unstructured.Unstructured) {
	internalGvk := GroupVersionKind{obj.GroupVersionKind()}
	objects := m[internalGvk]
	if objects == nil {
		// We only want to replace if it already exists, so do nothing.
		return
	}
	name := relativeName(parent, obj)
	if _, found := objects[name]; found {
		objects[name] = obj
	}
}

// List expands the RelativeObjectMap into a flat list of relative objects, in random order.
func (m RelativeObjectMap) List() []*unstructured.Unstructured {
	var list []*unstructured.Unstructured
	for _, group := range m {
		for _, obj := range group {
			list = append(list, obj)
		}
	}
	return list
}

// MakeRelativeObjectMap builds the map of objects resources that is suitable for use
// in the `children` field of a CompositeController SyncRequest or
// `attachments` field of  the  DecoratorControllers SyncRequest or `customize` field of
// customize hook
//
// This function returns a RelativeObjectMap which is a map of maps. The outer most map
// is keyed  using the object's type and the inner map is keyed using the
// object's name. If the parent resource is clustered and the object resource
// is namespaced the inner map's keys are prefixed by the namespace of the
// object resource.
//
// This function requires parent resources has the meta.Namespace accurately
// set. If the namespace of the parent is empty it's considered a clustered
// resource.
//
// If a user returns a namespaced as a object of a clustered resources without
// the namespace set this is considered a user error but it's not handled here
// since the api errorstring to create the object is clear.
func MakeRelativeObjectMap(parent metav1.Object, list []*unstructured.Unstructured) RelativeObjectMap {
	relativeObjects := make(RelativeObjectMap)

	relativeObjects.InsertAll(parent, list)

	return relativeObjects
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

type InformerMap map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer

func (m InformerMap) Set(gvr schema.GroupVersionResource, informer *dynamicinformer.ResourceInformer) {
	m[gvr] = informer
}

func (m InformerMap) Get(gvr schema.GroupVersionResource) *dynamicinformer.ResourceInformer {
	return m[gvr]
}

// GetObject return object via Lister from given informer, namespaced or not.
func GetObject(informer *dynamicinformer.ResourceInformer, namespace, name string) (*unstructured.Unstructured, error) {
	if namespace == "" {
		return informer.Lister().Get(name)
	}
	return informer.Lister().Namespace(namespace).Get(name)
}
