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
	v1 "metacontroller/pkg/controller/common/customize/api/v1"
	"testing"
)

func TestAdd_ElementFirstTime(t *testing.T) {
	responseCache := newResponseCache()
	mockResponse := v1.CustomizeHookResponse{}

	responseCache.Set(customizeKey{"some", 12}, &mockResponse)

	if cachedElement, _ := responseCache.Get(customizeKey{"some", 12}); cachedElement != &mockResponse {
		t.Errorf("Incorrect responseCache entry, got: %v, expected: %v", cachedElement, mockResponse)
	}
}

func TestGet_IfNotPresent(t *testing.T) {
	responseCache := newResponseCache()

	response, _ := responseCache.Get(customizeKey{"some", 13})

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfPresentWithDifferentGeneration(t *testing.T) {
	responseCache := newResponseCache()
	mockResponse := v1.CustomizeHookResponse{}
	responseCache.Set(customizeKey{"some", 12}, &mockResponse)

	response, _ := responseCache.Get(customizeKey{"some", 13})

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfExistsAndGenerationMatches(t *testing.T) {
	responseCache := newResponseCache()
	expectedResponse := v1.CustomizeHookResponse{}
	responseCache.Set(customizeKey{"some", 12}, &expectedResponse)

	response, _ := responseCache.Get(customizeKey{"some", 12})

	if response != &expectedResponse {
		t.Errorf("Incorrect cache entry, expected: %v, got: %v", expectedResponse, response)
	}
}

func Test_ConcurrentMapAccess(t *testing.T) {
	responseCache := newResponseCache()
	someResponse := v1.CustomizeHookResponse{}

	go responseCache.Set(customizeKey{"some", 12}, &someResponse)
	go responseCache.Set(customizeKey{"some_one", 12}, &someResponse)
	go responseCache.Set(customizeKey{"some_two", 12}, &someResponse)
	go responseCache.Set(customizeKey{"some_three", 12}, &someResponse)
	go responseCache.Set(customizeKey{"some_four", 12}, &someResponse)
}
