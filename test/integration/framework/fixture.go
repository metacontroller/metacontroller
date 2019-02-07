/*
Copyright 2019 Google Inc.

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

package framework

import (
	"fmt"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/typed/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	mcclientset "metacontroller.app/client/generated/clientset/internalclientset"
	dynamicclientset "metacontroller.app/dynamic/clientset"
)

const (
	defaultWaitTimeout  = 60 * time.Second
	defaultWaitInterval = 250 * time.Millisecond
)

// Fixture is a collection of scaffolding for each integration test method.
type Fixture struct {
	t *testing.T

	teardownFuncs []func() error

	dynamic        *dynamicclientset.Clientset
	kubernetes     kubernetes.Interface
	apiextensions  apiextensionsclient.ApiextensionsV1beta1Interface
	metacontroller mcclientset.Interface
}

func NewFixture(t *testing.T) *Fixture {
	config := ApiserverConfig()
	apiextensions, err := apiextensionsclient.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	dynClient, err := dynamicclientset.New(config, resourceMap)
	if err != nil {
		t.Fatal(err)
	}
	mcClient, err := mcclientset.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}

	return &Fixture{
		t:              t,
		dynamic:        dynClient,
		kubernetes:     clientset,
		apiextensions:  apiextensions,
		metacontroller: mcClient,
	}
}

// CreateNamespace creates a namespace that will be deleted after this test
// finishes.
func (f *Fixture) CreateNamespace(namespace string) *v1.Namespace {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	ns, err := f.kubernetes.CoreV1().Namespaces().Create(ns)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.kubernetes.CoreV1().Namespaces().Delete(ns.Name, nil)
	})
	return ns
}

// TearDown cleans up resources created through this instance of the test fixture.
func (f *Fixture) TearDown() {
	for i := len(f.teardownFuncs) - 1; i >= 0; i-- {
		teardown := f.teardownFuncs[i]
		if err := teardown(); err != nil {
			f.t.Logf("Error during teardown: %v", err)
			// Mark the test as failed, but continue trying to tear down.
			f.t.Fail()
		}
	}
}

// Wait polls the condition until it's true, with a default interval and timeout.
// This is meant for use in integration tests, so frequent polling is fine.
//
// The condition function returns a bool indicating whether it is satisfied,
// as well as an error which should be non-nil if and only if the function was
// unable to determine whether or not the condition is satisfied (for example
// if the check involves calling a remote server and the request failed).
//
// If the condition function returns a non-nil error, Wait will log the error
// and continue retrying until the timeout.
func (f *Fixture) Wait(condition func() (bool, error)) error {
	start := time.Now()
	for {
		ok, err := condition()
		if err == nil && ok {
			return nil
		}
		if err != nil {
			// Log error, but keep trying until timeout.
			f.t.Logf("error while waiting for condition: %v", err)
		}
		if time.Since(start) > defaultWaitTimeout {
			return fmt.Errorf("timed out waiting for condition (%v)", err)
		}
		time.Sleep(defaultWaitInterval)
	}
}

func (f *Fixture) deferTeardown(teardown func() error) {
	f.teardownFuncs = append(f.teardownFuncs, teardown)
}
