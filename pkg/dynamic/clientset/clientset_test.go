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

package clientset

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	. "metacontroller/pkg/internal/testutils/dynamic/discovery"
	"metacontroller/pkg/logging"
)

// Local fixtures mirroring pkg/internal/testutils/common, which cannot be
// imported from this test package: that testutil package pulls in
// pkg/controller/common/finalizer, which imports the package under test
// (import cycle not allowed in tests).
const (
	testGroup        = "testgroup"
	testVersion      = "testversion"
	testResource     = "testkinds"
	testResourceList = "TestkindsList"
	testNamespace    = "testns"
	testName         = "testname"
	testKind         = "TestKind"
	testAPIVersion   = "testgroup/testversion"
)

var testGVRToListKind = map[schema.GroupVersionResource]string{
	{Group: testGroup, Version: testVersion, Resource: testResource}: testResourceList,
}

func newTestObject() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": testAPIVersion,
			"kind":       testKind,
			"metadata": map[string]interface{}{
				"namespace": testNamespace,
				"name":      testName,
			},
		},
	}
}

func newObjectWithMetadata(labels, annotations map[string]string, finalizers []string) *unstructured.Unstructured {
	obj := newTestObject()
	if labels != nil {
		obj.SetLabels(labels)
	}
	if annotations != nil {
		obj.SetAnnotations(annotations)
	}
	if finalizers != nil {
		obj.SetFinalizers(finalizers)
	}
	return obj
}

func newTestAPIResourceList() []*metav1.APIResourceList {
	return []*metav1.APIResourceList{{
		TypeMeta:     metav1.TypeMeta{Kind: testKind, APIVersion: testAPIVersion},
		GroupVersion: testAPIVersion,
		APIResources: []metav1.APIResource{{
			Name:       testResource,
			Namespaced: true,
			Group:      testGroup,
			Version:    testVersion,
			Kind:       testKind,
		}},
	}}
}

func newTestRestConfig() *rest.Config {
	return &rest.Config{
		ContentConfig: rest.ContentConfig{
			GroupVersion: &schema.GroupVersion{Group: testGroup, Version: testVersion},
		},
	}
}

// Note: MergePatchValue returns nil both when there is no diff and when the
// diff is "replace with null" (which deletes a key when applied); callers
// invoke it only for fields they already know changed.
func TestMergePatchValue(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	valueTests := []struct {
		name string
		old  interface{}
		new  interface{}
		want interface{}
	}{
		{
			name: "removed map key becomes explicit null",
			old:  map[string]interface{}{"a": "1", "b": "2"},
			new:  map[string]interface{}{"b": "2"},
			want: map[string]interface{}{"a": nil},
		},
		{
			name: "added map key included",
			old:  map[string]interface{}{"a": "1"},
			new:  map[string]interface{}{"a": "1", "b": "2"},
			want: map[string]interface{}{"b": "2"},
		},
		{
			name: "changed scalar replaced",
			old:  "x",
			new:  "y",
			want: "y",
		},
		{
			name: "nil replaced by map",
			old:  nil,
			new:  map[string]interface{}{"a": "1"},
			want: map[string]interface{}{"a": "1"},
		},
		{
			name: "nested maps diffed recursively",
			old:  map[string]interface{}{"s": map[string]interface{}{"a": "1", "b": "2"}},
			new:  map[string]interface{}{"s": map[string]interface{}{"b": "2", "c": "3"}},
			want: map[string]interface{}{"s": map[string]interface{}{"a": nil, "c": "3"}},
		},
		{
			name: "arrays replaced wholesale",
			old:  map[string]interface{}{"l": []interface{}{"a", "b"}},
			new:  map[string]interface{}{"l": []interface{}{"a"}},
			want: map[string]interface{}{"l": []interface{}{"a"}},
		},
	}
	for _, tt := range valueTests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergePatchValue(tt.old, tt.new)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MergePatchValue() = %v, want %v", got, tt.want)
			}
		})
	}

	noDiffTests := []struct {
		name string
		old  interface{}
		new  interface{}
	}{
		{
			name: "equal maps",
			old:  map[string]interface{}{"a": "1"},
			new:  map[string]interface{}{"a": "1"},
		},
		{
			name: "equal nested maps",
			old:  map[string]interface{}{"s": map[string]interface{}{"a": "1"}},
			new:  map[string]interface{}{"s": map[string]interface{}{"a": "1"}},
		},
		{
			name: "nil to nil",
			old:  nil,
			new:  nil,
		},
		{
			name: "equal scalars",
			old:  "x",
			new:  "x",
		},
	}
	for _, tt := range noDiffTests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MergePatchValue(tt.old, tt.new); got != nil {
				t.Errorf("MergePatchValue() = %v, want nil", got)
			}
		})
	}
}

