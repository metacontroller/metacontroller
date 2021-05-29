package customize

import (
	"testing"
)

func TestAdd_ElementFirstTime(t *testing.T) {
	responseCache := NewResponseCache()
	mockResponse := CustomizeHookResponse{}

	responseCache.Add("some", 12, &mockResponse)

	expected := customizeResponseCacheEntry{parentGeneration: 12, cachedResponse: &mockResponse}

	if responseCache.cache["some"] != expected {
		t.Errorf("Incorrect responseCache entry, got: %v, expected: %v", responseCache.cache["someName"], expected)
	}
}

func TestAdd_ElementOverridePreviousOne(t *testing.T) {
	responseCache := NewResponseCache()
	mockResponse := CustomizeHookResponse{}
	expected := customizeResponseCacheEntry{parentGeneration: 14, cachedResponse: &mockResponse}
	responseCache.Add("some", 12, &mockResponse)

	responseCache.Add("some", 14, &mockResponse)

	if responseCache.cache["some"] != expected {
		t.Errorf("Incorrect cache entry, got: %v, expected: %v", responseCache.cache["someName"], expected)
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
