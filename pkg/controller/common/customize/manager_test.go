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
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr/funcr"

	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"metacontroller/pkg/internal/testutils/dynamic/discovery"
	. "metacontroller/pkg/internal/testutils/hooks"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	"k8s.io/client-go/dynamic/fake"
	fakeclientset "k8s.io/client-go/kubernetes/fake"
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
				rc.SetName("name")
				return rc
			}(),
			relatedRule: &v1alpha1.RelatedResourceRule{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Resource:   "clusterroles",
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			},
			relatedRuleKind: "ClusterRole",
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
				nsInformer, _ = tt.dynInformers.Resource("v1", "namespaces")
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

func TestGetRelatedObjects_ErrorWhenNamespaceSelectorForClusterScopedResource(t *testing.T) {
	// Setup fake discovery
	simple := fakeclientset.NewClientset()
	fakeDiscovery := simple.Discovery().(*fakediscovery.FakeDiscovery)
	fakeDiscovery.Resources = []*metav1.APIResourceList{
		{
			GroupVersion: "rbac.authorization.k8s.io/v1",
			APIResources: []metav1.APIResource{
				{
					Name:       "clusterroles",
					Kind:       "ClusterRole",
					Namespaced: false,
					Group:      "rbac.authorization.k8s.io",
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
	nsInformer, _ := dynInformers.Resource("v1", "namespaces")
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
	parentGK := schema.GroupKind{Group: "test", Kind: "Parent"}
	parentResource := &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: "parents", Namespaced: true, Kind: "Parent", Group: "test", Version: "v1"},
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
		stopCh:           stopCh,
		customizeCache:   newResponseCache(),
	}

	// Setup customize hook response
	expectedResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV2,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{
			{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Resource:   "clusterroles",
				},
				NamespaceSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"env": "prod"}},
			},
		},
	}
	rm.customizeHook = NewHookExecutorStub(expectedResponse)

	// Trigger creation and wait for sync of the related informer
	for {
		_, informer, err := rm.getRelatedClient("rbac.authorization.k8s.io/v1", "clusterroles")
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
	_, err := rm.GetRelatedObjects(parent)

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
					Name:       "clusterroles",
					Kind:       "ClusterRole",
					Namespaced: false,
					Group:      "rbac.authorization.k8s.io",
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
	parentGK := schema.GroupKind{Group: "test", Kind: "Parent"}
	parentResource := &dynamicdiscovery.APIResource{
		APIResource: metav1.APIResource{Name: "parents", Namespaced: true, Kind: "Parent", Group: "test", Version: "v1"},
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
		stopCh:           stopCh,
		customizeCache:   newResponseCache(),
	}

	// Setup customize hook response with explicit namespace for cluster-scoped resource
	expectedResponse := &v1.CustomizeHookResponse{
		Version: v1alpha1.HookVersionV2,
		RelatedResourceRules: []*v1alpha1.RelatedResourceRule{
			{
				ResourceRule: v1alpha1.ResourceRule{
					APIVersion: "rbac.authorization.k8s.io/v1",
					Resource:   "clusterroles",
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
	for {
		_, informer, err := rm.getRelatedClient("rbac.authorization.k8s.io/v1", "clusterroles")
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
	relatedObjects, err := rm.GetRelatedObjects(parent)

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