func TestStringMapMergePatchValue(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	tests := []struct {
		name    string
		old     map[string]string
		new     map[string]string
		want    map[string]interface{}
		wantNil bool
	}{
		{
			name: "removed key becomes explicit null",
			old:  map[string]string{"a": "1", "b": "2"},
			new:  map[string]string{"b": "2"},
			want: map[string]interface{}{"a": nil},
		},
		{
			name: "added and changed keys carry new values",
			old:  map[string]string{"a": "1"},
			new:  map[string]string{"a": "2", "c": "3"},
			want: map[string]interface{}{"a": "2", "c": "3"},
		},
		{
			name:    "equal maps produce nil",
			old:     map[string]string{"a": "1"},
			new:     map[string]string{"a": "1"},
			wantNil: true,
		},
		{
			name: "nil old map adds everything",
			old:  nil,
			new:  map[string]string{"a": "1"},
			want: map[string]interface{}{"a": "1"},
		},
		{
			name: "nil new map removes everything",
			old:  map[string]string{"a": "1"},
			new:  nil,
			want: map[string]interface{}{"a": nil},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StringMapMergePatchValue(tt.old, tt.new)
			if tt.wantNil {
				if got != nil {
					t.Errorf("StringMapMergePatchValue() = %v, want nil", got)
				}
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("StringMapMergePatchValue() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetadataJSONMergePatch(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	t.Run("label deletion becomes explicit null entry", func(t *testing.T) {
		older := newObjectWithMetadata(map[string]string{"a": "1", "b": "2"}, nil, nil)
		newer := newObjectWithMetadata(map[string]string{"b": "2"}, nil, nil)
		patch, err := metadataJSONMergePatch(older, newer)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]interface{}
		if err := json.Unmarshal(patch, &got); err != nil {
			t.Fatal(err)
		}
		md, ok := got["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("patch missing metadata: %s", patch)
		}
		labels, ok := md["labels"].(map[string]interface{})
		if !ok {
			t.Fatalf("patch metadata missing labels: %s", patch)
		}
		if v, exists := labels["a"]; !exists || v != nil {
			t.Errorf("want labels.a = null (delete), got %v (patch: %s)", v, patch)
		}
		if _, exists := labels["b"]; exists {
			t.Errorf("unchanged label b must not appear in the patch: %s", patch)
		}
	})

	t.Run("nil patch when nothing supported changed", func(t *testing.T) {
		older := newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
		newer := older.DeepCopy()
		newer.Object["spec"] = map[string]interface{}{"x": "y"}
		patch, err := metadataJSONMergePatch(older, newer)
		if err != nil {
			t.Fatal(err)
		}
		if patch != nil {
			t.Errorf("want nil patch for a spec-only change, got %s", patch)
		}
	})

	t.Run("finalizers replaced wholesale", func(t *testing.T) {
		older := newObjectWithMetadata(nil, nil, []string{"a", "b"})
		newer := newObjectWithMetadata(nil, nil, []string{"a"})
		patch, err := metadataJSONMergePatch(older, newer)
		if err != nil {
			t.Fatal(err)
		}
		var got map[string]interface{}
		if err := json.Unmarshal(patch, &got); err != nil {
			t.Fatal(err)
		}
		md := got["metadata"].(map[string]interface{})
		finalizers, ok := md["finalizers"].([]interface{})
		if !ok || !reflect.DeepEqual(finalizers, []interface{}{"a"}) {
			t.Errorf("want finalizers [a], got %v (patch: %s)", md["finalizers"], patch)
		}
	})
}

func TestUpdateTouchesFieldsOutsideMetadata(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	tests := []struct {
		name   string
		before func() *unstructured.Unstructured
		after  func() *unstructured.Unstructured
		want   bool
	}{
		{
			name: "spec change detected",
			before: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
			},
			after: func() *unstructured.Unstructured {
				obj := newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
				obj.Object["spec"] = map[string]interface{}{"x": "y"}
				return obj
			},
			want: true,
		},
		{
			name: "status change detected",
			before: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
			},
			after: func() *unstructured.Unstructured {
				obj := newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
				obj.Object["status"] = map[string]interface{}{"x": "y"}
				return obj
			},
			want: true,
		},
		{
			name: "metadata-only changes not detected",
			before: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"a": "1", "b": "2"}, nil, nil)
			},
			after: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"b": "2", "c": "3"}, map[string]string{"k": "v"}, []string{"f"})
			},
			want: false,
		},
		{
			name: "no change at all not detected",
			before: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
			},
			after: func() *unstructured.Unstructured {
				return newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := updateTouchesFieldsOutsideMetadata(tt.before(), tt.after()); got != tt.want {
				t.Errorf("updateTouchesFieldsOutsideMetadata() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAtomicUpdate(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	newResourceClient := func(obj *unstructured.Unstructured, reactFn func(fakeDynamicClient *fake.FakeDynamicClient)) *ResourceClient {
		simpleDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), testGVRToListKind, obj)
		reactFn(simpleDynClient)
		simpleClientset := fakeclientset.NewClientset(newTestObject())
		simpleClientset.Resources = newTestAPIResourceList()
		resourceMap := NewFakeResourceMap(simpleClientset)
		restConfig := newTestRestConfig()
		testClientset := NewClientset(restConfig, resourceMap, simpleDynClient)
		rc, err := testClientset.Resource(testAPIVersion, testResource)
		if err != nil {
			t.Fatal(err)
		}
		return rc
	}

	t.Run("metadata-only change is applied with a merge patch carrying null deletions", func(t *testing.T) {
		var patchBody []byte
		var updateCalls int
		obj := newObjectWithMetadata(map[string]string{"a": "1", "b": "2"}, nil, nil)
		rc := newResourceClient(obj, func(fakeDynamicClient *fake.FakeDynamicClient) {
			fakeDynamicClient.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				patchAction, ok := action.(clientgotesting.PatchAction)
				if !ok {
					return true, nil, nil
				}
				patchBody = patchAction.GetPatch()
				return true, nil, nil
			})
			fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				updateCalls++
				return true, nil, nil
			})
		})
		orig := newObjectWithMetadata(map[string]string{"a": "1", "b": "2"}, nil, nil)
		_, err := rc.Namespace(obj.GetNamespace()).AtomicUpdate(context.TODO(), orig, func(o *unstructured.Unstructured) bool {
			labels := o.GetLabels()
			delete(labels, "a")
			o.SetLabels(labels)
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		if updateCalls != 0 {
			t.Errorf("want patch-only path, got %d update calls", updateCalls)
		}
		if patchBody == nil {
			t.Fatal("no patch action recorded")
		}
		var patch map[string]interface{}
		if err := json.Unmarshal(patchBody, &patch); err != nil {
			t.Fatal(err)
		}
		md, ok := patch["metadata"].(map[string]interface{})
		if !ok {
			t.Fatalf("patch missing metadata: %s", patchBody)
		}
		labels, ok := md["labels"].(map[string]interface{})
		if !ok {
			t.Fatalf("patch metadata missing labels: %s", patchBody)
		}
		if v, exists := labels["a"]; !exists || v != nil {
			t.Errorf("want labels.a = null (delete), got %v (patch: %s)", v, patchBody)
		}
		if _, exists := labels["b"]; exists {
			t.Errorf("unchanged label b must not appear in the patch: %s", patchBody)
		}
	})

	t.Run("metadata plus spec change falls back to a full update and keeps the spec change", func(t *testing.T) {
		var patchCalls int
		var updatedSpec interface{}
		obj := newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
		rc := newResourceClient(obj, func(fakeDynamicClient *fake.FakeDynamicClient) {
			fakeDynamicClient.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				patchCalls++
				return true, nil, nil
			})
			fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
				updateAction, ok := action.(clientgotesting.UpdateAction)
				if ok {
					updated, castOk := updateAction.GetObject().(*unstructured.Unstructured)
					if castOk {
						updatedSpec = updated.Object["spec"]
					}
				}
				return true, nil, nil
			})
		})
		orig := newObjectWithMetadata(map[string]string{"a": "1"}, nil, nil)
		_, err := rc.Namespace(obj.GetNamespace()).AtomicUpdate(context.TODO(), orig, func(o *unstructured.Unstructured) bool {
			labels := o.GetLabels()
			delete(labels, "a")
			o.SetLabels(labels)
			o.Object["spec"] = map[string]interface{}{"x": "y"}
			return true
		})
		if err != nil {
			t.Fatal(err)
		}
		if patchCalls != 0 {
			t.Errorf("want full-update fallback, got %d patch calls", patchCalls)
		}
		if !reflect.DeepEqual(updatedSpec, map[string]interface{}{"x": "y"}) {
			t.Errorf("spec change lost in fallback update; updated spec = %v", updatedSpec)
		}
	})
}
