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

package customize

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	v1 "metacontroller/pkg/controller/common/customize/api/v1"

	"github.com/go-logr/logr/funcr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/api"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"
	"metacontroller/pkg/hooks"

	"metacontroller/pkg/internal/testutils/dynamic/discovery"
	. "metacontroller/pkg/internal/testutils/hooks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clienttesting "k8s.io/client-go/testing"
	controllerruntimemetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
)

const (
	someValue            = "some"
	labelKeyAAA          = "aaa"
	labelValueBBB        = "bbb"
	labelValueProd       = "prod"
	nsNamespace          = "Namespace"
	nameValue            = "name"
	labelKeyEnv          = "env"
	someNS               = "some-ns"
	resourceSecrets      = "secrets"
	kindSecret           = "Secret"
	resourceConfigMaps   = "configmaps"
	kindConfigMap        = "ConfigMap"
	resourceNamespaces   = "namespaces"
	rbacAPIVersion       = "rbac.authorization.k8s.io/v1"
	resourceClusterRoles = "clusterroles"
	kindClusterRole      = "ClusterRole"
	rbacGroup            = "rbac.authorization.k8s.io"
	kindParent           = "Parent"
	groupTest            = "test"
)

var (
	fakeEnqueueParent = func(obj interface{}) {}
	dynClient         = dynamicclientset.Clientset{}
	dynInformers      = dynamicinformer.SharedInformerFactory{}
)

var fakeLogger = funcr.New(
	func(pfx, args string) { fmt.Println(pfx, args) },
	funcr.Options{
		LogCaller:    funcr.All,
		LogTimestamp: true,
	})

var customizeManagerWithNilController, _ = NewCustomizeManager(
	context.TODO(),
	"test",
	fakeEnqueueParent,
	nil,
	&NilCustomizableController{},
	&dynClient,
	&dynInformers,
	common.NewInformerMap(),
	common.NewGroupKindMap(),
	fakeLogger,
	common.CompositeController,
	nil,
)

var customizeManagerWithFakeController, _ = NewCustomizeManager(
	context.TODO(),
	"test",
	fakeEnqueueParent,
	nil,
	&FakeCustomizableController{},
	&dynClient,
	&dynInformers,
	common.NewInformerMap(),
	common.NewGroupKindMap(),
	fakeLogger,
	common.DecoratorController,
	nil,
)

func TestGetRelatedObjects_whenHookDisabled_returnEmptyMap(t *testing.T) {
	parent := &unstructured.Unstructured{}
	parent.SetName("test")
	parent.SetGeneration(1)

	relatedObjects, err := customizeManagerWithNilController.GetRelatedObjects(context.TODO(), parent)
	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if len(relatedObjects.List()) != 0 {
		t.Errorf("Expected empty map, got %v", relatedObjects)
	}
}

func TestGetRelatedObject_requestResponse(t *testing.T) {
	expectedResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV1,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{{
			ResourceRule: v1alpha1.ResourceRule{
				APIVersion: someValue,
				Resource:   someValue,
			},
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
			Namespace:     nsNamespace,
			Names:         []string{nameValue},
		}},
	}

	customizeManagerWithFakeController.customizeHook = NewHookExecutorStub(expectedResponse)
	parent := &unstructured.Unstructured{}
	parent.SetUID(someValue)
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.getCustomizeHookResponse(context.TODO(), parent)
	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if !reflect.DeepEqual(*response, *expectedResponse) {
		t.Errorf("Response should be equal to %v, got %v", expectedResponse, response)
	}

	if _, found := customizeManagerWithFakeController.customizeCache.Get(customizeKey{someValue, 1}); !found {
		t.Error("Expected not nil here, response should be cached")
	}
}

func TestDetermineSelectionType_returnNamespaceAndLabelsWhenLabelSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
		Namespace:     nsNamespace,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndLabels, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceSelectorWhenPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceSelector || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceSelector, selectionType)
	}
}

func TestDetermineSelectionType_returnErrorWhenNamespaceSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
		Namespace:         someNS,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid || err == nil {
		t.Errorf("Expected invalid selection type due to combining namespace and namespaceSelector")
	}
}

func TestDetermineSelectionType_returnErrorWhenNamespaceSelectorAndNamesIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
		Names:             []string{"name"},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid || err == nil {
		t.Errorf("Expected invalid selection type due to combining names and namespaceSelector")
	}
}

func TestDetermineSelectionType_returnErrorWhenLabelSelectorAndNameIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
		Names:         []string{nameValue},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid && err == nil {
		t.Errorf("Expected error and 'invalid' selection type, but got %v", selectionType)
	}
}

