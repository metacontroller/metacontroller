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
	"fmt"
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"reflect"
	"testing"

	"github.com/go-logr/logr/funcr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"metacontroller/pkg/internal/testutils/dynamic/discovery"
	. "metacontroller/pkg/internal/testutils/hooks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
)

var fakeEnqueueParent = func(obj interface{}) {}
var dynClient = dynamicclientset.Clientset{}
var dynInformers = dynamicinformer.SharedInformerFactory{}

var fakeLogger = funcr.New(
	func(pfx, args string) { fmt.Println(pfx, args) },
	funcr.Options{
		LogCaller:    funcr.All,
		LogTimestamp: true,
	})

var customizeManagerWithNilController, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&NilCustomizableController{},
	&dynClient,
	&dynInformers,
	common.NewInformerMap(),
	common.NewGroupKindMap(),
	fakeLogger,
	common.CompositeController,
)

var customizeManagerWithFakeController, _ = NewCustomizeManager(
	"test",
	fakeEnqueueParent,
	&FakeCustomizableController{},
	&dynClient,
	&dynInformers,
	common.NewInformerMap(),
	common.NewGroupKindMap(),
	fakeLogger,
	common.DecoratorController,
)

func TestGetRelatedObjects_whenHookDisabled_returnEmptyMap(t *testing.T) {
	parent := &unstructured.Unstructured{}
	parent.SetName("test")
	parent.SetGeneration(1)

	relatedObjects, err := customizeManagerWithNilController.GetRelatedObjects(parent)

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
				APIVersion: "some",
				Resource:   "some",
			},
			LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			Namespace:     "Namespace",
			Names:         []string{"name"},
		}},
	}

	customizeManagerWithFakeController.customizeHook = NewHookExecutorStub(expectedResponse)
	parent := &unstructured.Unstructured{}
	parent.SetUID("some")
	parent.SetGeneration(1)

	response, err := customizeManagerWithFakeController.getCustomizeHookResponse(parent)

	if err != nil {
		t.Errorf("Incorrect invocation, err should be nil, got: %v", err)
	}

	if !reflect.DeepEqual(*response, *expectedResponse) {
		t.Errorf("Response should be equal to %v, got %v", expectedResponse, response)
	}

	if _, found := customizeManagerWithFakeController.customizeCache.Get(customizeKey{"some", 1}); !found {
		t.Error("Expected not nil here, response should be cached")
	}
}

func TestDetermineSelectionType_returnNamespaceAndLabelsWhenLabelSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
		Namespace:     "Namespace",
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndLabels, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceSelectorWhenPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceSelector || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceSelector, selectionType)
	}
}

func TestDetermineSelectionType_returnErrorWhenNamespaceSelectorAndNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
		Namespace:         "some-ns",
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid || err == nil {
		t.Errorf("Expected invalid selection type due to combining namespace and namespaceSelector")
	}
}

func TestDetermineSelectionType_returnErrorWhenNamespaceSelectorAndNamesIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
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
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
		Names:         []string{"name"},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != invalid && err == nil {
		t.Errorf("Expected error and 'invalid' selection type, but got %v", selectionType)
	}
}

func TestDetermineSelectionType_returnLabelSelectorWhenPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamespaceIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Namespace:     "some",
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnNamespaceAndNamesWhenNamesIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
		Names:         []string{"name"},
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByNamespaceAndNames || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByNamespaceAndNames, selectionType)
	}
}

func TestDetermineSelectionType_returnLabelsWhenNothingIsPresent(t *testing.T) {
	resourceRule := v1alpha1.RelatedResourceRule{
		ResourceRule: v1alpha1.ResourceRule{
			APIVersion: "some",
			Resource:   "some",
		},
		LabelSelector: nil,
	}

	selectionType, err := determineSelectionType(&resourceRule)

	if selectionType != selectByLabels || err != nil {
		t.Errorf("Expected %v selection type, but got %v", selectByLabels, selectionType)
	}
}

