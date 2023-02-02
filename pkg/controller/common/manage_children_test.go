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
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	commonv2 "metacontroller/pkg/controller/common/api/v2"
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
	simpleClientset := NewFakeNewSimpleClientsetWithResources(NewDefaultAPIResourceList())
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
			if err := ManageChildren(tt.args.dynClient(), tt.args.updateStrategy, tt.args.parent, tt.args.observedChildren, tt.args.desiredChildren); (err != nil) != tt.wantErr {
				t.Errorf("ManageChildren() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
