/*
Copyright 2017 Google Inc.

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

package controllerref

import (
	"fmt"

	"k8s.io/utils/pointer"

	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"

	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	k8s "metacontroller/pkg/third_party/kubernetes"
)

type UnstructuredManager struct {
	k8s.BaseControllerRefManager
	parentKind schema.GroupVersionKind
	childKind  schema.GroupVersionKind
	client     *dynamicclientset.ResourceClient
}

func NewUnstructuredManager(client *dynamicclientset.ResourceClient, parent metav1.Object, selector labels.Selector, parentKind, childKind schema.GroupVersionKind, canAdopt func() error) *UnstructuredManager {
	return &UnstructuredManager{
		BaseControllerRefManager: k8s.BaseControllerRefManager{
			Controller:   parent,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		parentKind: parentKind,
		childKind:  childKind,
		client:     client,
	}
}

func (m *UnstructuredManager) ClaimChildren(children []*unstructured.Unstructured) ([]*unstructured.Unstructured, error) {
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

	for _, child := range children {
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

func atomicUpdate(rc *dynamicclientset.ResourceClient, obj *unstructured.Unstructured, updateFunc func(obj *unstructured.Unstructured) bool) error {
	// We can't use strategic merge patch because we want this to work with custom resources.
	// We can't use merge patch because that would replace the whole list.
	// We can't use JSON patch ops because that wouldn't be idempotent.
	// The only option is GET/PUT with ResourceVersion.
	_, err := rc.Namespace(obj.GetNamespace()).AtomicUpdate(obj, updateFunc)
	return err
}

func (m *UnstructuredManager) adoptChild(obj *unstructured.Unstructured) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt %v %v/%v (%v): %v", m.childKind.Kind, obj.GetNamespace(), obj.GetName(), obj.GetUID(), err)
	}
	klog.InfoS("Adopting", "parent_kind", m.parentKind.Kind, "controller", klog.KObj(m.Controller), "child_kind", m.childKind.Kind, "object", klog.KObj(obj))
	controllerRef := metav1.OwnerReference{
		APIVersion:         m.parentKind.GroupVersion().String(),
		Kind:               m.parentKind.Kind,
		Name:               m.Controller.GetName(),
		UID:                m.Controller.GetUID(),
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}
	return atomicUpdate(m.client, obj, func(obj *unstructured.Unstructured) bool {
		ownerRefs := addOwnerReference(obj.GetOwnerReferences(), controllerRef)
		obj.SetOwnerReferences(ownerRefs)
		return true
	})
}

func (m *UnstructuredManager) releaseChild(obj *unstructured.Unstructured) error {
	klog.InfoS("Releasing", "parent_kind", m.parentKind.Kind, "controller", klog.KObj(m.Controller), "child_kind", m.childKind.Kind, "object", klog.KObj(obj))
	err := atomicUpdate(m.client, obj, func(obj *unstructured.Unstructured) bool {
		ownerRefs := removeOwnerReference(obj.GetOwnerReferences(), m.Controller.GetUID())
		obj.SetOwnerReferences(ownerRefs)
		return true
	})
	if apierrors.IsNotFound(err) || apierrors.IsGone(err) {
		// If the original object is gone, that's fine because we're giving up on this child anyway.
		return nil
	}
	return err
}
