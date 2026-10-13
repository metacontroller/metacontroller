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

package decorator

import (
	"context"
	"fmt"
	"testing"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/finalizer"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	"metacontroller/test/integration/framework"
)

// TestFinalizerRemovedWhenDecoratorControllerDeleted tests that Metacontroller
// removes its finalizer from the parent objects when the DecoratorController
// is deleted.
func TestFinalizerRemovedWhenDecoratorControllerDeleted(t *testing.T) {
	ns := "test-finalizer-cleanup"
	dcName := "dc-finalizer-cleanup"
	finalizerName := "metacontroller.io/decoratorcontroller-" + dcName
	// Use more than one parent. With one parent, a finalizer that a worker
	// adds again during the cleanup is easy to miss.
	const parentCount = 10

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("FinalizerCleanupParent", apiextensions.NamespaceScoped)

	hook := f.ServeWebhook(func(_ []byte) ([]byte, error) {
		return json.Marshal(v1.DecoratorHookResponse{})
	})

	// The finalize hook makes Metacontroller add a finalizer to the parents
	// and to the controller object.
	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      dcName,
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			ResyncPeriodSeconds: ptr.To[int32](3600),
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *framework.CRDResourceRule(parentCRD)},
			},
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync:     &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hook.URL}},
				Finalize: &v1alpha1.Hook{Webhook: &v1alpha1.Webhook{URL: &hook.URL}},
			},
		},
	}
	if err := f.MetacontrollerClient().Create(context.TODO(), dc); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < parentCount; i++ {
		parent := framework.UnstructuredCRD(parentCRD, fmt.Sprintf("parent-%d", i))
		if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}

	parentsWithFinalizer := func() (int, error) {
		list, err := parentClient.Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return 0, err
		}
		count := 0
		for i := range list.Items {
			if controllerutil.ContainsFinalizer(&list.Items[i], finalizerName) {
				count++
			}
		}
		return count, nil
	}

	// If the test fails before it deletes the controller, stop the controller
	// and remove the finalizers, so that the fixture can delete the parent
	// objects. Do the removal again until no parent has the finalizer, because
	// the controller can add it again while it stops.
	defer func() {
		if err := f.MetacontrollerClient().Delete(context.TODO(), dc); err != nil && !apierrors.IsNotFound(err) {
			t.Logf("can't delete the controller during teardown: %v", err)
		}
		_ = f.Wait(func() (bool, error) {
			list, err := parentClient.Namespace(ns).List(context.TODO(), metav1.ListOptions{})
			if err != nil {
				return false, err
			}
			clean := true
			for i := range list.Items {
				if controllerutil.RemoveFinalizer(&list.Items[i], finalizerName) {
					clean = false
					_, _ = parentClient.Namespace(ns).Update(context.TODO(), &list.Items[i], metav1.UpdateOptions{})
				}
			}
			return clean, nil
		})
	}()

	t.Logf("Waiting for the finalizer on the DecoratorController...")
	err := f.Wait(func() (bool, error) {
		live := &v1alpha1.DecoratorController{}
		if err := f.MetacontrollerClient().Get(context.TODO(), client.ObjectKeyFromObject(dc), live); err != nil {
			return false, err
		}
		return controllerutil.ContainsFinalizer(live, finalizer.ControllerFinalizer), nil
	})
	if err != nil {
		t.Fatalf("the DecoratorController did not get the finalizer: %v", err)
	}

	t.Logf("Waiting for the finalizer on all parent objects...")
	err = f.Wait(func() (bool, error) {
		count, err := parentsWithFinalizer()
		return count == parentCount, err
	})
	if err != nil {
		t.Fatalf("parent objects did not get the finalizer: %v", err)
	}

	if err := f.MetacontrollerClient().Delete(context.TODO(), dc); err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for the finalizer to be removed from all parent objects...")
	err = f.Wait(func() (bool, error) {
		count, err := parentsWithFinalizer()
		return count == 0, err
	})
	if err != nil {
		count, _ := parentsWithFinalizer()
		t.Fatalf("%d of %d parent objects still have the finalizer: %v", count, parentCount, err)
	}

	// The DecoratorController must stay until the parent objects are clean, and
	// then it must go away.
	t.Logf("Waiting for the DecoratorController to be deleted...")
	err = f.Wait(func() (bool, error) {
		live := &v1alpha1.DecoratorController{}
		err := f.MetacontrollerClient().Get(context.TODO(), client.ObjectKeyFromObject(dc), live)
		return apierrors.IsNotFound(err), nil
	})
	if err != nil {
		t.Errorf("the DecoratorController was not deleted: %v", err)
	}
	count, err := parentsWithFinalizer()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("%d of %d parent objects have the finalizer after the DecoratorController was deleted", count, parentCount)
	}
}

