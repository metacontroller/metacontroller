/*
Copyright 2026 Metacontroller authors.

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

package framework

import (
	"context"
	"encoding/json"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

// SpecChangeCase is the input for the test of the parent finalizer cleanup
// after a change of the parent resources. The composite and the decorator
// suites use the same test.
type SpecChangeCase struct {
	// NewController returns the controller object with the given parent
	// resource. Its sync hook and finalize hook must call hookURL.
	NewController func(hookURL string, parent v1alpha1.ResourceRule) client.Object
	// SetParent replaces the parent resource of the controller object.
	SetParent func(controller client.Object, parent v1alpha1.ResourceRule)
	// ParentFinalizer is the finalizer that the controller adds to its parents.
	ParentFinalizer string
}

type specChangeTest struct {
	f          *Fixture
	c          SpecChangeCase
	namespace  string
	controller client.Object
}

func (s *specChangeTest) live() (client.Object, error) {
	live, ok := s.controller.DeepCopyObject().(client.Object)
	if !ok {
		s.f.t.Fatalf("can't copy %T", s.controller)
	}
	err := s.f.metacontroller.Get(context.TODO(), client.ObjectKeyFromObject(s.controller), live)
	return live, err
}

// retiredIsEmpty reports whether the cleanup record exists and its
// removeParentFinalizerFrom list is empty.
func (s *specChangeTest) retiredIsEmpty() (bool, error) {
	live, err := s.live()
	if err != nil {
		return false, err
	}
	raw, ok := live.GetAnnotations()[finalizer.CleanupRecordAnnotation]
	if !ok {
		return false, nil
	}
	var record struct {
		RemoveParentFinalizerFrom []json.RawMessage `json:"removeParentFinalizerFrom"`
	}
	if err := json.Unmarshal([]byte(raw), &record); err != nil {
		return false, err
	}
	return len(record.RemoveParentFinalizerFrom) == 0, nil
}

func (s *specChangeTest) createParent(crd *apiextensionsv1.CustomResourceDefinition, parentClient *dynamicclientset.ResourceClient, name string) {
	parent := UnstructuredCRD(crd, name)
	_ = unstructured.SetNestedStringMap(parent.Object, map[string]string{"test": s.namespace}, "spec", "selector", "matchLabels")
	if _, err := parentClient.Namespace(s.namespace).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
		s.f.t.Fatal(err)
	}
}

// waitForFinalizer waits for the parent finalizer on the parent object. want
// is true to wait for the finalizer and false to wait until it is gone.
func (s *specChangeTest) waitForFinalizer(parentClient *dynamicclientset.ResourceClient, name string, want bool) {
	err := s.f.Wait(func() (bool, error) {
		live, err := parentClient.Namespace(s.namespace).Get(context.TODO(), name, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		return controllerutil.ContainsFinalizer(live, s.c.ParentFinalizer) == want, nil
	})
	if err != nil {
		s.f.t.Fatalf("parent %s: the finalizer is not in the expected state (want %v): %v", name, want, err)
	}
}

// RunSpecChangeCleanup checks that Metacontroller removes the parent
// finalizer from the objects of the old resource after a spec change, and
// that the new controller works.
func (f *Fixture) RunSpecChangeCleanup(c SpecChangeCase, namespace string) {
	f.CreateNamespace(namespace)
	hook := f.ServeWebhook(func([]byte) ([]byte, error) { return []byte("{}"), nil })
	oldCRD, oldClient := f.CreateCRD("OldParent", apiextensionsv1.NamespaceScoped)
	newCRD, newClient := f.CreateCRD("UpdatedParent", apiextensionsv1.NamespaceScoped)

	s := &specChangeTest{f: f, c: c, namespace: namespace, controller: c.NewController(hook.URL, *CRDResourceRule(oldCRD))}
	if err := f.metacontroller.Create(context.TODO(), s.controller); err != nil {
		f.t.Fatal(err)
	}
	// The teardown deletes the controller object and waits until it is gone.
	// The parent objects can then be deleted.
	f.deferTeardown(func() error {
		if err := f.metacontroller.Delete(context.TODO(), s.controller); err != nil && !apierrors.IsNotFound(err) {
			return err
		}
		return f.Wait(func() (bool, error) {
			_, err := s.live()
			return apierrors.IsNotFound(err), nil
		})
	})
	s.createParent(oldCRD, oldClient, "old-parent")
	s.waitForFinalizer(oldClient, "old-parent", true)

	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		live, err := s.live()
		if err != nil {
			return err
		}
		c.SetParent(live, *CRDResourceRule(newCRD))
		return f.metacontroller.Update(context.TODO(), live)
	})
	if err != nil {
		f.t.Fatal(err)
	}
	s.createParent(newCRD, newClient, "new-parent")

	s.waitForFinalizer(newClient, "new-parent", true)
	// The cleanup removes the finalizer. The old parent is not deleted.
	s.waitForFinalizer(oldClient, "old-parent", false)
	if err := f.Wait(s.retiredIsEmpty); err != nil {
		f.t.Fatalf("the removeParentFinalizerFrom list of the cleanup record did not become empty: %v", err)
	}
}
