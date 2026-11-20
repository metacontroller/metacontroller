/*
Copyright 2021 Metacontroller authors.

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

package common

import (
	"context"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	commonv2 "metacontroller/pkg/controller/common/api/v2"
	dynamicapply "metacontroller/pkg/dynamic/apply"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	. "metacontroller/pkg/internal/testutils/common"
	. "metacontroller/pkg/internal/testutils/dynamic/clientset"
	. "metacontroller/pkg/internal/testutils/dynamic/discovery"
	"metacontroller/pkg/logging"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type childUpdateOnDeleteStrategy struct{}

func (m childUpdateOnDeleteStrategy) GetMethod(string, string) v1alpha1.ChildUpdateMethod {
	return v1alpha1.ChildUpdateOnDelete
}

type childUpdateInPlaceStrategy struct{}

func (m childUpdateInPlaceStrategy) GetMethod(string, string) v1alpha1.ChildUpdateMethod {
	return v1alpha1.ChildUpdateInPlace
}

type childUpdateRecreateStrategy struct{}

func (m childUpdateRecreateStrategy) GetMethod(string, string) v1alpha1.ChildUpdateMethod {
	return v1alpha1.ChildUpdateRecreate
}

func TestRevertObjectMetaSystemFields(t *testing.T) {
	origJSON := `{
		"metadata": {
			"origMeta": "should stay gone",
			"otherMeta": "should change value",
			"creationTimestamp": "should restore orig value",
			"deletionTimestamp": "should restore orig value",
			"uid": "should bring back removed value"
		},
		"other": "should change value"
	}`
	newObjJSON := `{
		"metadata": {
			"creationTimestamp": null,
			"deletionTimestamp": "new value",
			"newMeta": "new value",
			"otherMeta": "new value",
			"selfLink": "should be removed"
		},
		"other": "new value"
	}`
	wantJSON := `{
		"metadata": {
			"otherMeta": "new value",
			"newMeta": "new value",
			"creationTimestamp": "should restore orig value",
			"deletionTimestamp": "should restore orig value",
			"uid": "should bring back removed value"
		},
		"other": "new value"
	}`

	orig := make(map[string]interface{})
	if err := json.Unmarshal([]byte(origJSON), &orig); err != nil {
		t.Fatalf("can't unmarshal orig: %v", err)
	}
	newObj := make(map[string]interface{})
	if err := json.Unmarshal([]byte(newObjJSON), &newObj); err != nil {
		t.Fatalf("can't unmarshal newObj: %v", err)
	}
	want := make(map[string]interface{})
	if err := json.Unmarshal([]byte(wantJSON), &want); err != nil {
		t.Fatalf("can't unmarshal want: %v", err)
	}

	err := revertObjectMetaSystemFields(&unstructured.Unstructured{Object: newObj}, &unstructured.Unstructured{Object: orig})
	if err != nil {
		t.Fatalf("revertObjectMetaSystemFields error: %v", err)
	}

	if got := newObj; !reflect.DeepEqual(got, want) {
		t.Logf("reflect diff: a=got, b=want:\n%s", cmp.Diff(got, want))
		t.Fatalf("revertObjectMetaSystemFields() = %#v, want %#v", got, want)
	}
}

func TestIsCacheInvalid(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	const cacheKey = "testgroup/TestKind/testns/testname"
	const someHash uint64 = 12345
	const someUID = "uid-abc"
	const someResourceVersion = "rv-1"
	const someGeneration int64 = 3

	makeObserved := func(uid string, generation int64, resourceVersion string) *unstructured.Unstructured {
		obj := NewDefaultUnstructured()
		obj.SetUID(types.UID(uid))
		obj.SetGeneration(generation)
		obj.SetResourceVersion(resourceVersion)
		return obj
	}

	matchingCache := &lastUpdate{
		hash:               someHash,
		resourcegeneration: someGeneration,
		resourceversion:    someResourceVersion,
		uid:                someUID,
	}

	tests := []struct {
		name        string
		cachedEntry *lastUpdate // nil means no cache entry
		observed    *unstructured.Unstructured
		desiredHash uint64
		wantInvalid bool
	}{
		{
			name:        "no cache entry → invalid",
			cachedEntry: nil,
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: true,
		},
		{
			name: "UID changed → invalid",
			cachedEntry: &lastUpdate{
				hash:               someHash,
				resourcegeneration: someGeneration,
				resourceversion:    someResourceVersion,
				uid:                "old-uid",
			},
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: true,
		},
		{
			name: "desired hash changed → invalid",
			cachedEntry: &lastUpdate{
				hash:               99999,
				resourcegeneration: someGeneration,
				resourceversion:    someResourceVersion,
				uid:                someUID,
			},
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: true,
		},
		{
			name: "generation is 0 and resourceVersion changed → invalid",
			cachedEntry: &lastUpdate{
				hash:               someHash,
				resourcegeneration: 0,
				resourceversion:    "rv-old",
				uid:                someUID,
			},
			observed:    makeObserved(someUID, 0, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: true,
		},
		{
			name: "generation changed → invalid",
			cachedEntry: &lastUpdate{
				hash:               someHash,
				resourcegeneration: 1,
				resourceversion:    someResourceVersion,
				uid:                someUID,
			},
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: true,
		},
		{
			name:        "all fields match (generation > 0) → valid cache hit",
			cachedEntry: matchingCache,
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: false,
		},
		{
			name: "generation is 0 and resourceVersion matches → valid cache hit",
			cachedEntry: &lastUpdate{
				hash:               someHash,
				resourcegeneration: 0,
				resourceversion:    someResourceVersion,
				uid:                someUID,
			},
			observed:    makeObserved(someUID, 0, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: false,
		},
		{
			name: "generation is not changed, but resourceVersion changed → valid cache hit",
			cachedEntry: &lastUpdate{
				hash:               someHash,
				resourcegeneration: someGeneration,
				resourceversion:    "rv-old",
				uid:                someUID,
			},
			observed:    makeObserved(someUID, someGeneration, someResourceVersion),
			desiredHash: someHash,
			wantInvalid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cache := NewLastUpdateCache()
			if tt.cachedEntry != nil {
				cache.Set(cacheKey, tt.cachedEntry.hash, tt.cachedEntry.resourcegeneration, tt.cachedEntry.resourceversion, tt.cachedEntry.uid)
			}
			got := isCacheInvalid(cache, cacheKey, tt.observed, tt.desiredHash)
			if got != tt.wantInvalid {
				t.Errorf("isCacheInvalid() = %v, want %v", got, tt.wantInvalid)
			}
		})
	}
}

func TestManageChildren(t *testing.T) {
	logging.InitLogging(&zap.Options{})
	type args struct {
		dynClient        func() *dynamicclientset.Clientset
		updateStrategy   ChildUpdateStrategy
		parent           *unstructured.Unstructured
		observedChildren commonv2.UniformObjectMap
		desiredChildren  commonv2.UniformObjectMap
	}

	unstructuredDefault := NewDefaultUnstructured()
	unstructuredDefaultList := []*unstructured.Unstructured{unstructuredDefault}
	simpleClientset := NewFakeClientsetWithResources(NewDefaultAPIResourceList())
	testResourceMap := NewFakeResourceMap(simpleClientset)
	testRestConfig := NewDefaultRestConfig()

	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "no error on successful child delete",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateOnDeleteStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: nil,
			},
		},
		{
			name: "no error on child delete with not found api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("delete", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateOnDeleteStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: nil,
			},
		},
		{
			name: "no error on child delete during recreate with not found api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("delete", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateRecreateStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
		},
		{
			name: "error on child delete during recreate with unexpected api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("delete", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateRecreateStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
			wantErr: true,
		},
		{
			name: "error on child delete with unexpected api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("delete", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateOnDeleteStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: nil,
			},
			wantErr: true,
		},
		{
			name: "no error on successful child update",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateInPlaceStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
		},
		{
			name: "no error on child update with not found api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateInPlaceStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
		},
		{
			name: "no error on child update with conflict api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewConflict(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName, nil)
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateInPlaceStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
		},
		{
			name: "error on child update with unexpected api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy: childUpdateInPlaceStrategy{},
				parent:         unstructuredDefault,
				observedChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
			wantErr: true,
		},
		{
			name: "no error on child create with already exists api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewAlreadyExists(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy:   childUpdateInPlaceStrategy{},
				parent:           unstructuredDefault,
				observedChildren: nil,
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
		},
		{
			name: "error on child create with unexpected api error",
			args: args{
				dynClient: func() *dynamicclientset.Clientset {
					simpleDynClient := fake.NewSimpleDynamicClient(scheme, NewDefaultUnstructured())
					simpleDynClient.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
					return NewClientset(testRestConfig, testResourceMap, simpleDynClient)
				},
				updateStrategy:   childUpdateOnDeleteStrategy{},
				parent:           unstructuredDefault,
				observedChildren: nil,
				desiredChildren: commonv2.MakeUniformObjectMap(
					unstructuredDefault,
					unstructuredDefaultList,
				),
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := ManageChildren(context.TODO(), tt.args.dynClient(), tt.args.updateStrategy, tt.args.parent, tt.args.observedChildren, tt.args.desiredChildren, &ApplyOptions{Strategy: ApplyStrategyDynamicApply}); (err != nil) != tt.wantErr {
				t.Errorf("ManageChildren() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func newEchoedAttachment(resourceVersion string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":              "attachment",
			"namespace":         "default",
			"uid":               "uid-1",
			"resourceVersion":   resourceVersion,
			"generation":        int64(1),
			"creationTimestamp": "2026-11-19T00:00:00Z",
		},
		"data":   map[string]interface{}{"key": "value"},
		"status": map[string]interface{}{"phase": "Ready"},
	}}
}

// newHookChild returns a child as a Go hook sends it. A hook that marshals a
// typed object sends creationTimestamp: null and an empty status.
func newHookChild(value string) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":              "attachment",
			"namespace":         "default",
			"creationTimestamp": nil,
		},
		"data":   map[string]interface{}{"key": value},
		"status": map[string]interface{}{},
	}}
}

// addServerFields returns a copy of obj with the fields that the API server
// sets.
func addServerFields(t *testing.T, obj *unstructured.Unstructured, resourceVersion string) *unstructured.Unstructured {
	t.Helper()
	c := obj.DeepCopy()
	c.SetUID("uid-1")
	c.SetResourceVersion(resourceVersion)
	if err := unstructured.SetNestedField(c.Object, "2026-11-19T00:00:00Z", "metadata", "creationTimestamp"); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestApplyUpdate_whenUpdateHasSystemFieldsAndStatus_excludesThemFromLastApplied(t *testing.T) {
	observed := newEchoedAttachment("100")
	update := newEchoedAttachment("100")
	update.Object["data"] = map[string]interface{}{"key": "changed"}

	newObj, err := ApplyUpdate(observed, update)

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	lastApplied, err := dynamicapply.GetLastApplied(newObj)
	if err != nil {
		t.Fatalf("GetLastApplied() error = %v", err)
	}
	for _, fieldName := range objectMetaSystemFields {
		if _, found, _ := unstructured.NestedFieldNoCopy(lastApplied, "metadata", fieldName); found {
			t.Errorf("last-applied annotation contains metadata.%s", fieldName)
		}
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(lastApplied, "status"); found {
		t.Errorf("last-applied annotation contains status")
	}
	if got, _, _ := unstructured.NestedString(lastApplied, "data", "key"); got != "changed" {
		t.Errorf("last-applied data.key = %q, want %q", got, "changed")
	}
}

func TestApplyUpdate_whenCalled_doesNotChangeUpdateArgument(t *testing.T) {
	observed := newEchoedAttachment("100")
	update := newEchoedAttachment("100")
	if err := dynamicapply.SetLastApplied(update, map[string]interface{}{"data": map[string]interface{}{"key": "value"}}); err != nil {
		t.Fatal(err)
	}
	want := update.DeepCopy()

	_, err := ApplyUpdate(observed, update)

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	if diff := cmp.Diff(want.UnstructuredContent(), update.UnstructuredContent()); diff != "" {
		t.Errorf("ApplyUpdate() changed the update argument (-want +got):\n%s", diff)
	}
}

// See: https://github.com/metacontroller/metacontroller/issues/1003
func TestApplyUpdate_whenHookEchoesObservedChild_converges(t *testing.T) {
	observed := newEchoedAttachment("100")
	newObj, err := ApplyUpdate(observed, observed.DeepCopy())
	if err != nil {
		t.Fatalf("first ApplyUpdate() error = %v", err)
	}
	// The API server sets a new resourceVersion after the first update.
	observed = newObj.DeepCopy()
	observed.SetResourceVersion("101")

	newObj, err = ApplyUpdate(observed, observed.DeepCopy())

	if err != nil {
		t.Fatalf("second ApplyUpdate() error = %v", err)
	}
	if diff := cmp.Diff(observed.UnstructuredContent(), newObj.UnstructuredContent()); diff != "" {
		t.Errorf("second sync found a diff, want no update (-observed +new):\n%s", diff)
	}
}

func TestApplyUpdate_whenChildIsCreatedAndHookSendsSameChild_findsNoDiff(t *testing.T) {
	desired := newHookChild("value")
	created := desired.DeepCopy()
	if err := dynamicapply.SetLastApplied(created, sanitizeLastApplied(created.UnstructuredContent())); err != nil {
		t.Fatal(err)
	}
	observed := addServerFields(t, created, "100")

	newObj, err := ApplyUpdate(observed, newHookChild("value"))

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	if diff := cmp.Diff(observed.UnstructuredContent(), newObj.UnstructuredContent()); diff != "" {
		t.Errorf("sync after create found a diff, want no update (-observed +new):\n%s", diff)
	}
}

func TestApplyUpdate_whenOldAnnotationHasSystemFieldsAndHookIsUnchanged_keepsAnnotation(t *testing.T) {
	// An older Metacontroller version stored the full hook output, with
	// system fields and status.
	child := newEchoedAttachment("100")
	if err := dynamicapply.SetLastApplied(child, newEchoedAttachment("100").UnstructuredContent()); err != nil {
		t.Fatal(err)
	}
	child.SetLabels(map[string]string{"other-client": "label"})
	observed := addServerFields(t, child, "200")

	newObj, err := ApplyUpdate(observed, newEchoedAttachment("150"))

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	if diff := cmp.Diff(observed.UnstructuredContent(), newObj.UnstructuredContent()); diff != "" {
		t.Errorf("sync after upgrade found a diff, want no update (-observed +new):\n%s", diff)
	}
}

func TestApplyUpdate_whenOldAnnotationHasEmptyAnnotationsAndHookOmitsThem_keepsOtherAnnotations(t *testing.T) {
	// An older Metacontroller version stored "annotations": {} when a hook
	// echoed a child that had only the last-applied annotation.
	oldLastApplied := newHookChild("value").UnstructuredContent()
	if err := unstructured.SetNestedStringMap(oldLastApplied, map[string]string{}, "metadata", "annotations"); err != nil {
		t.Fatal(err)
	}
	child := newHookChild("value")
	if err := dynamicapply.SetLastApplied(child, oldLastApplied); err != nil {
		t.Fatal(err)
	}
	annotations := child.GetAnnotations()
	annotations["other-client"] = "x"
	child.SetAnnotations(annotations)
	observed := addServerFields(t, child, "100")

	newObj, err := ApplyUpdate(observed, newHookChild("value"))

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	if got := newObj.GetAnnotations()["other-client"]; got != "x" {
		t.Errorf("annotation other-client = %q, want %q", got, "x")
	}
	if diff := cmp.Diff(observed.UnstructuredContent(), newObj.UnstructuredContent()); diff != "" {
		t.Errorf("sync found a diff, want no update (-observed +new):\n%s", diff)
	}
}

func TestApplyUpdate_whenHookChangesChild_writesNewAnnotation(t *testing.T) {
	created := newHookChild("value")
	if err := dynamicapply.SetLastApplied(created, sanitizeLastApplied(created.UnstructuredContent())); err != nil {
		t.Fatal(err)
	}
	observed := addServerFields(t, created, "100")

	newObj, err := ApplyUpdate(observed, newHookChild("changed"))

	if err != nil {
		t.Fatalf("ApplyUpdate() error = %v", err)
	}
	if got, _, _ := unstructured.NestedString(newObj.Object, "data", "key"); got != "changed" {
		t.Errorf("data.key = %q, want %q", got, "changed")
	}
	lastApplied, err := dynamicapply.GetLastApplied(newObj)
	if err != nil {
		t.Fatalf("GetLastApplied() error = %v", err)
	}
	if got, _, _ := unstructured.NestedString(lastApplied, "data", "key"); got != "changed" {
		t.Errorf("last-applied data.key = %q, want %q", got, "changed")
	}
}

func TestSanitizeLastApplied_whenAnnotationsAreEmpty_removesAnnotations(t *testing.T) {
	obj := newHookChild("value").UnstructuredContent()
	if err := unstructured.SetNestedStringMap(obj, map[string]string{}, "metadata", "annotations"); err != nil {
		t.Fatal(err)
	}

	got := sanitizeLastApplied(obj)

	if _, found, _ := unstructured.NestedFieldNoCopy(got, "metadata", "annotations"); found {
		t.Errorf("sanitizeLastApplied() kept an empty metadata.annotations map")
	}
	if _, found, _ := unstructured.NestedFieldNoCopy(obj, "metadata", "annotations"); !found {
		t.Errorf("sanitizeLastApplied() changed its argument")
	}
}

func TestSanitizeLastApplied_whenAnnotationsAreNotEmpty_keepsAnnotations(t *testing.T) {
	obj := newHookChild("value").UnstructuredContent()
	if err := unstructured.SetNestedStringMap(obj, map[string]string{"a": "b"}, "metadata", "annotations"); err != nil {
		t.Fatal(err)
	}

	got := sanitizeLastApplied(obj)

	if value, _, _ := unstructured.NestedString(got, "metadata", "annotations", "a"); value != "b" {
		t.Errorf("metadata.annotations.a = %q, want %q", value, "b")
	}
}
