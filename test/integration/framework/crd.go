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
	"strings"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"

	dynamicclientset "metacontroller.app/dynamic/clientset"
)

const (
	// APIGroup is the group used for CRDs created as part of the test.
	APIGroup = "test.metacontroller.app"
	// APIVersion is the group-version used for CRDs created as part of the test.
	APIVersion = APIGroup + "/v1"
)

// CreateCRD generates a quick-and-dirty CRD for use in tests,
// and installs it in the test environment's API server.
func (f *Fixture) CreateCRD(kind string, scope v1beta1.ResourceScope) (*v1beta1.CustomResourceDefinition, *dynamicclientset.ResourceClient) {
	singular := strings.ToLower(kind)
	plural := singular + "s"
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s.%s", plural, APIGroup),
		},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group: APIGroup,
			Scope: scope,
			Names: v1beta1.CustomResourceDefinitionNames{
				Singular: singular,
				Plural:   plural,
				Kind:     kind,
			},
			Versions: []v1beta1.CustomResourceDefinitionVersion{
				{
					Name:    "v1",
					Served:  true,
					Storage: true,
				},
			},
		},
	}
	crd, err := f.apiextensions.CustomResourceDefinitions().Create(crd)
	if err != nil {
		f.t.Fatal(err)
	}
	f.deferTeardown(func() error {
		return f.apiextensions.CustomResourceDefinitions().Delete(crd.Name, nil)
	})

	f.t.Logf("Waiting for %v CRD to appear in API server discovery info...", kind)
	err = f.Wait(func() (bool, error) {
		return resourceMap.Get(APIVersion, plural) != nil, nil
	})
	if err != nil {
		f.t.Fatal(err)
	}

	client, err := f.dynamic.Resource(APIVersion, plural)
	if err != nil {
		f.t.Fatal(err)
	}

	f.t.Logf("Waiting for %v CRD client List() to succeed...", kind)
	err = f.Wait(func() (bool, error) {
		_, err := client.List(metav1.ListOptions{})
		return err == nil, err
	})
	if err != nil {
		f.t.Fatal(err)
	}

	return crd, client
}

// UnstructuredCRD creates a new Unstructured object for the given CRD.
func UnstructuredCRD(crd *v1beta1.CustomResourceDefinition, name string) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion(crd.Spec.Group + "/" + crd.Spec.Versions[0].Name)
	obj.SetKind(crd.Spec.Names.Kind)
	obj.SetName(name)
	return obj
}

// UnstructuredJSON creates a new Unstructured object from the given JSON.
// It panics on a decode error because it's meant for use with hard-coded test
// data.
func UnstructuredJSON(apiVersion, kind, name, jsonStr string) *unstructured.Unstructured {
	obj := map[string]interface{}{}
	if err := json.Unmarshal([]byte(jsonStr), &obj); err != nil {
		panic(err)
	}
	u := &unstructured.Unstructured{Object: obj}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	return u
}