func TestDetermineSelectionType_returnLabelSelectorWhenPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: nil,
		Namespace:     someValue,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamesIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: nil,
		Names:         []string{nameValue},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnLabelsWhenNothingIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: someValue,
			Resource:   someValue,
		},
		LabelSelector: nil,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func Test_matchRelatedRule(t *testing.T) {
	fakeGenericParent := func() *unstructured.Unstructured {
		p := &unstructured.Unstructured{}
		p.SetAPIVersion("other")
		p.SetKind("other")
		return p
	}
	fakeGenericParentWithNamespace := func() *unstructured.Unstructured {
		p := fakeGenericParent()
		p.SetNamespace(someValue)
		return p
	}

	tests := []struct {
		name            string
		hookVersion     v1alpha1.HookVersion
		isNamespaced    bool
		parent          *unstructured.Unstructured
		related         *unstructured.Unstructured
		relatedRule     *v1alpha1.RelatedResourceRule
		relatedRuleKind string
		dynInformers    *dynamicinformer.SharedInformerFactory
		wantMatch       bool
		wantErr         bool
	}{
		// When parent is namespace scoped
		{
			name:        "return true if labels match",
			hookVersion: v1alpha1.HookVersionV1,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetLabels(map[string]string{labelKeyAAA: labelValueBBB})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       true,
		},
		{
			name:        "return false when labels do not match",
			hookVersion: v1alpha1.HookVersionV1,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetLabels(map[string]string{labelKeyAAA: "cbb"})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		{
			name:        "return false when no labels",
			hookVersion: v1alpha1.HookVersionV1,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{}
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		{
			name:        "return true if labels and namespace match",
			hookVersion: v1alpha1.HookVersionV2,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someNS)
				rc.SetLabels(map[string]string{labelKeyAAA: labelValueBBB})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
				Namespace:     someNS,
			},
			relatedRuleKind: kindSecret,
			wantMatch:       true,
		},
		{
			name:        "return false if labels match but namespace does not match",
			hookVersion: v1alpha1.HookVersionV2,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace("other-ns")
				rc.SetLabels(map[string]string{labelKeyAAA: labelValueBBB})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyAAA: labelValueBBB}},
				Namespace:     someNS,
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		{
			name:         "return true when parent is namespace scoped and name and namespace matches",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someValue)
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       true,
		},
		{
			name:         "return false when parent is namespace scoped and name matches but namespace does not match",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion(someValue)
				rc.SetKind(kindSecret)
				rc.SetNamespace("other")
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		{
			name:         "return false when parent is namespace scoped and namespace matches but name does not match",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someValue)
				rc.SetName("othername")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		// When parent is cluster scoped
		{
			name:         "return true when name and namespace matches",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: false,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someValue)
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       true,
		},
		{
			name:         "return false when name matches but namespace does not match",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: false,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace("other")
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
			wantErr:         true,
		},
		{
			name:         "return false when namespace matches but name does not match",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: false,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someValue)
				rc.SetName("othername")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     someValue,
				Names:         []string{nameValue},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		// v2 improvements
		{
			name:         "v2: return true when parent is namespace scoped and related is cluster scoped",
			hookVersion:  v1alpha1.HookVersionV2,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind("Namespace")
				rc.SetName("some-namespace")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceNamespaces,
				},
				LabelSelector: nil,
				Names:         []string{"some-namespace"},
			},
			relatedRuleKind: "Namespace",
			wantMatch:       true,
		},
		{
			name:         "v2: return true when parent is namespace scoped and related is in different namespace",
			hookVersion:  v1alpha1.HookVersionV2,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace("other-namespace")
				rc.SetName("some-secret")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     "other-namespace",
				Names:         []string{"some-secret"},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       true,
		},
		{
			name:         "v2: return false when parent is namespace scoped and related is in different namespace than requested in rule",
			hookVersion:  v1alpha1.HookVersionV2,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace("other-namespace")
				rc.SetName("some-secret")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				LabelSelector: nil,
				Namespace:     "yet-another-namespace",
				Names:         []string{"some-secret"},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
		},
		{
			name:         "return ErrRelatedInformerNotSynced when namespaceSelector is used but nsInformer not synced",
			hookVersion:  v1alpha1.HookVersionV2,
			isNamespaced: false,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind(kindSecret)
				rc.SetNamespace(someValue)
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   resourceSecrets,
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
			},
			relatedRuleKind: kindSecret,
			wantMatch:       false,
			wantErr:         true,
			dynInformers: dynamicinformer.NewSharedInformerFactory(
				dynamicclientset.NewClientset(&rest.Config{}, discovery.NewFakeResourceMap(fakeclientset.NewClientset()), nil),
				0,
			),
		},
		{
			name:         "return error when namespaceSelector is used for cluster-scoped resource",
			hookVersion:  v1alpha1.HookVersionV2,
			isNamespaced: false,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind("ClusterRole")
				rc.SetName(nameValue)
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: rbacAPIVersion,
					Resource:   resourceClusterRoles,
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
			},
			relatedRuleKind: kindClusterRole,
			wantMatch:       false,
			wantErr:         true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.dynInformers == nil {
				// Provide a fake if not specified, some tests might need it.
				tt.dynInformers = dynamicinformer.NewSharedInformerFactory(nil, 0)
			}
			var nsInformer *dynamicinformer.ResourceInformer
			if tt.dynInformers.IsInitialized() {
				nsInformer, _ = tt.dynInformers.Resource(context.TODO(), "v1", "namespaces")
			}
			rm := &Manager{
				dynInformers: tt.dynInformers,
				nsInformer:   nsInformer,
			}
			matches, err := rm.matchesRelatedRule(tt.hookVersion, tt.isNamespaced, tt.parent, tt.related, tt.relatedRule, tt.relatedRuleKind, tt.related.GetNamespace() != "")
			if err != nil && !tt.wantErr {
				t.Error(err)
			}
			if matches != tt.wantMatch {
				t.Errorf("Expected match: %v, actual match: %v", tt.wantMatch, matches)
			}
		})
	}
}

