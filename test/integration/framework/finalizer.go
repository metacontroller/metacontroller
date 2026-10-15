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

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"metacontroller/pkg/controller/common/finalizer"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
)

// FinalizerCleanupCase is the input for the tests of the parent finalizer
// cleanup. The composite and the decorator suites use the same tests.
type FinalizerCleanupCase struct {
	// Controller is the CompositeController or DecoratorController. The test
	// creates it.
	Controller client.Object
	// ParentFinalizer is the finalizer that the controller adds to its parents.
	ParentFinalizer string
	// Namespace is the namespace of the parent objects.
	Namespace string
	// ParentClient is the client for the parent resource. It is nil when the
	// parent resource does not exist.
	ParentClient *dynamicclientset.ResourceClient
	// Parents are the parent objects. The test creates them.
	Parents []*unstructured.Unstructured
}

// RunFinalizerCleanupOnDelete creates the controller object and the parent
// objects, and waits until they have their finalizers. Then it deletes the
// controller object. It makes sure that Metacontroller removes the parent
// finalizer from all parent objects, and that the controller object stays until
// the parents are clean. The controller must have a finalize hook.
func (f *Fixture) RunFinalizerCleanupOnDelete(c FinalizerCleanupCase) {
	t := f.t
	defer f.cleanupFinalizerCase(c)

	if err := f.metacontroller.Create(context.TODO(), c.Controller); err != nil {
		t.Fatal(err)
	}
	f.createParents(c)

	t.Logf("Waiting for the finalizer on the controller object...")
	err := f.Wait(func() (bool, error) {
		live, err := f.getController(c)
		if err != nil {
			return false, err
		}
		return controllerutil.ContainsFinalizer(live, finalizer.ControllerFinalizer), nil
	})
	if err != nil {
		t.Fatalf("the controller object did not get the finalizer: %v", err)
	}

	t.Logf("Waiting for the finalizer on all parent objects...")
	err = f.Wait(func() (bool, error) {
		count, err := f.parentsWithFinalizer(c)
		return count == len(c.Parents), err
	})
	if err != nil {
		t.Fatalf("parent objects did not get the finalizer: %v", err)
	}

	if err := f.metacontroller.Delete(context.TODO(), c.Controller); err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for the finalizer to be removed from all parent objects...")
	err = f.Wait(func() (bool, error) {
		count, err := f.parentsWithFinalizer(c)
		return count == 0, err
	})
	if err != nil {
		count, _ := f.parentsWithFinalizer(c)
		t.Fatalf("%d of %d parent objects still have the finalizer: %v", count, len(c.Parents), err)
	}

	f.waitForControllerDeleted(c)
}

// RunFinalizerCleanupNotInOperation does a test of the cleanup for a controller
// object that has no controller in operation. This is the condition after a
// restart of Metacontroller, or when the controller did not start. The
// controller object must have a spec with which its controller cannot start,
// for example a spec without hooks.
func (f *Fixture) RunFinalizerCleanupNotInOperation(c FinalizerCleanupCase) {
	t := f.t
	defer f.cleanupFinalizerCase(c)

	// The parents have the finalizer from an earlier controller.
	for _, parent := range c.Parents {
		parent.SetFinalizers([]string{c.ParentFinalizer})
	}
	f.createParents(c)

	c.Controller.SetFinalizers([]string{finalizer.ControllerFinalizer})
	if err := f.metacontroller.Create(context.TODO(), c.Controller); err != nil {
		t.Fatal(err)
	}
	if err := f.metacontroller.Delete(context.TODO(), c.Controller); err != nil {
		t.Fatal(err)
	}

	f.waitForControllerDeleted(c)
}

func (f *Fixture) createParents(c FinalizerCleanupCase) {
	for _, parent := range c.Parents {
		if _, err := c.ParentClient.Namespace(c.Namespace).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
			f.t.Fatal(err)
		}
	}
}

func (f *Fixture) getController(c FinalizerCleanupCase) (client.Object, error) {
	live, ok := c.Controller.DeepCopyObject().(client.Object)
	if !ok {
		f.t.Fatalf("can't copy %T", c.Controller)
	}
	err := f.metacontroller.Get(context.TODO(), client.ObjectKeyFromObject(c.Controller), live)
	return live, err
}

func (f *Fixture) parentsWithFinalizer(c FinalizerCleanupCase) (int, error) {
	if c.ParentClient == nil {
		return 0, nil
	}
	list, err := c.ParentClient.Namespace(c.Namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return 0, err
	}
	count := 0
	for i := range list.Items {
		if controllerutil.ContainsFinalizer(&list.Items[i], c.ParentFinalizer) {
			count++
		}
	}
	return count, nil
}

// waitForControllerDeleted waits until the controller object is gone, and then
// makes sure that no parent object has the finalizer.
func (f *Fixture) waitForControllerDeleted(c FinalizerCleanupCase) {
	t := f.t
	t.Logf("Waiting for the controller object to be deleted...")
	err := f.Wait(func() (bool, error) {
		_, err := f.getController(c)
		return apierrors.IsNotFound(err), nil
	})
	if err != nil {
		t.Fatalf("the controller object was not deleted: %v", err)
	}
	count, err := f.parentsWithFinalizer(c)
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("%d of %d parent objects have the finalizer after the controller object was deleted", count, len(c.Parents))
	}
}

// cleanupFinalizerCase removes what a failed test leaves, so that the fixture
// can delete the parent objects. It changes nothing after a test that passed.
func (f *Fixture) cleanupFinalizerCase(c FinalizerCleanupCase) {
	if err := f.metacontroller.Delete(context.TODO(), c.Controller); err != nil && !apierrors.IsNotFound(err) {
		f.t.Logf("can't delete the controller object during teardown: %v", err)
	}
	// Remove the finalizer again until no parent has it. The controller can
	// add it again until it stops.
	_ = f.Wait(func() (bool, error) {
		if c.ParentClient == nil {
			return true, nil
		}
		list, err := c.ParentClient.Namespace(c.Namespace).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return false, err
		}
		clean := true
		for i := range list.Items {
			if controllerutil.RemoveFinalizer(&list.Items[i], c.ParentFinalizer) {
				clean = false
				_, _ = c.ParentClient.Namespace(c.Namespace).Update(context.TODO(), &list.Items[i], metav1.UpdateOptions{})
			}
		}
		return clean, nil
	})
	if live, err := f.getController(c); err == nil && len(live.GetFinalizers()) > 0 {
		live.SetFinalizers(nil)
		_ = f.metacontroller.Update(context.TODO(), live)
	}
}
