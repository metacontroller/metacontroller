/*
Copyright 2017 Google Inc.

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

package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CompositeController struct {
	metav1.TypeMeta               `json:",inline"`
	metav1.ObjectMeta             `json:"metadata"`
	Spec   CompositeControllerSpec   `json:"spec"`
	Status CompositeControllerStatus `json:"status,omitempty"`
}

type CompositeControllerSpec struct {
	ParentResource   ResourceRule          `json:"parentResource"`
	ChildResources   []ResourcesRule       `json:"childResources,omitempty"`
	ClientConfig     ClientConfig          `json:"clientConfig,omitempty"`
	Hooks            CompositeControllerHooks `json:"hooks,omitempty"`
	GenerateSelector bool                  `json:"generateSelector,omitempty"`
}

type ResourceRule struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
}

type ResourcesRule struct {
	APIVersion string   `json:"apiVersion`
	Resources  []string `json:"resources"`
}

type ClientConfig struct {
	Service ServiceReference `json:"service,omitempty"`
}

type ServiceReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`
}

type CompositeControllerHooks struct {
	Sync CompositeControllerSyncHook `json:"sync,omitempty"`
}

type CompositeControllerSyncHook struct {
	Path string `json:"path"`
}

type CompositeControllerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type CompositeControllerList struct {
	metav1.TypeMeta          `json:",inline"`
	metav1.ListMeta          `json:"metadata"`
	Items []CompositeController `json:"items"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InitializerController struct {
	metav1.TypeMeta                    `json:",inline"`
	metav1.ObjectMeta                  `json:"metadata"`
	Spec   InitializerControllerSpec   `json:"spec"`
	Status InitializerControllerStatus `json:"status,omitempty"`
}

type InitializerControllerSpec struct {
	InitializerName        string                     `json:"initializerName"`
	UninitializedResources []ResourcesRule            `json:"uninitializedResources,omitempty"`
	ClientConfig           ClientConfig               `json:"clientConfig,omitempty"`
	Hooks                  InitializerControllerHooks `json:"hooks,omitempty"`
}

type InitializerControllerHooks struct {
	Init InitializerControllerInitHook `json:"init,omitempty"`
}

type InitializerControllerInitHook struct {
	Path string `json:"path"`
}

type InitializerControllerStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type InitializerControllerList struct {
	metav1.TypeMeta               `json:",inline"`
	metav1.ListMeta               `json:"metadata"`
	Items []InitializerController `json:"items"`
}