// countingHookExecutorStub counts the calls to the hook that it wraps.
type countingHookExecutorStub struct {
	hooks.Hook
	calls atomic.Int32
}

func (h *countingHookExecutorStub) Call(ctx context.Context, request api.WebhookRequest, response interface{}) error {
	h.calls.Add(1)
	return h.Hook.Call(ctx, request, response)
}

// Test_findRelatedParents makes sure that the customize webhook is called
// only for parents that satisfy matchesParent. If matchesParent is nil, the
// webhook is called for all parents.
func Test_findRelatedParents(t *testing.T) {
	const (
		selectorAnnotation    = "transform"
		matchingParentName    = "matching-secret"
		nonMatchingParentName = "non-matching-secret"
	)

	// Setup fake discovery
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: resourceSecrets, Kind: kindSecret, Namespaced: true, Group: "", Version: "v1", Verbs: []string{"get", "list", "watch"}},
				{Name: resourceConfigMaps, Kind: kindConfigMap, Namespaced: true, Group: "", Version: "v1", Verbs: []string{"get", "list", "watch"}},
			},
		},
	}
	resourceMap := discovery.NewFakeResourceMap(simple)

	// Create parent objects. Only the first one has the annotation.
	matchingParent := &unstructured.Unstructured{}
	matchingParent.SetAPIVersion("v1")
	matchingParent.SetKind(kindSecret)
	matchingParent.SetNamespace(someNS)
	matchingParent.SetName(matchingParentName)
	matchingParent.SetUID("matching-uid")
	matchingParent.SetAnnotations(map[string]string{selectorAnnotation: "true"})

	nonMatchingParent := &unstructured.Unstructured{}
	nonMatchingParent.SetAPIVersion("v1")
	nonMatchingParent.SetKind(kindSecret)
	nonMatchingParent.SetNamespace(someNS)
	nonMatchingParent.SetName(nonMatchingParentName)
	nonMatchingParent.SetUID("non-matching-uid")

	// Setup fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: resourceSecrets}: "SecretList",
	}, matchingParent, nonMatchingParent)
	dynClient := dynamicclientset.NewClientset(&rest.Config{}, resourceMap, fakeDynClient)

	// Setup parent informer and wait for sync
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, 0)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	parentInformer, err := dynInformers.Resource(ctx, "v1", resourceSecrets)
	if err != nil {
		t.Fatalf("can't create parent informer: %v", err)
	}
	defer parentInformer.Close()
	if !waitForSync(ctx, parentInformer) {
		t.Fatal("timed out waiting for parent informer sync")
	}

	parentInformers := common.NewInformerMap()
	parentInformers.Set(schema.GroupVersionResource{Group: "", Version: "v1", Resource: resourceSecrets}, parentInformer)

	parentKinds := common.NewGroupKindMap()
	parentKinds.Set(schema.GroupKind{Group: "", Kind: kindSecret}, &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: resourceSecrets, Namespaced: true, Kind: kindSecret, Group: "", Version: "v1"},
	})

	// Setup customize hook response. The rule selects all ConfigMaps.
	customizeResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV1,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{{
			ResourceRule:  v1alpha1.ResourceRule{APIVersion: "v1", Resource: resourceConfigMaps},
			LabelSelector: &metav1.LabelSelector{},
		}},
	}

	// Create related object
	relatedConfigMap := &unstructured.Unstructured{}
	relatedConfigMap.SetAPIVersion("v1")
	relatedConfigMap.SetKind(kindConfigMap)
	relatedConfigMap.SetNamespace(someNS)
	relatedConfigMap.SetName("some-configmap")

	tests := []struct {
		name          string
		matchesParent func(*unstructured.Unstructured) bool
		wantParents   []string
	}{
		{
			name: "selector filters out non-matching parent",
			matchesParent: func(parent *unstructured.Unstructured) bool {
				return parent.GetAnnotations()[selectorAnnotation] == "true"
			},
			wantParents: []string{matchingParentName},
		},
		{
			name:          "nil matchesParent considers every parent",
			matchesParent: nil,
			wantParents:   []string{matchingParentName, nonMatchingParentName},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			customizeHook := &countingHookExecutorStub{Hook: NewHookExecutorStub(customizeResponse)}
			rm := &Manager{
				parentKinds:      parentKinds,
				dynClient:        dynClient,
				dynInformers:     dynInformers,
				parentInformers:  parentInformers,
				relatedInformers: common.NewInformerMap(),
				customizeCache:   newResponseCache(),
				customizeHook:    customizeHook,
				matchesParent:    tt.matchesParent,
				logger:           fakeLogger,
				ctx:              ctx,
			}

			matches := rm.findRelatedParents(relatedConfigMap)

			if got, want := int(customizeHook.calls.Load()), len(tt.wantParents); got != want {
				t.Errorf("expected customize hook to be called %d time(s), got %d", want, got)
			}
			gotParents := make([]string, 0, len(matches))
			for _, match := range matches {
				gotParents = append(gotParents, match.GetName())
			}
			slices.Sort(gotParents)
			if !slices.Equal(gotParents, tt.wantParents) {
				t.Errorf("expected related parents %v, got %v", tt.wantParents, gotParents)
			}
		})
	}
}

