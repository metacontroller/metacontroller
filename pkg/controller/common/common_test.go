/*
Copyright 2026 Metacontroller authors.

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
	"testing"

	dynamicdiscovery "metacontroller/pkg/dynamic/discovery"
	dynamicinformer "metacontroller/pkg/dynamic/informer"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestSyncMap(t *testing.T) {
	sm := &SyncMap[string, int]{}

	// Test Store and Get
	sm.Store("one", 1)
	if v := sm.Get("one"); v != 1 {
		t.Errorf("expected 1, got %v", v)
	}

	// Test Set and Get
	sm.Set("one-set", 11)
	if v := sm.Get("one-set"); v != 11 {
		t.Errorf("expected 11, got %v", v)
	}

	// Test Get for non-existent key
	if v := sm.Get("two"); v != 0 {
		t.Errorf("expected 0 for non-existent key, got %v", v)
	}

	// Test Len
	if l := sm.Len(); l != 2 {
		t.Errorf("expected Len 2, got %v", l)
	}

	sm.Store("two", 2)
	if l := sm.Len(); l != 3 {
		t.Errorf("expected Len 3, got %v", l)
	}

	// Test LoadOrStore
	actual, loaded := sm.LoadOrStore("three", 3)
	if loaded || actual != 3 {
		t.Errorf("expected 3, false; got %v, %v", actual, loaded)
	}
	actual, loaded = sm.LoadOrStore("one", 10)
	if !loaded || actual != 1 {
		t.Errorf("expected 1, true; got %v, %v", actual, loaded)
	}

	// Test GetOrCreate
	actual, loaded = sm.GetOrCreate("four", 4)
	if loaded || actual != 4 {
		t.Errorf("expected 4, false; got %v, %v", actual, loaded)
	}
	actual, loaded = sm.GetOrCreate("one", 10)
	if !loaded || actual != 1 {
		t.Errorf("expected 1, true; got %v, %v", actual, loaded)
	}

	// Test ForEach
	sum := 0
	sm.ForEach(func(k string, v int) {
		sum += v
	})
	if sum != 21 { // 1 + 11 + 2 + 3 + 4
		t.Errorf("expected sum 21, got %v", sum)
	}

	// Test Delete
	sm.Delete("one")
	if v := sm.Get("one"); v != 0 {
		t.Errorf("expected 0 after delete, got %v", v)
	}
	if l := sm.Len(); l != 4 {
		t.Errorf("expected Len 4 after delete, got %v", l)
	}
}

func TestGroupKindMap(t *testing.T) {
	m := NewGroupKindMap()
	gk := schema.GroupKind{Group: "g", Kind: "k"}
	res := &dynamicdiscovery.APIResource{APIVersion: "v1"}

	m.Store(gk, res)
	if got := m.Get(gk); got != res {
		t.Errorf("expected %v, got %v", res, got)
	}

	if l := m.Len(); l != 1 {
		t.Errorf("expected Len 1, got %v", l)
	}
}

func TestInformerMap(t *testing.T) {
	m := NewInformerMap()
	gvr := schema.GroupVersionResource{Group: "g", Version: "v", Resource: "r"}
	inf := &dynamicinformer.ResourceInformer{}

	m.Store(gvr, inf)
	if got := m.Get(gvr); got != inf {
		t.Errorf("expected %v, got %v", inf, got)
	}

	if l := m.Len(); l != 1 {
		t.Errorf("expected Len 1, got %v", l)
	}
}
