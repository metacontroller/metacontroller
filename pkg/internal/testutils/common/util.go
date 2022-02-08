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
	fakeCtrlRuntime "sigs.k8s.io/controller-runtime/pkg/client/fake"

	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

func NewCh() chan struct{} {
	return make(chan struct{}, 1)
}

var NoOpFn = func(fakeDynamicClient *fake.FakeDynamicClient) {}

var DefaultFinalizerManager = finalizer.NewManager(fakeCtrlRuntime.NewClientBuilder().Build(), "testFinalizerManager", false)

var DefaultApiResource = dynamicdiscovery.APIResource{
	APIResource: NewDefaultAPIResource(),
	APIVersion:  TestAPIVersion,
}

func NewFakeRecorder() *record.FakeRecorder {
	return record.NewFakeRecorder(1)
}

func NewDefaultWorkQueue() workqueue.RateLimitingInterface {
	return workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "testQueue")
}