var configMapsGVR = schema.GroupVersionResource{Group: "", Version: "v1", Resource: resourceConfigMaps}

func TestEnqueueParentsWhenSynced_whenParentsWait_enqueuesThem(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	f.rm.markWaiting(f.syncedInformer, newParentSecret("first"))
	f.rm.markWaiting(f.syncedInformer, newParentSecret("second"))

	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, f.syncedInformer)

	got := f.enqueued.names()
	slices.Sort(got)
	if want := []string{"first", "second"}; !slices.Equal(got, want) {
		t.Errorf("expected parents %v to be enqueued, got %v", want, got)
	}
}

func TestEnqueueParentsWhenSynced_whenParentSyncedMeanwhile_doesNotEnqueueIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	synced := newParentSecret("synced")
	f.rm.markWaiting(f.syncedInformer, newParentSecret("waiting"))
	f.rm.markWaiting(f.syncedInformer, synced)
	f.rm.waiting.remove(f.syncedInformer, synced)

	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, f.syncedInformer)

	if got, want := f.enqueued.names(), []string{"waiting"}; !slices.Equal(got, want) {
		t.Errorf("expected parents %v to be enqueued, got %v", want, got)
	}
}

func TestEnqueueParentsWhenSynced_whenParentWaitsForOtherInformer_doesNotEnqueueIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	other, err := f.rm.dynInformers.Resource(ctx, "v1", resourceSecrets)
	if err != nil {
		t.Fatalf("can't create other informer: %v", err)
	}
	defer other.Close()
	f.rm.markWaiting(other, newParentSecret("other-informer"))

	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, f.syncedInformer)

	if got := f.enqueued.names(); len(got) != 0 {
		t.Errorf("expected no parents to be enqueued, got %v", got)
	}
}

func TestEnqueueParentsWhenSynced_whenContextIsCanceled_enqueuesNoParents(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	f.holdRelatedList()
	notSynced, err := f.rm.dynInformers.Resource(ctx, "v1", resourceConfigMaps)
	if err != nil {
		t.Fatalf("can't create related informer: %v", err)
	}
	defer notSynced.Close()
	f.rm.markWaiting(notSynced, newParentSecret("parent"))
	cancel()

	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, notSynced)

	if got := f.enqueued.names(); len(got) != 0 {
		t.Errorf("expected no parents to be enqueued, got %v", got)
	}
}

func TestEnqueueParentsWhenSynced_whenParentsAreEnqueued_recordsTheWakeupMetrics(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	f.rm.name = "wakeup-metrics-test-controller"
	f.rm.markWaiting(f.syncedInformer, newParentSecret("first"))
	f.rm.markWaiting(f.syncedInformer, newParentSecret("second"))
	const wakeups = "metacontroller_customize_related_informer_wakeups_total"
	const parents = "metacontroller_customize_related_informer_enqueued_parents_total"
	wakeupsBefore := wakeupCounterValue(t, wakeups, f.rm.name)
	parentsBefore := wakeupCounterValue(t, parents, f.rm.name)

	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, f.syncedInformer)

	if got := wakeupCounterValue(t, wakeups, f.rm.name) - wakeupsBefore; got != 1 {
		t.Errorf("expected 1 wake-up in the metric, got %v", got)
	}
	if got := wakeupCounterValue(t, parents, f.rm.name) - parentsBefore; got != 2 {
		t.Errorf("expected 2 enqueued parents in the metric, got %v", got)
	}
}

