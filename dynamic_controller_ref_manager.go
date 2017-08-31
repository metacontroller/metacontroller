package main

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
)

type dynamicControllerRefManager struct {
	BaseControllerRefManager
	parentKind schema.GroupVersionKind
	childKind  schema.GroupVersionKind
	client     *dynamicResourceClient
}

func newDynamicControllerRefManager(client *dynamicResourceClient, parent metav1.Object, selector labels.Selector, parentKind, childKind schema.GroupVersionKind, canAdopt func() error) *dynamicControllerRefManager {
	return &dynamicControllerRefManager{
		BaseControllerRefManager: BaseControllerRefManager{
			Controller:   parent,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		parentKind: parentKind,
		childKind:  childKind,
		client:     client,
	}
}

func (m *dynamicControllerRefManager) claimChildren(children []unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
	var claimed []*unstructured.Unstructured
	var errlist []error

	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.adoptChild(obj.(*unstructured.Unstructured))
	}
	release := func(obj metav1.Object) error {
		return m.releaseChild(obj.(*unstructured.Unstructured))
	}

	for i := range children {
		child := &children[i]
		ok, err := m.ClaimObject(child, match, adopt, release)
		if err != nil {
			errlist = append(errlist, err)
			continue
		}
		if ok {
			claimed = append(claimed, child)
		}
	}
	return claimed, utilerrors.NewAggregate(errlist)
}

func (m *dynamicControllerRefManager) adoptChild(obj *unstructured.Unstructured) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt %v %v/%v (%v): %v", m.childKind.Kind, obj.GetNamespace(), obj.GetName(), obj.GetUID(), err)
	}
	parentUID := string(m.Controller.GetUID())
	controllerRef := map[string]interface{}{
		"apiVersion":         m.parentKind.GroupVersion().String(),
		"kind":               m.parentKind.Kind,
		"name":               m.Controller.GetName(),
		"uid":                parentUID,
		"controller":         true,
		"blockOwnerDeletion": true,
	}
	return updateOwnerReferences(m.client, obj, func(ownerRefs []interface{}) ([]interface{}, bool) {
		// Check if we're in the list.
		for _, ref := range ownerRefs {
			ownerRef := ref.(map[string]interface{})
			if getNestedString(ownerRef, "uid") == parentUID {
				// We already own this. Update other fields as needed.
				changed := false
				for k, v := range controllerRef {
					if ownerRef[k] != v {
						ownerRef[k] = v
						changed = true
					}
				}
				return ownerRefs, changed
			}
		}
		// Add ourselves to the list.
		// Note that server-side validation is responsible for ensuring only one ControllerRef.
		return append(ownerRefs, controllerRef), true
	})
}

func (m *dynamicControllerRefManager) releaseChild(obj *unstructured.Unstructured) error {
	parentUID := string(m.Controller.GetUID())
	err := updateOwnerReferences(m.client, obj, func(ownerRefs []interface{}) ([]interface{}, bool) {
		// Remove ourselves from the list.
		for i, ref := range ownerRefs {
			ownerRef := ref.(map[string]interface{})
			if getNestedString(ownerRef, "uid") == parentUID {
				return append(ownerRefs[:i], ownerRefs[i+1:]...), true
			}
		}
		// We're not listed. Nothing to do.
		return ownerRefs, false
	})
	if errors.IsNotFound(err) || isUIDError(err) {
		// If the original object is gone, that's fine because we're giving up on this child anyway.
		return nil
	}
	return err
}

func updateOwnerReferences(client *dynamicResourceClient, orig *unstructured.Unstructured, update func(ownerReferences []interface{}) ([]interface{}, bool)) error {
	// We can't use strategic merge patch because we want this to work with custom resources.
	// We can't use merge patch because that would replace the whole list.
	// We can't use JSON patch ops because that wouldn't be idempotent.
	return client.UpdateWithRetries(orig, func(obj *unstructured.Unstructured) bool {
		ownerRefs, ok := getNestedField(obj.UnstructuredContent(), "metadata", "ownerReferences").([]interface{})
		if !ok {
			// Nothing there. Start a list from scratch.
			ownerRefs = nil
		}
		ownerRefs, changed := update(ownerRefs)
		if !changed {
			// There's nothing to do.
			return false
		}
		setNestedField(obj.UnstructuredContent(), ownerRefs, "metadata", "ownerReferences")
		return true
	})
}

type uidError string

func (e uidError) Error() string {
	return string(e)
}

func newUIDError(format string, args ...interface{}) error {
	return uidError(fmt.Sprintf(format, args...))
}

func isUIDError(err error) bool {
	_, ok := err.(uidError)
	return ok
}
