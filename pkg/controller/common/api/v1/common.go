package v1

import (
	"fmt"
	"strings"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// RelativeObjectMap holds object related to given parent object.
// The structure is [GroupVersionKind] -> [common.relativeName] -> *unstructured.Unstructured
// where:
//
//	GroupVersionKind - identifies type stored in entry
//	relativeName() - return path to object in relation to parent
//	*unstructured.Unstructured - object to store
//
// i.e. :
// "v1/Pod -> 'prometheus/Prometheus'" (for namespaced child resource if the parent is cluster scope)
// "v1/Pod -> 'Prometheus'" (for namespaced child resource if parent is namespaced)
// "v1/Namespace -> 'some'" (for cluster-scope child resource if parent is cluster scope)
type RelativeObjectMap map[GroupVersionKind]map[string]*unstructured.Unstructured

// InitGroup initializes a map for given schema.GroupVersionKind if not yet initialized
func (m RelativeObjectMap) InitGroup(gvk schema.GroupVersionKind) {
	internalGvk := GroupVersionKind{gvk}
	if m[internalGvk] == nil {
		m[internalGvk] = make(map[string]*unstructured.Unstructured)
	}
}

// Insert inserts given obj to RelativeObjectMap regarding parent
func (m RelativeObjectMap) Insert(parent v1.Object, obj *unstructured.Unstructured) {
	internalGvk := GroupVersionKind{obj.GroupVersionKind()}
	if m[internalGvk] == nil {
		m.InitGroup(obj.GroupVersionKind())
	}
	group := m[internalGvk]
	name := relativeName(parent, obj)
	group[name] = obj
}

// InsertAll inserts given slice of objects to RelativeObjectMap regarding parent
func (m RelativeObjectMap) InsertAll(parent v1.Object, objects []*unstructured.Unstructured) {
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
// name is of the format <namespace>/<name>. Otherwise, the name of the object
// is returned.
func relativeName(parent v1.Object, obj *unstructured.Unstructured) string {
	if parent.GetNamespace() == "" && obj.GetNamespace() != "" {
		return fmt.Sprintf("%s/%s", obj.GetNamespace(), obj.GetName())
	}
	return obj.GetName()
}

// ReplaceObjectIfExists replaces the object with the same name & namespace as
// the given object with the contents of the given object. If no object exists
// in the existing map then no action is taken.
func (m RelativeObjectMap) ReplaceObjectIfExists(parent v1.Object, obj *unstructured.Unstructured) {
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
func MakeRelativeObjectMap(parent v1.Object, list []*unstructured.Unstructured) RelativeObjectMap {
	relativeObjects := make(RelativeObjectMap)

	relativeObjects.InsertAll(parent, list)

	return relativeObjects
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