func TestMarkWaiting_whenWakeupIsDone_enqueuesParentImmediately(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	f.rm.enqueueParentsWhenSynced(ctx, configMapsGVR, f.syncedInformer)

	f.rm.markWaiting(f.syncedInformer, newParentSecret("late"))

	if got, want := f.enqueued.names(), []string{"late"}; !slices.Equal(got, want) {
		t.Errorf("expected parents %v to be enqueued, got %v", want, got)
	}
}

func TestGetRelatedObjects_whenRelatedInformerIsNotSynced_enqueuesParentAfterCacheSync(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	parent := newParentSecret("parent")
	f := newWakeupFixture(ctx, t, parent)
	f.rm.customizeHook = NewHookExecutorStub(responseWithRelatedRule("v1", resourceConfigMaps))
	release := f.holdRelatedList()

	_, err := f.rm.GetRelatedObjects(ctx, parent)

	if !errors.Is(err, ErrRelatedInformerNotSynced) {
		t.Fatalf("expected ErrRelatedInformerNotSynced while the list is held, got %v", err)
	}
	if got := f.enqueued.names(); len(got) != 0 {
		t.Fatalf("expected no parents before the related cache is filled, got %v", got)
	}
	release()
	f.enqueued.waitFor(ctx, t, "parent")
}

func TestGetRelatedObjects_whenRelatedInformerIsSynced_forgetsTheWaitingParent(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	parent := newParentSecret("parent")
	f := newWakeupFixture(ctx, t, parent)
	f.rm.customizeHook = NewHookExecutorStub(responseWithRelatedRule("v1", resourceConfigMaps))
	related, err := f.rm.dynInformers.Resource(ctx, "v1", resourceConfigMaps)
	if err != nil {
		t.Fatalf("can't create related informer: %v", err)
	}
	if !waitForSync(ctx, related) {
		t.Fatal("timed out waiting for related informer sync")
	}
	f.rm.relatedInformers.Set(configMapsGVR, related)
	f.rm.markWaiting(related, parent)

	_, err = f.rm.GetRelatedObjects(ctx, parent)

	if err != nil {
		t.Fatalf("can't get related objects: %v", err)
	}
	if got := f.rm.waiting.take(related); len(got) != 0 {
		t.Errorf("expected no waiting parents after a successful sync, got %d", len(got))
	}
}

func TestGetOrCreateRelatedInformer_whenCacheIsAlreadyFilled_wakesParentsThatWait(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	f := newWakeupFixture(ctx, t)
	// Another controller already has a filled informer for the resource. So
	// the new informer of this manager is filled when it is created.
	shared, err := f.rm.dynInformers.Resource(ctx, "v1", resourceConfigMaps)
	if err != nil {
		t.Fatalf("can't create shared informer: %v", err)
	}
	defer shared.Close()
	if !waitForSync(ctx, shared) {
		t.Fatal("timed out waiting for shared informer sync")
	}

	related, err := f.rm.getOrCreateRelatedInformer("v1", resourceConfigMaps, configMapsGVR)

	if err != nil {
		t.Fatalf("can't create related informer: %v", err)
	}
	// A parallel sync could get the informer from the map while it was not
	// filled, and record its parent now. The wake-up must still reach it.
	f.rm.markWaiting(related, newParentSecret("parallel"))
	f.enqueued.waitFor(ctx, t, "parallel")
}

func TestGetRelatedClient_whenRelatedObjectExistsBeforeInformer_doesNotEnqueueParentsForIt(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	// Each parent has a rule for one ConfigMap. Only the first ConfigMap
	// exists before the related informer.
	f := newWakeupFixture(ctx, t, newParentSecret("parent-of-existing"), newParentSecret("parent-of-created"), newRelatedConfigMap("existing"))
	f.rm.customizeCache.Set(customizeKey{"uid-parent-of-existing", 0}, responseWithRelatedName("existing"))
	f.rm.customizeCache.Set(customizeKey{"uid-parent-of-created", 0}, responseWithRelatedName("created"))
	// No parent waits for the related informer, so only the related handler
	// can enqueue a parent.

	_, _, err := f.rm.getRelatedClient("v1", resourceConfigMaps)

	if err != nil && !errors.Is(err, ErrRelatedInformerNotSynced) {
		t.Fatalf("can't get related client: %v", err)
	}
	if !waitForSync(ctx, f.rm.relatedInformers.Get(schema.GroupVersionResource{Version: "v1", Resource: resourceConfigMaps})) {
		t.Fatal("timed out waiting for related informer sync")
	}
	if _, err := f.dynamicClient.Resource(schema.GroupVersionResource{Version: "v1", Resource: resourceConfigMaps}).Namespace(someNS).Create(ctx, newRelatedConfigMap("created"), metav1.CreateOptions{}); err != nil {
		t.Fatalf("can't create related object: %v", err)
	}
	// The events of one handler come in order. So when the parent of the new
	// object is enqueued, the handler has already got the first list.
	f.enqueued.waitFor(ctx, t, "parent-of-created")
	if got, want := f.enqueued.names(), []string{"parent-of-created"}; !slices.Equal(got, want) {
		t.Errorf("expected parents %v to be enqueued, got %v", want, got)
	}
}

