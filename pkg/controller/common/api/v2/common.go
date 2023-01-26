package v2

import (
	"fmt"
	"strings"

	commonv1 "metacontroller/pkg/controller/common/api/v1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

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

// UniformObjectMap holds objects grouped by GroupVersionKind and uniform location in the form namespace/name (if object is clustered scope, just name)
// it holds entries in form [GroupVersionKind] -> [namespace/name] -> object
// where namespace is empty in case of resources which are cluster scoped,
// i.e. :
// "v1/Pod -> 'some-namespace/some-name'" (for namespaced child resource)
// "v1/Namespace -> 'some-name'" (for cluster-scope child resource)
type UniformObjectMap map[GroupVersionKind]map[string]*unstructured.Unstructured

// MakeUniformObjectMap builds the map of objects resources that is suitable for use
// in the `children` field of a CompositeController SyncRequest or
// `attachments` field of  the  DecoratorControllers SyncRequest or `customize` field of
// customize hook
//
// This function returns a UniformObjectMap which is a map of maps. The outer most map
// is keyed  using the object's type and the inner map is keyed using the
// object's namespace/name. If the object is clustered scope it it just its name
func MakeUniformObjectMap(parent v1.Object, list []*unstructured.Unstructured) UniformObjectMap {
	objectMap := make(UniformObjectMap)

	objectMap.InsertAll(parent, list)

	return objectMap
}

// InitGroup initializes a map for given schema.GroupVersionKind if not yet initialized
func (m UniformObjectMap) InitGroup(gvk schema.GroupVersionKind) {
	internalGvk := GroupVersionKind{gvk}
	if m[internalGvk] == nil {
		m[internalGvk] = make(map[string]*unstructured.Unstructured)
	}
}

// Insert inserts given obj to UniformObjectMap
func (m UniformObjectMap) Insert(parent v1.Object, obj *unstructured.Unstructured) {
	internalGvk := GroupVersionKind{obj.GroupVersionKind()}
	if m[internalGvk] == nil {
		m.InitGroup(obj.GroupVersionKind())
	}
	group := m[internalGvk]
	name := m.qualifiedName(obj)
	group[name] = obj
}

// InsertAll inserts given slice of objects to UniformObjectMap
func (m UniformObjectMap) InsertAll(parent v1.Object, objects []*unstructured.Unstructured) {
	for _, object := range objects {
		m.Insert(parent, object)
	}
}

// qualifiedName returns the namespace/name of the object. If obj is clustered scope,
// return just name.
func (m UniformObjectMap) qualifiedName(obj *unstructured.Unstructured) string {
	if obj.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	}
	return obj.GetName()
}

// FindGroupKindName search object by name in each schema.GroupKind (ignoring version part)
func (m UniformObjectMap) FindGroupKindName(gk schema.GroupKind, name string) *unstructured.Unstructured {
	// The map is keyed by group-version-kind, but we don't know the version.
	// So, check inside any GVK that matches the group and kind, ignoring version.
	for key, objects := range m {
		if key.GroupKind() == gk {
			object, found := objects[name]
			if found {
				return object
			}
		}
	}
	return nil
}

// ReplaceObjectIfExists replaces the object with the same name & namespace as
// the given object with the contents of the given object. If no object exists
// in the existing map then no action is taken.
func (m UniformObjectMap) ReplaceObjectIfExists(parent v1.Object, obj *unstructured.Unstructured) {
	internalGvk := GroupVersionKind{obj.GroupVersionKind()}
	objects, found := m[internalGvk]
	if !found || len(objects) == 0 {
		// We only want to replace if it already exists, so do nothing.
		return
	}
	name := m.qualifiedName(obj)
	if _, found := objects[name]; found {
		objects[name] = obj
	}
}

// List expands the UniformObjectMap into a flat list of relative objects, in random order.
func (m UniformObjectMap) List() []*unstructured.Unstructured {
	var list []*unstructured.Unstructured
	for _, group := range m {
		for _, obj := range group {
			list = append(list, obj)
		}
	}
	return list
}

// Convert returns commonv1.RelativeObjectMap against given parent, removing non matching objects
func (m UniformObjectMap) Convert(parent *unstructured.Unstructured) commonv1.RelativeObjectMap {
	potentialChildren := m.List()
	relativeObjects := make(commonv1.RelativeObjectMap)
	parentIsClusterScope := parent.GetNamespace() == ""
	if parentIsClusterScope {
		// we can safely add all objects
		relativeObjects.InsertAll(parent, potentialChildren)
		return relativeObjects
	}
	// parent is namespace scope, we need filter out cluster-scope objects and objects from different namespace
	for _, child := range potentialChildren {
		if parent.GetNamespace() == child.GetNamespace() {
			relativeObjects.Insert(parent, child)
		}
	}
	return relativeObjects
}
