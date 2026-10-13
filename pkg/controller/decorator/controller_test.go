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

package decorator

import (
	"context"
	"encoding/json"
	"fmt"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/customize"
	"metacontroller/pkg/controller/common/finalizer"
	v1 "metacontroller/pkg/controller/decorator/api/v1"
	dynamicclientset "metacontroller/pkg/dynamic/clientset"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"
	"metacontroller/pkg/hooks"
	. "metacontroller/pkg/internal/testutils/common"
	. "metacontroller/pkg/internal/testutils/dynamic/clientset"
	. "metacontroller/pkg/internal/testutils/dynamic/discovery"
	. "metacontroller/pkg/internal/testutils/hooks"
	"metacontroller/pkg/logging"
	"reflect"
	"testing"
	"time"

	"github.com/go-logr/logr"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/fake"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func defaultCustomizeManager() *customize.Manager {
	customizeManager, _ := customize.NewCustomizeManager(
		context.TODO(),
		"name",
		func(obj interface{}) {},
		&NilCustomizableController{},
		&dynamicclientset.Clientset{},
		&dynamicinformer.SharedInformerFactory{},
		common.NewInformerMap(),
		common.NewGroupKindMap(),
		logging.Logger,
		common.DecoratorController,
		nil,
	)
	return customizeManager
}

var defaultSyncResponse = &v1.DecoratorHookResponse{
	Status:             nil,
	ResyncAfterSeconds: 0,
	Finalized:          false,
}

var changedStatusSyncResponse = &v1.DecoratorHookResponse{
	ResyncAfterSeconds: 0,
	Finalized:          false,
	Status: map[string]interface{}{
		"changed": "true",
	},
}

func newDefaultControllerClientsAndInformers(fakeDynamicClientFn func(client *fake.FakeDynamicClient), syncCache bool, hasStatusSubresource bool) (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: TestGroup, Version: TestVersion, Resource: TestResource}: TestResourceList,
	}

	simpleDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, newUnstructuredWithSelectors())
	fakeDynamicClientFn(simpleDynClient)

	var apiResourceList []*metav1.APIResourceList
	if hasStatusSubresource {
		apiResourceList = NewDefaultStatusAPIResourceList()
	} else {
		apiResourceList = NewDefaultAPIResourceList()
	}

	simpleClientset := NewFakeClientsetWithResources(apiResourceList)
	resourceMap := NewFakeResourceMap(simpleClientset)
	restConfig := NewDefaultRestConfig()
	testClientset := NewClientset(restConfig, resourceMap, simpleDynClient)
	parentResourceClient, _ := testClientset.Resource(TestAPIVersion, TestResource)
	informerFactory := dynamicinformer.NewSharedInformerFactory(testClientset, 5*time.Minute)
	resourceInformer, _ := informerFactory.Resource(context.TODO(), TestAPIVersion, TestResource)
	stopCh := make(chan struct{})
	time.AfterFunc(1*time.Second, func() { close(stopCh) })
	if syncCache && !cache.WaitForNamedCacheSync("controllerName", stopCh, resourceInformer.Informer().HasSynced) {
		panic("could not sync resource informer cache")
	}
	resourceInformers := map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer{
		{Group: TestGroup, Version: TestVersion, Resource: TestResource}: resourceInformer,
	}
	return simpleDynClient, resourceMap, testClientset, parentResourceClient, resourceInformers
}

var defaultTestKey = fmt.Sprintf("%s:%s:%s:%s", TestAPIVersion, TestKind, TestNamespace, TestName)

func newDefaultDecoratorController() *v1alpha1.DecoratorController {
	return &v1alpha1.DecoratorController{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1alpha1.DecoratorControllerSpec{
			Hooks: &v1alpha1.DecoratorControllerHooks{
				Sync: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL:     nil,
						Timeout: nil,
						Path:    nil,
						Service: nil,
					},
				},
				Finalize: &v1alpha1.Hook{
					Webhook: &v1alpha1.Webhook{
						URL:     nil,
						Timeout: nil,
						Path:    nil,
						Service: nil,
					},
				},
			},
		},
		Status: v1alpha1.DecoratorControllerStatus{},
	}
}

var defaultGroupKindMap = func() *common.GroupKindMap {
	m := common.NewGroupKindMap()
	m.Set(schema.GroupKind{Group: TestGroup, Kind: TestKind}, &DefaultApiResource)
	return m
}()

