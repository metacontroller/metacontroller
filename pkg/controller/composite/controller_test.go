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

package composite

import (
	"fmt"
	"github.com/stretchr/testify/assert"
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/client/generated/clientset/internalclientset"
	mclisters "metacontroller/pkg/client/generated/lister/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common"
	"metacontroller/pkg/controller/common/customize"
	"metacontroller/pkg/controller/common/finalizer"
	composite "metacontroller/pkg/controller/composite/api/v1"
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
		common.CompositeController,
	)
	return customizeManager
}

var defaultSyncResponse = &composite.CompositeHookResponse{
	Status:             nil,
	Children:           nil,
	ResyncAfterSeconds: 0,
	Finalized:          false,
}

func newDefaultControllerClientsAndInformers(fakeDynamicClientFn func(client *fake.FakeDynamicClient), syncCache bool) (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
	gvrToListKind := map[schema.GroupVersionResource]string{
		{Group: TestGroup, Version: TestVersion, Resource: TestResource}: TestResourceList,
	}

	simpleDynClient := fake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), gvrToListKind, NewDefaultUnstructured())
	fakeDynamicClientFn(simpleDynClient)
	simpleClientset := NewFakeNewSimpleClientsetWithResources(NewDefaultAPIResourceList())
	resourceMap := NewFakeResourceMap(simpleClientset)
	restConfig := NewDefaultRestConfig()
	testClientset := NewClientset(restConfig, resourceMap, simpleDynClient)
	parentResourceClient, _ := testClientset.Resource(TestAPIVersion, TestResource)
	informerFactory := dynamicinformer.NewSharedInformerFactory(testClientset, 5*time.Minute)
	resourceInformer, _ := informerFactory.Resource(TestAPIVersion, TestResource)
	if syncCache && !cache.WaitForNamedCacheSync("controllerName", NewCh(), resourceInformer.Informer().HasSynced) {
		panic("could not sync resource informer cache")
	}
	return simpleDynClient, resourceMap, testClientset, parentResourceClient, resourceInformer
}

var defaultTestKey = fmt.Sprintf("%s/%s", TestNamespace, TestName)

func newDefaultCompositeController() *v1alpha1.CompositeController {
	generateSelector := true
	return &v1alpha1.CompositeController{
		TypeMeta:   metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{},
		Spec: v1alpha1.CompositeControllerSpec{
			GenerateSelector: &generateSelector,
			Hooks: &v1alpha1.CompositeControllerHooks{
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
		Status: v1alpha1.CompositeControllerStatus{},
	}
}

func Test_parentController_sync(t *testing.T) {
	logging.InitLogging(&zap.Options{})
	type fields struct {
		cc             *v1alpha1.CompositeController
		parentResource *dynamicdiscovery.APIResource
		mcClient       internalclientset.Interface
		revisionLister mclisters.ControllerRevisionLister
		stopCh         chan struct{}
		doneCh         chan struct{}
		queue          workqueue.RateLimitingInterface
		updateStrategy updateStrategyMap
		childInformers common.InformerMap
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
		clientsAndInformers func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer)
		fields              fields
		args                args
		wantErr             bool
	}{
		{
			name: "no error on successful sync",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
				return newDefaultControllerClientsAndInformers(ListFn, true)
			},
			fields: fields{
				cc:             newDefaultCompositeController(),
				parentResource: &DefaultApiResource,
				mcClient:       nil,
				revisionLister: nil,
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
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
				return newDefaultControllerClientsAndInformers(NoOpFn, false)
			},
			fields: fields{
				cc:             newDefaultCompositeController(),
				parentResource: &DefaultApiResource,
				mcClient:       nil,
				revisionLister: nil,
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
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewNotFound(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true)
			},
			fields: fields{
				cc:             newDefaultCompositeController(),
				parentResource: &DefaultApiResource,
				mcClient:       nil,
				revisionLister: nil,
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
			name: "no error on update parent with conflict api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewConflict(schema.GroupResource{
							Group:    TestGroup,
							Resource: TestResource,
						}, TestName, nil)
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true)
			},
			fields: fields{
				cc:             newDefaultCompositeController(),
				parentResource: &DefaultApiResource,
				mcClient:       nil,
				revisionLister: nil,
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
			name: "error on update parent status with unexpected api error",
			clientsAndInformers: func() (*fake.FakeDynamicClient, *dynamicdiscovery.ResourceMap, *dynamicclientset.Clientset, *dynamicclientset.ResourceClient, *dynamicinformer.ResourceInformer) {
				fakeDynamicClientFn := func(fakeDynamicClient *fake.FakeDynamicClient) {
					fakeDynamicClient.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
						return true, nil, apierrors.NewBadRequest("bad request")
					})
				}
				return newDefaultControllerClientsAndInformers(fakeDynamicClientFn, true)
			},
			fields: fields{
				cc:             newDefaultCompositeController(),
				parentResource: &DefaultApiResource,
				mcClient:       nil,
				revisionLister: nil,
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
			args:    args{key: defaultTestKey},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, dynClient, parentClient, parentInformer := tt.clientsAndInformers()
			pc := &parentController{
				cc:             tt.fields.cc,
				parentResource: tt.fields.parentResource,
				mcClient:       tt.fields.mcClient,
				dynClient:      dynClient,
				parentClient:   parentClient,
				parentInformer: parentInformer,
				revisionLister: tt.fields.revisionLister,
				stopCh:         tt.fields.stopCh,
				doneCh:         tt.fields.doneCh,
				queue:          tt.fields.queue,
				updateStrategy: tt.fields.updateStrategy,
				childInformers: tt.fields.childInformers,
				numWorkers:     tt.fields.numWorkers,
				eventRecorder:  tt.fields.eventRecorder,
				finalizer:      tt.fields.finalizer,
				customize:      tt.fields.customize,
				syncHook:       tt.fields.syncHook,
				finalizeHook:   tt.fields.finalizeHook,
				logger:         tt.fields.logger,
			}
			if err := pc.sync(tt.args.key); (err != nil) != tt.wantErr {
				t.Errorf("sync() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_parentController_sync_requeue_item_when_hook_throw_TooManyRequestError(t *testing.T) {
	logging.InitLogging(&zap.Options{})
	_, _, dynClient, parentClient, parentInformer := newDefaultControllerClientsAndInformers(ListFn, true)
	pc := &parentController{
		cc:             newDefaultCompositeController(),
		parentResource: &DefaultApiResource,
		mcClient:       nil,
		dynClient:      dynClient,
		parentClient:   parentClient,
		parentInformer: parentInformer,
		revisionLister: nil,
		stopCh:         NewCh(),
		doneCh:         NewCh(),
		queue:          NewDefaultWorkQueue(),
		updateStrategy: nil,
		childInformers: nil,
		numWorkers:     1,
		eventRecorder:  NewFakeRecorder(),
		finalizer:      DefaultFinalizerManager,
		customize:      defaultCustomizeManager(),
		syncHook:       NewErrorExecutorStub(&hooks.TooManyRequestError{AfterSecond: 0}),
		finalizeHook:   NewHookExecutorStub(defaultSyncResponse),
		logger:         logging.Logger,
	}

	err := pc.sync(defaultTestKey)
	assert.Nil(t, err)
	assert.Equal(t, 1, pc.queue.Len())
}
