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

package controllerref

import (
	"fmt"

	"github.com/golang/glog"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/pointer"

	"metacontroller.app/apis/metacontroller/v1alpha1"
	client "metacontroller.app/client/generated/clientset/internalclientset/typed/metacontroller/v1alpha1"
	k8s "metacontroller.app/third_party/kubernetes"
)

type ControllerRevisionManager struct {
	k8s.BaseControllerRefManager
	parentKind schema.GroupVersionKind
	client     client.ControllerRevisionInterface
}

func NewControllerRevisionManager(client client.ControllerRevisionInterface, parent metav1.Object, selector labels.Selector, parentKind schema.GroupVersionKind, canAdopt func() error) *ControllerRevisionManager {
	return &ControllerRevisionManager{
		BaseControllerRefManager: k8s.BaseControllerRefManager{
			Controller:   parent,
			Selector:     selector,
			CanAdoptFunc: canAdopt,
		},
		parentKind: parentKind,
		client:     client,
	}
}

func (m *ControllerRevisionManager) ClaimControllerRevisions(children []*v1alpha1.ControllerRevision) ([]*v1alpha1.ControllerRevision, error) {
	var claimed []*v1alpha1.ControllerRevision
	var errlist []error

	match := func(obj metav1.Object) bool {
		return m.Selector.Matches(labels.Set(obj.GetLabels()))
	}
	adopt := func(obj metav1.Object) error {
		return m.adoptControllerRevision(obj.(*v1alpha1.ControllerRevision))
	}
	release := func(obj metav1.Object) error {
		return m.releaseControllerRevision(obj.(*v1alpha1.ControllerRevision))
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

func (m *ControllerRevisionManager) adoptControllerRevision(obj *v1alpha1.ControllerRevision) error {
	if err := m.CanAdopt(); err != nil {
		return fmt.Errorf("can't adopt ControllerRevision %v/%v (%v): %v", obj.GetNamespace(), obj.GetName(), obj.GetUID(), err)
	}
	glog.Infof("%v %v/%v: adopting ControllerRevision %v", m.parentKind.Kind, m.Controller.GetNamespace(), m.Controller.GetName(), obj.GetName())
	controllerRef := metav1.OwnerReference{
		APIVersion:         m.parentKind.GroupVersion().String(),
		Kind:               m.parentKind.Kind,
		Name:               m.Controller.GetName(),
		UID:                m.Controller.GetUID(),
		Controller:         pointer.BoolPtr(true),
		BlockOwnerDeletion: pointer.BoolPtr(true),
	}

	// We can't use strategic merge patch because we want this to work with custom resources.
	// We can't use merge patch because that would replace the whole list.
	// We can't use JSON patch ops because that wouldn't be idempotent.
	// The only option is GET/PUT with ResourceVersion.
	_, err := m.client.UpdateWithRetries(obj, func(obj *v1alpha1.ControllerRevision) bool {
		ownerRefs := addOwnerReference(obj.GetOwnerReferences(), controllerRef)
		obj.SetOwnerReferences(ownerRefs)
		return true
	})
	return err
}

func (m *ControllerRevisionManager) releaseControllerRevision(obj *v1alpha1.ControllerRevision) error {
	glog.Infof("%v %v/%v: releasing ControllerRevision %v", m.parentKind.Kind, m.Controller.GetNamespace(), m.Controller.GetName(), obj.GetName())
	_, err := m.client.UpdateWithRetries(obj, func(obj *v1alpha1.ControllerRevision) bool {
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
