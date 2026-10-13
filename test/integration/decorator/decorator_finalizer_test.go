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
	"time"

	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	"metacontroller/test/integration/framework"
)

// TestFinalizerRemovedWhenDecoratorControllerDeleted tests that Metacontroller
// removes its finalizer from the parent objects when the DecoratorController
// is deleted, and that the finalizer does not come back.
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

	// The finalize hook makes Metacontroller add the finalizer to the parents.
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

	// If the test fails, remove the finalizers so that the fixture can delete
	// the parent objects.
	defer func() {
		list, err := parentClient.Namespace(ns).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return
		}
		for i := range list.Items {
			if controllerutil.RemoveFinalizer(&list.Items[i], finalizerName) {
				_, _ = parentClient.Namespace(ns).Update(context.TODO(), &list.Items[i], metav1.UpdateOptions{})
			}
		}
	}()

	t.Logf("Waiting for the finalizer on all parent objects...")
	err := f.Wait(func() (bool, error) {
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

	// Make sure that no worker adds the finalizer again.
	time.Sleep(2 * time.Second)
	count, err := parentsWithFinalizer()
	if err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Errorf("%d of %d parent objects got the finalizer again", count, parentCount)
	}
}