var defaultSelectorKey = fmt.Sprintf("%s.%s", TestKind, TestGroup)
var defaultLabels = map[string]string{"key": "val"}
var defaultSelector = labels.SelectorFromSet(defaultLabels)
var defaultParentSelector = func() *decoratorSelector {
	ds := &decoratorSelector{}
	ds.labelSelectors.Store(defaultSelectorKey, defaultSelector)
	ds.annotationSelectors.Store(defaultSelectorKey, defaultSelector)
	return ds
}()

func newUnstructuredWithSelectors() *unstructured.Unstructured {
	defaultUnstructured := NewDefaultUnstructured()
	defaultUnstructured.SetLabels(defaultLabels)
	defaultUnstructured.SetAnnotations(defaultLabels)
	return defaultUnstructured
}

func Test_decoratorController_sync(t *testing.T) {
	logging.InitLogging(&zap.Options{})
	type fields struct {
		dc             *v1alpha1.DecoratorController
		parentKinds    *common.GroupKindMap
		parentSelector *decoratorSelector
		queue          workqueue.TypedRateLimitingInterface[string]
		updateStrategy updateStrategyMap
		childInformers *common.InformerMap
		numWorkers     int
		eventRecorder  record.EventRecorder
		finalizer      *finalizer.Manager
		customize      *customize.Manager
		syncHook       hooks.Hook
		finalizeHook   hooks.Hook
		logger         logr.Logger
	}
	type args struct {
		key string
	}
	tests := []struct {
		name                string
		clientsAndInformers func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer)
		fields              fields
		args                args
		wantErr             bool
	}{
		{
			name: "no error on successful sync",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("list", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						result := unstructured.UnstructuredList{
							Object: make(map[string]interface{}),
							Items: []unstructured.Unstructured{
								*newUnstructuredWithSelectors(),
							},
						}
						return true, &result, nil
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(defaultSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "no error on sync with not found api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				return newDefaultControllerClientsAndInformers(NoOpFn, false, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(defaultSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "no error on update parent with not found api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "no error on update status parent with not found api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, true)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "no error on update parent with conflict api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewConflict(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName, nil)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "no error on update status parent with conflict api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewConflict(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName, nil)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, true)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args: args{key: defaultTestKey},
		},
		{
			name: "error on update parent with unexpected api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args:    args{key: defaultTestKey},
			wantErr: true,
		},
		{
			name: "error on update status parent with unexpected api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, map[schema.GroupVersionResource]*dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, true)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: common.NewInformerMap(),
				numWorkers:     1,
				eventRecorder:  NewFakeRecorder(),
				finalizer:      DefaultFinalizerManager,
				customize:      defaultCustomizeManager(),
				syncHook:       NewHookExecutorStub(changedStatusSyncResponse),
				finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
				logger:         logging.Logger,
			},
			args:    args{key: defaultTestKey},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, resources, dynClient, _, parentInformersMap := tt.clientsAndInformers()
			parentInformers := common.NewInformerMap()
			for gvr, informer := range parentInformersMap {
				parentInformers.Set(gvr, informer)
			}
			c := &decoratorController{
				dc:              tt.fields.dc,
				resources:       resources,
				parentKinds:     tt.fields.parentKinds,
				parentSelector:  tt.fields.parentSelector,
				dynClient:       dynClient,
				queue:           tt.fields.queue,
				updateStrategy:  tt.fields.updateStrategy,
				parentInformers: parentInformers,
				childInformers:  tt.fields.childInformers,
				numWorkers:      tt.fields.numWorkers,
				eventRecorder:   tt.fields.eventRecorder,
				finalizer:       tt.fields.finalizer,
				customize:       tt.fields.customize,
				syncHook:        tt.fields.syncHook,
				finalizeHook:    tt.fields.finalizeHook,
				logger:          tt.fields.logger,
			}
			if err := c.sync(context.TODO(), tt.args.key); (err != nil) != tt.wantErr {
				t.Errorf("sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// Test_decoratorController_syncPatchBody asserts the exact merge-patch bodies
// the decorator controller sends when syncing a parent.
func Test_decoratorController_syncPatchBody(t *testing.T) {
	logging.InitLogging(&zap.Options{})

	tests := []struct {
		name                 string
		hasStatusSubresource bool
		syncResponse         *v1.DecoratorHookResponse
		assert               func(t *testing.T, patch map[string]interface{})
	}{
		{
			name:                 "status without subresource is patched at top level, not under metadata",
			hasStatusSubresource: false,
			syncResponse:         changedStatusSyncResponse,
			assert: func(t *testing.T, patch map[string]interface{}) {
				status, ok := patch["status"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want top-level status object, got %v", patch)
				}
				if !reflect.DeepEqual(status, map[string]interface{}{"changed": "true"}) {
					t.Errorf("patch body: want status {\"changed\":\"true\"}, got %v", status)
				}
				md, ok := patch["metadata"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want metadata object, got %v", patch)
				}
				if _, exists := md["status"]; exists {
					t.Errorf("patch body: status must not be nested under metadata, got %v", md)
				}
			},
		},
		{
			name:                 "status with subresource is not part of the main patch",
			hasStatusSubresource: true,
			syncResponse:         changedStatusSyncResponse,
			assert: func(t *testing.T, patch map[string]interface{}) {
				if _, exists := patch["status"]; exists {
					t.Errorf("patch body: want no top-level status when the parent has a status subresource, got %v", patch)
				}
			},
		},
		{
			name:                 "hook-requested label deletion is patched as explicit null",
			hasStatusSubresource: false,
			syncResponse: &v1.DecoratorHookResponse{
				Labels: map[string]*string{"key": nil},
			},
			assert: func(t *testing.T, patch map[string]interface{}) {
				md, ok := patch["metadata"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want metadata object, got %v", patch)
				}
				labels, ok := md["labels"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want labels in metadata, got %v", md)
				}
				if v, exists := labels["key"]; !exists || v != nil {
					t.Errorf("patch body: want labels.key = null (delete), got %v (patch: %v)", v, patch)
				}
			},
		},
		{
			name:                 "hook-requested label addition and modification are patched with new values",
			hasStatusSubresource: false,
			syncResponse: &v1.DecoratorHookResponse{
				Labels: func() map[string]*string {
					val := "newval"
					return map[string]*string{"key": &val, "added": &val}
				}(),
			},
			assert: func(t *testing.T, patch map[string]interface{}) {
				md, ok := patch["metadata"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want metadata object, got %v", patch)
				}
				labels, ok := md["labels"].(map[string]interface{})
				if !ok {
					t.Fatalf("patch body: want labels in metadata, got %v", md)
				}
				want := map[string]interface{}{"key": "newval", "added": "newval"}
				if !reflect.DeepEqual(labels, want) {
					t.Errorf("patch body: want labels %v, got %v", want, labels)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var patchBodies [][]byte
			fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
				fakeDynamicClient.PrependReactor("patch", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
					patchAction, ok := action.(clientgotesting.PatchAction)
					if !ok {
						return true, nil, fmt.Errorf("unexpected action type %T", action)
					}
					patchBodies = append(patchBodies, patchAction.GetPatch())
					return true, nil, nil
				})
			}
			_, resources, dynClient, _, parentInformersMap := newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, tt.hasStatusSubresource)
			parentInformers := common.NewInformerMap()
			for gvr, informer := range parentInformersMap {
				parentInformers.Set(gvr, informer)
			}
			c := &decoratorController{
				dc:              newDefaultDecoratorController(),
				resources:       resources,
				parentKinds:     defaultGroupKindMap,
				parentSelector:  defaultParentSelector,
				dynClient:       dynClient,
				queue:           NewDefaultWorkQueue(),
				updateStrategy:  nil,
				parentInformers: parentInformers,
				childInformers:  common.NewInformerMap(),
				numWorkers:      1,
				eventRecorder:   NewFakeRecorder(),
				finalizer:       DefaultFinalizerManager,
				customize:       defaultCustomizeManager(),
				syncHook:        NewHookExecutorStub(tt.syncResponse),
				finalizeHook:    NewHookExecutorStub(defaultSyncResponse),
				logger:          logging.Logger,
			}
			if err := c.sync(context.TODO(), defaultTestKey); err != nil {
				t.Fatalf("sync() error = %v", err)
			}
			if len(patchBodies) == 0 {
				t.Fatalf("sync() recorded no patch actions; want the parent patched")
			}
			var patch map[string]interface{}
			if err := json.Unmarshal(patchBodies[0], &patch); err != nil {
				t.Fatalf("can't parse patch body %s: %v", patchBodies[0], err)
			}
			tt.assert(t, patch)
		})
	}
}
