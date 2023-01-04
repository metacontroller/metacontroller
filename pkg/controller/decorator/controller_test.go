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
		"name",
		func(obj interface{}) {},
		&NilCustomizableController{},
		&dynamicclientset.Clientset{},
		&dynamicinformer.SharedInformerFactory{},
		make(common.InformerMap),
		make(common.GroupKindMap),
		logging.Logger,
		common.DecoratorController,
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

	simpleClientset := NewFakeNewSimpleClientsetWithResources(apiResourceList)
	resourceMap := NewFakeResourceMap(simpleClientset)
	restConfig := NewDefaultRestConfig()
	testClientset := NewClientset(restConfig, resourceMap, simpleDynClient)
	parentResourceClient, _ := testClientset.Resource(TestAPIVersion, TestResource)
	informerFactory := dynamicinformer.NewSharedInformerFactory(testClientset, 5*time.Minute)
	resourceInformer, _ := informerFactory.Resource(TestAPIVersion, TestResource)
	if syncCache && !cache.WaitForNamedCacheSync("controllerName", NewCh(), resourceInformer.Informer().HasSynced) {
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

var defaultGroupKindMap = map[schema.GroupKind]*dynamicdiscovery.APIResource{
	{Group: TestGroup, Kind: TestKind}: &DefaultApiResource,
}

var defaultSelectorKey = fmt.Sprintf("%s.%s", TestKind, TestGroup)
var defaultLabels = map[string]string{"key": "val"}
var defaultSelector = map[string]labels.Selector{
	defaultSelectorKey: labels.SelectorFromSet(defaultLabels),
}
var defaultParentSelector = &decoratorSelector{
	labelSelectors:      defaultSelector,
	annotationSelectors: defaultSelector,
}

func newUnstructuredWithSelectors() *unstructured.Unstructured {
	defaultUnstructured := NewDefaultUnstructured()
	defaultUnstructured.SetLabels(defaultLabels)
	defaultUnstructured.SetAnnotations(defaultLabels)
	return defaultUnstructured
}

func makeDefaultManagingControllerMap(controller bool) map[string]bool {
	m := make(map[string]bool)
	key := fmt.Sprintf("%s.%s", TestKind, TestGroup)
	m[key] = controller
	return m
}

func Test_decoratorController_sync(t *testing.T) {
	logging.InitLogging(&zap.Options{})
	type fields struct {
		dc                 *v1alpha1.DecoratorController
		parentKinds        common.GroupKindMap
		parentSelector     *decoratorSelector
		stopCh             chan struct{}
		doneCh             chan struct{}
		queue              workqueue.RateLimitingInterface
		managingController map[string]bool
		updateStrategy     updateStrategyMap
		childInformers     common.InformerMap
		numWorkers         int
		eventRecorder      record.EventRecorder
		finalizer          *finalizer.Manager
		customize          *customize.Manager
		syncHook           hooks.Hook
		finalizeHook       hooks.Hook
		logger             logr.Logger
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true, false)
			},
			fields: fields{
				dc:             newDefaultDecoratorController(),
				parentKinds:    defaultGroupKindMap,
				parentSelector: defaultParentSelector,
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
				stopCh:         NewCh(),
				doneCh:         NewCh(),
				queue:          NewDefaultWorkQueue(),
				updateStrategy: nil,
				childInformers: nil,
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
			_, resources, dynClient, _, parentInformers := tt.clientsAndInformers()
			c := &decoratorController{
				dc:              tt.fields.dc,
				resources:       resources,
				parentKinds:     tt.fields.parentKinds,
				parentSelector:  tt.fields.parentSelector,
				dynClient:       dynClient,
				stopCh:          tt.fields.stopCh,
				doneCh:          tt.fields.doneCh,
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
			if err := c.sync(tt.args.key); (err != nil) != tt.wantErr {
				t.Errorf("sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
