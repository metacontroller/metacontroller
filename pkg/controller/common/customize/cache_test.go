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
	"testing"
)

func TestAdd_ElementFirstTime(t *testing.T) {
	responseCache := NewResponseCache()
	mockResponse := CustomizeHookResponse{}

	responseCache.Add("some", 12, &mockResponse)

	expected := customizeResponseCacheEntry{parentGeneration: 12, cachedResponse: &mockResponse}

	if cachedElement := responseCache.Get("some", 12); cachedElement != expected.cachedResponse {
		t.Errorf("Incorrect responseCache entry, got: %v, expected: %v", cachedElement, expected)
	}
}

func TestAdd_ElementOverridePreviousOne(t *testing.T) {
	responseCache := NewResponseCache()
	mockResponse := CustomizeHookResponse{}
	expected := customizeResponseCacheEntry{parentGeneration: 14, cachedResponse: &mockResponse}
	responseCache.Add("some", 12, &mockResponse)

	responseCache.Add("some", 14, &mockResponse)

	if cachedElement := responseCache.Get("some", 14); cachedElement != expected.cachedResponse {
		t.Errorf("Incorrect cache entry, got: %v, expected: %v", cachedElement, expected)
	}
	if cachedElement := responseCache.Get("some", 12); cachedElement != nil {
		t.Errorf("Incorrect cache entry, got: %v, expected nil", cachedElement)
	}
}

func TestGet_IfNotPresent(t *testing.T) {
	responseCache := NewResponseCache()

	response := responseCache.Get("some", 13)

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfPresentWithDifferentGeneration(t *testing.T) {
	responseCache := NewResponseCache()
	mockResponse := CustomizeHookResponse{}
	responseCache.Add("some", 12, &mockResponse)

	response := responseCache.Get("some", 13)

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfExistsAndGenerationMatches(t *testing.T) {
	responseCache := NewResponseCache()
	expectedResponse := CustomizeHookResponse{}
	responseCache.Add("some", 12, &expectedResponse)

	response := responseCache.Get("some", 12)

	if response != &expectedResponse {
		t.Errorf("Incorrect cache entry, expected: %v, got: %v", expectedResponse, response)
	}
}

func Test_ConcurrentMapAccess(t *testing.T) {
	responseCache := NewResponseCache()
	someResponse := CustomizeHookResponse{}

	go responseCache.Add("some", 1, &someResponse)
	go responseCache.Add("some_one", 1, &someResponse)
	go responseCache.Add("some_two", 1, &someResponse)
	go responseCache.Add("some_three", 1, &someResponse)
	go responseCache.Add("some_four", 1, &someResponse)
}