// wakeupFixture is a customize manager with one parent resource (Secrets)
// and one related resource (ConfigMaps) on a fake API.
type wakeupFixture struct {
	rm *Manager
	// syncedInformer is an informer whose cache is filled.
	syncedInformer *dynamicinformer.ResourceInformer
	dynamicClient  *fake.FakeDynamicClient
	enqueued       *enqueuedParents
	t              *testing.T
}

// wakeupResourceMap returns one discovery map for the wake-up tests. The fake
// discovery helper sleeps for one second, so do not call it in each test.
var wakeupResourceMap = sync.OnceValue(func() *dynamicdiscovery.ResourceMap {
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{Name: resourceSecrets, Kind: kindSecret, Namespaced: true, Group: "", Version: "v1", Verbs: []string{"get", "list", "watch"}},
				{Name: resourceConfigMaps, Kind: kindConfigMap, Namespaced: true, Group: "", Version: "v1", Verbs: []string{"get", "list", "watch"}},
			},
		},
	}
	return discovery.NewFakeResourceMap(simple)
})

// newWakeupFixture creates a manager whose parent informer holds the Secrets
// in objects, and whose related informers watch the ConfigMaps in objects.
func newWakeupFixture(ctx context.Context, t *testing.T, objects ...runtime.Object) *wakeupFixture {
	t.Helper()

	dynamicClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		{Group: "", Version: "v1", Resource: resourceSecrets}:    "SecretList",
		{Group: "", Version: "v1", Resource: resourceConfigMaps}: "ConfigMapList",
	}, objects...)
	dynClient := dynamicclientset.NewClientset(&rest.Config{}, wakeupResourceMap(), dynamicClient)
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, 0)
	parentInformer, err := dynInformers.Resource(ctx, "v1", resourceSecrets)
	if err != nil {
		t.Fatalf("can't create parent informer: %v", err)
	}
	t.Cleanup(parentInformer.Close)
	if !waitForSync(ctx, parentInformer) {
		t.Fatal("timed out waiting for parent informer sync")
	}
	parentInformers := common.NewInformerMap()
	parentInformers.Set(schema.GroupVersionResource{Group: "", Version: "v1", Resource: resourceSecrets}, parentInformer)
	parentKinds := common.NewGroupKindMap()
	parentKinds.Set(schema.GroupKind{Group: "", Kind: kindSecret}, &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: resourceSecrets, Namespaced: true, Kind: kindSecret, Group: "", Version: "v1"},
	})
	enqueued := &enqueuedParents{}
	rm := &Manager{
		parentKinds:      parentKinds,
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		parentInformers:  parentInformers,
		relatedInformers: common.NewInformerMap(),
		customizeCache:   newResponseCache(),
		customizeHook:    NewHookExecutorStub(&v1.CustomizeHookResponse{}),
		enqueueParent:    enqueued.add,
		logger:           fakeLogger,
		ctx:              ctx,
		informerCtx:      ctx,
	}
	t.Cleanup(rm.Stop)

	return &wakeupFixture{rm: rm, syncedInformer: parentInformer, dynamicClient: dynamicClient, enqueued: enqueued, t: t}
}

// holdRelatedList makes each list of the related resource wait, so the cache
// of a related informer stays empty. Call the returned function to release
// the lists. The test cleanup also releases them.
func (f *wakeupFixture) holdRelatedList() func() {
	release := make(chan struct{})
	var once sync.Once
	releaseList := func() { once.Do(func() { close(release) }) }
	f.dynamicClient.PrependReactor("list", resourceConfigMaps, func(clienttesting.Action) (bool, runtime.Object, error) {
		<-release
		return false, nil, nil
	})
	f.t.Cleanup(releaseList)
	return releaseList
}

// enqueuedParents records the parents that the manager adds to the work queue.
type enqueuedParents struct {
	mutex   sync.Mutex
	parents []string
}

func (e *enqueuedParents) add(obj interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.parents = append(e.parents, obj.(*unstructured.Unstructured).GetName())
}

func (e *enqueuedParents) names() []string {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	return slices.Clone(e.parents)
}