// TestFinalizerRemovedWhenDecoratorControllerIsNotInOperation tests the cleanup for
// a DecoratorController that has no controller in operation. This is the
// condition after a restart of Metacontroller, or when the controller did not
// start. The DecoratorController has no hooks, thus its controller cannot start.
func TestFinalizerRemovedWhenDecoratorControllerIsNotInOperation(t *testing.T) {
	ns := "test-finalizer-cleanup-stopped"
	name := "dc-finalizer-cleanup-stopped"
	finalizerName := "metacontroller.io/decoratorcontroller-" + name

	f := framework.NewFixture(t)
	defer f.TearDown()

	f.CreateNamespace(ns)
	parentCRD, parentClient := f.CreateCRD("FinalizerCleanupStoppedParent", apiextensions.NamespaceScoped)

	// This parent has the finalizer from an earlier controller.
	parent := framework.UnstructuredCRD(parentCRD, "parent")
	parent.SetFinalizers([]string{finalizerName})
	if _, err := parentClient.Namespace(ns).Create(context.TODO(), parent, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	defer func() {
		live, err := parentClient.Namespace(ns).Get(context.TODO(), "parent", metav1.GetOptions{})
		if err == nil && controllerutil.RemoveFinalizer(live, finalizerName) {
			_, _ = parentClient.Namespace(ns).Update(context.TODO(), live, metav1.UpdateOptions{})
		}
	}()

	dc := &v1alpha1.DecoratorController{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:  "default",
			Name:       name,
			Finalizers: []string{finalizer.ControllerFinalizer},
		},
		Spec: v1alpha1.DecoratorControllerSpec{
			Resources: []v1alpha1.DecoratorControllerResourceRule{
				{ResourceRule: *framework.CRDResourceRule(parentCRD)},
			},
		},
	}
	if err := f.MetacontrollerClient().Create(context.TODO(), dc); err != nil {
		t.Fatal(err)
	}
	defer func() {
		live := &v1alpha1.DecoratorController{}
		if err := f.MetacontrollerClient().Get(context.TODO(), client.ObjectKeyFromObject(dc), live); err != nil {
			return
		}
		live.SetFinalizers(nil)
		_ = f.MetacontrollerClient().Update(context.TODO(), live)
		_ = f.MetacontrollerClient().Delete(context.TODO(), live)
	}()
	if err := f.MetacontrollerClient().Delete(context.TODO(), dc); err != nil {
		t.Fatal(err)
	}

	t.Logf("Waiting for the DecoratorController to be deleted...")
	err := f.Wait(func() (bool, error) {
		live := &v1alpha1.DecoratorController{}
		err := f.MetacontrollerClient().Get(context.TODO(), client.ObjectKeyFromObject(dc), live)
		return apierrors.IsNotFound(err), nil
	})
	if err != nil {
		t.Fatalf("the DecoratorController was not deleted: %v", err)
	}

	live, err := parentClient.Namespace(ns).Get(context.TODO(), "parent", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if controllerutil.ContainsFinalizer(live, finalizerName) {
		t.Errorf("the parent object has the finalizer after the DecoratorController was deleted")
	}
}
