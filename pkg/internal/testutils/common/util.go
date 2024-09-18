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
	"metacontroller/pkg/controller/common/finalizer"
	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"

	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

func NewCh() chan struct{} {
	return make(chan struct{}, 1)
}

var NoOpFn = func(fakeDynamicClient *fake.FakeDynamicClient) {}

var ListFn = func(fakeDynamicClient *fake.FakeDynamicClient) {
	fakeDynamicClient.PrependReactor("list", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		result := unstructured.UnstructuredList{
			Object: make(map[string]interface{}),
			Items: []unstructured.Unstructured{
				*NewDefaultUnstructured(),
			},
		}
		return true, &result, nil
	})
}

var DefaultFinalizerManager = finalizer.NewManager("testFinalizerManager", false)

var DefaultApiResource = dynamicdiscovery.APIResource{
	APIResource: NewDefaultAPIResource(),
	APIVersion:  TestAPIVersion,
}

func NewFakeRecorder() *record.FakeRecorder {
	return record.NewFakeRecorder(1)
}

func NewDefaultWorkQueue() workqueue.TypedRateLimitingInterface[any] {
	return workqueue.NewTypedRateLimitingQueueWithConfig(
		workqueue.DefaultTypedControllerRateLimiter[any](),
		workqueue.TypedRateLimitingQueueConfig[any]{
			Name: "testQueue",
		},
	)
}