// waitFor waits until the named parent is in the work queue.
func (e *enqueuedParents) waitFor(ctx context.Context, t *testing.T, name string) {
	t.Helper()

	for !slices.Contains(e.names(), name) {
		select {
		case <-ctx.Done():
			t.Fatalf("parent %q was not enqueued, the enqueued parents are %v", name, e.names())
		case <-time.After(5 * time.Millisecond):
		}
	}
}

func newParentSecret(name string) *unstructured.Unstructured {
	secret := &unstructured.Unstructured{}
	secret.SetAPIVersion("v1")
	secret.SetKind(kindSecret)
	secret.SetNamespace(someNS)
	secret.SetName(name)
	secret.SetUID(types.UID("uid-" + name))
	return secret
}

func newRelatedConfigMap(name string) *unstructured.Unstructured {
	configMap := &unstructured.Unstructured{}
	configMap.SetAPIVersion("v1")
	configMap.SetKind(kindConfigMap)
	configMap.SetNamespace(someNS)
	configMap.SetName(name)
	return configMap
}

// responseWithRelatedRule returns a customize response with one rule that
// selects all objects of the resource.
func responseWithRelatedRule(apiVersion, resource string) *v1.CustomizeHookResponse {
	return &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV1,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{{
			ResourceRule: v1alpha1.ResourceRule{APIVersion: apiVersion, Resource: resource},
		}},
	}
}

// responseWithRelatedName returns a customize response with one rule that
// selects the ConfigMap with the given name.
func responseWithRelatedName(name string) *v1.CustomizeHookResponse {
	response := responseWithRelatedRule("v1", resourceConfigMaps)
	response.RelatedResourceRules[0].Namespace = someNS
	response.RelatedResourceRules[0].Names = []string{name}
	return response
}

// wakeupCounterValue reads a wake-up counter of one controller from the
// metrics registry.
func wakeupCounterValue(t *testing.T, name, controllerName string) float64 {
	t.Helper()

	families, err := controllerruntimemetrics.Registry.Gather()
	if err != nil {
		t.Fatalf("can't gather metrics: %v", err)
	}
	for _, family := range families {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			for _, label := range metric.GetLabel() {
				if label.GetName() == "controller_name" && label.GetValue() == controllerName {
					return metric.GetCounter().GetValue()
				}
			}
		}
	}
	return 0
}

