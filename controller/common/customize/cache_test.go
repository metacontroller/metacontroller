package customize

import (
	"testing"
)

func TestAdd_ElementFirstTime(t *testing.T) {
	customizeCache := make(CustomizeResponseCache)
	mockResponse := CustomizeHookResponse{}

	customizeCache.Add("some", 12, &mockResponse)

	expected := customizeResponseCacheEntry{parentGeneration: 12, cachedResponse: &mockResponse}

	if customizeCache["some"] != expected {
		t.Errorf("Incorrect cache entry, got: %v, expected: %v", customizeCache["someName"], expected)
	}
}

func TestAdd_ElementOverridePreviousOne(t *testing.T) {
	customizeCache := make(CustomizeResponseCache)
	mockResponse := CustomizeHookResponse{}
	expected := customizeResponseCacheEntry{parentGeneration: 14, cachedResponse: &mockResponse}
	customizeCache.Add("some", 12, &mockResponse)

	customizeCache.Add("some", 14, &mockResponse)

	if customizeCache["some"] != expected {
		t.Errorf("Incorrect cache entry, got: %v, expected: %v", customizeCache["someName"], expected)
	}
}

func TestGet_IfNotPresent(t *testing.T) {
	customizeCache := make(CustomizeResponseCache)

	response := customizeCache.Get("some", 13)

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfPresentWithDifferentGeneration(t *testing.T) {
	customizeCache := make(CustomizeResponseCache)
	mockResponse := CustomizeHookResponse{}
	customizeCache.Add("some", 12, &mockResponse)

	response := customizeCache.Get("some", 13)

	if response != nil {
		t.Errorf("Incorrect cache entry, should be nil, got: %v", response)
	}
}

func TestGet_IfExistsAndGenerationMatches(t *testing.T) {
	customizeCache := make(CustomizeResponseCache)
	expectedResponse := CustomizeHookResponse{}
	customizeCache.Add("some", 12, &expectedResponse)

	response := customizeCache.Get("some", 12)

	if response != &expectedResponse {
		t.Errorf("Incorrect cache entry, expected: %v, got: %v", expectedResponse, response)
	}
}