func Test_matchRelatedRule(t *testing.T) {
	var fakeGenericParent = func() *unstructured.Unstructured {
		p := &unstructured.Unstructured{}
		p.SetAPIVersion("other")
		p.SetKind("other")
		return p
	}
	var fakeGenericParentWithNamespace = func() *unstructured.Unstructured {
		p := fakeGenericParent()
		p.SetNamespace("some")
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
				rc.SetKind("Secret")
				rc.SetLabels(map[string]string{"aaa": "bbb"})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			},
			relatedRuleKind: "Secret",
			wantMatch:       true,
		},
		{
			name:        "return false when labels do not match",
			hookVersion: v1alpha1.HookVersionV1,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetLabels(map[string]string{"aaa": "cbb"})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			},
			relatedRuleKind: "Secret",
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
					Resource:   "secrets",
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
			},
			relatedRuleKind: "Secret",
			wantMatch:       false,
		},
		{
			name:        "return true if labels and namespace match",
			hookVersion: v1alpha1.HookVersionV2,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind("Secret")
				rc.SetNamespace("some-ns")
				rc.SetLabels(map[string]string{"aaa": "bbb"})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
				Namespace:     "some-ns",
			},
			relatedRuleKind: "Secret",
			wantMatch:       true,
		},
		{
			name:        "return false if labels match but namespace does not match",
			hookVersion: v1alpha1.HookVersionV2,
			parent:      fakeGenericParent(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("v1")
				rc.SetKind("Secret")
				rc.SetNamespace("other-ns")
				rc.SetLabels(map[string]string{"aaa": "bbb"})
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"aaa": "bbb"}},
				Namespace:     "some-ns",
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("some")
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
			wantMatch:       true,
		},
		{
			name:         "return false when parent is namespace scoped and name matches but namespace does not match",
			hookVersion:  v1alpha1.HookVersionV1,
			isNamespaced: true,
			parent:       fakeGenericParentWithNamespace(),
			related: func() *unstructured.Unstructured {
				rc := &unstructured.Unstructured{}
				rc.SetAPIVersion("some")
				rc.SetKind("Secret")
				rc.SetNamespace("other")
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("some")
				rc.SetName("othername")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("some")
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("other")
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("some")
				rc.SetName("othername")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "some",
				Names:         []string{"name"},
			},
			relatedRuleKind: "Secret",
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
					Resource:   "namespaces",
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
				rc.SetKind("Secret")
				rc.SetNamespace("other-namespace")
				rc.SetName("some-secret")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "other-namespace",
				Names:         []string{"some-secret"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("other-namespace")
				rc.SetName("some-secret")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				LabelSelector: nil,
				Namespace:     "yet-another-namespace",
				Names:         []string{"some-secret"},
			},
			relatedRuleKind: "Secret",
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
				rc.SetKind("Secret")
				rc.SetNamespace("some")
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "v1",
					Resource:   "secrets",
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			},
			relatedRuleKind: "Secret",
			wantMatch:       false,
			wantErr:         true,
			dynInformers: dynamicinformer.NewSharedInformerFactory(
				dynamicclientset.NewClientset(&rest.Config{}, discovery.NewFakeResourceMap(fake.NewSimpleClientset()), nil),
				0,
			),
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
				nsInformer, _ = tt.dynInformers.Resource("v1", "namespaces")
			}
			rm := &Manager{
				dynInformers: tt.dynInformers,
				nsInformer:   nsInformer,
			}
			matches, err := rm.matchesRelatedRule(tt.hookVersion, tt.isNamespaced, tt.parent, tt.related, tt.relatedRule, tt.relatedRuleKind)
			if err != nil && !tt.wantErr {
				t.Error(err)
			}
			if matches != tt.wantMatch {
				t.Errorf("Expected match: %v, actual match: %v", tt.wantMatch, matches)
			}
		})
	}
}