func TestGetRelatedObjects_ErrorWhenNamespaceSelectorForClusterScopedResource(t *testing.T) {
	// Setup fake discovery
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       resourceClusterRoles,
					Kind:       "ClusterRole",
					Namespaced: false,
					Group:      rbacGroup,
					Version:    "v1",
				},
			},
		},
		{
			GroupVersion: "v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "namespaces",
					Kind:       "Namespace",
					Namespaced: false,
					Group:      "",
					Version:    "v1",
				},
			},
		},
	}
	resourceMap := discovery.NewFakeResourceMap(simple)

	// Setup fake dynamic client
	scheme := runtime.NewScheme()
	fakeDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
		{Group: "", Version: "v1", Resource: "namespaces"}:                            "NamespaceList",
	})
	dynClient := dynamicclientset.NewClientset(&rest.Config{}, resourceMap, fakeDynClient)

	// Setup informers
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, 0)
	nsInformer, _ := dynInformers.Resource(context.TODO(), "v1", "namespaces")
	defer nsInformer.Close()

	// Start informers and wait for sync
	stopCh := make(chan struct{})
	defer close(stopCh)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if !waitForSync(ctx, nsInformer) {
		t.Fatal("Timed out waiting for namespace informer sync")
	}

	// Create manager
	parentGK := schema.GroupKind{Group: groupTest, Kind: kindParent}
	parentResource := &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: "parents", Namespaced: true, Kind: kindParent, Group: groupTest, Version: "v1"},
	}
	parentKinds := common.NewGroupKindMap()
	parentKinds.Set(parentGK, parentResource)

	rm := &Manager{
		controller:       &v1alpha1.CompositeController{},
		parentKinds:      parentKinds,
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		nsInformer:       nsInformer,
		parentInformers:  common.NewInformerMap(),
		relatedInformers: common.NewInformerMap(),
		logger:           fakeLogger,
		customizeCache:   newResponseCache(),
	}

	// Setup customize hook response
	expectedResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV2,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{
			{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: rbacAPIVersion,
					Resource:   resourceClusterRoles,
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{labelKeyEnv: labelValueProd}},
			},
		},
	}
	rm.customizeHook = NewHookExecutorStub(expectedResponse)
	rm.Start(ctx)

	// Trigger creation and wait for sync of the related informer
	for {
		_, informer, err := rm.getRelatedClient("rbac.authorization.k8s.io/v1", resourceClusterRoles)
		if err == nil {
			if informer.Informer().HasSynced() {
				break
			}
		} else if !errors.Is(err, ErrRelatedInformerNotSynced) {
			t.Fatalf("Failed to get related client: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Fatal("Timed out waiting for related informer sync")
		default:
		}
	}

	// Create parent object
	parent := &unstructured.Unstructured{}
	parent.SetAPIVersion("test/v1")
	parent.SetKind("Parent")
	parent.SetName("test-parent")
	parent.SetNamespace("test-ns")
	parent.SetUID("123")
	parent.SetGeneration(1)

	// Call GetRelatedObjects
	_, err := rm.GetRelatedObjects(ctx, parent)

	if err == nil {
		t.Fatal("Expected error when using namespaceSelector for cluster-scoped resource, but got nil")
	}

	expectedErr := "namespaceSelector is only supported for namespaced related resources"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestGetRelatedObjects_IgnoreNamespaceForClusterScopedResource(t *testing.T) {
	// Setup fake discovery
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       resourceClusterRoles,
					Kind:       "ClusterRole",
					Namespaced: false,
					Group:      rbacGroup,
					Version:    "v1",
					Verbs:      []string{"get", "list", "watch"},
				},
			},
		},
	}
	resourceMap := discovery.NewFakeResourceMap(simple)

	// Setup fake dynamic client with some objects
	scheme := runtime.NewScheme()
	clusterRole := &unstructured.Unstructured{}
	clusterRole.SetAPIVersion("rbac.authorization.k8s.io/v1")
	clusterRole.SetKind("ClusterRole")
	clusterRole.SetName("test-clusterrole")
	clusterRole.SetLabels(map[string]string{"app": "test"})

	fakeDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(scheme, map[schema.GroupVersionResource]string{
		{Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"}: "ClusterRoleList",
	}, clusterRole)
	dynClient := dynamicclientset.NewClientset(&rest.Config{}, resourceMap, fakeDynClient)

	// Setup informers
	dynInformers := dynamicinformer.NewSharedInformerFactory(dynClient, 0)

	// Start informers and wait for sync
	stopCh := make(chan struct{})
	defer close(stopCh)

	// Create manager
	parentGK := schema.GroupKind{Group: groupTest, Kind: kindParent}
	parentResource := &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: "parents", Namespaced: true, Kind: kindParent, Group: groupTest, Version: "v1"},
	}
	parentKinds := common.NewGroupKindMap()
	parentKinds.Set(parentGK, parentResource)

	rm := &Manager{
		controller:       &v1alpha1.CompositeController{},
		parentKinds:      parentKinds,
		dynClient:        dynClient,
		dynInformers:     dynInformers,
		parentInformers:  common.NewInformerMap(),
		relatedInformers: common.NewInformerMap(),
		logger:           fakeLogger,
		customizeCache:   newResponseCache(),
	}

	// Setup customize hook response with explicit namespace for cluster-scoped resource
	expectedResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV2,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{
			{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: rbacAPIVersion,
					Resource:   resourceClusterRoles,
				},
				Namespace:     "some-namespace", // This should be ignored
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
			},
		},
	}
	rm.customizeHook = NewHookExecutorStub(expectedResponse)

	// Trigger creation and wait for sync of the related informer
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	rm.Start(ctx)
	for {
		_, informer, err := rm.getRelatedClient("rbac.authorization.k8s.io/v1", resourceClusterRoles)
		if err == nil {
			if informer.Informer().HasSynced() {
				break
			}
		} else if !errors.Is(err, ErrRelatedInformerNotSynced) {
			t.Fatalf("Failed to get related client: %v", err)
		}
		time.Sleep(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Fatal("Timed out waiting for related informer sync")
		default:
		}
	}

	// Create parent object
	parent := &unstructured.Unstructured{}
	parent.SetAPIVersion("test/v1")
	parent.SetKind("Parent")
	parent.SetName("test-parent")
	parent.SetNamespace("test-ns")
	parent.SetUID("123")
	parent.SetGeneration(1)

	// Call GetRelatedObjects
	relatedObjects, err := rm.GetRelatedObjects(ctx, parent)
	if err != nil {
		t.Fatalf("Expected no error, but got: %v", err)
	}

	// Verify that the clusterrole was found despite the namespace being specified
	list := relatedObjects.List()
	if len(list) != 1 {
		t.Errorf("Expected 1 related object, got %d", len(list))
	} else if list[0].GetName() != "test-clusterrole" {
		t.Errorf("Expected test-clusterrole, got %s", list[0].GetName())
	}
}

func waitForSync(ctx context.Context, informer *dynamicinformer.ResourceInformer) bool {
	for {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(100 * time.Millisecond):
			if informer.Informer().HasSynced() {
				return true
			}
		}
	}
}
