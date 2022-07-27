/*
 *
 * Copyright 2022. Metacontroller authors.
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 * https://www.apache.org/licenses/LICENSE-2.0
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

// +groupName=examples.metacontroller.io
package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// ConfigMapPropagation
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=configmappropagations,scope=Cluster
type ConfigMapPropagation struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   ConfigMapPropagationSpec   `json:"spec"`
	Status ConfigMapPropagationStatus `json:"status,omitempty"`
}

type ConfigMapPropagationSpec struct {
	// Name of the configmap to propagate
	sourceName string `json:"sourceName"`
	// Namespace of the configmap to propagate
	sourceNamespace string `json:"sourceNamespace"`
	// List of namesppaces to which propagate configmap
	targetNamespaces []string `json:"targetNamespaces"`
}

type ConfigMapPropagationStatus struct {
	expected_copies    int `json:"expected_copies,omitempty"`
	actual_copies      int `json:"actual_copies,omitempty"`
	observedGeneration int `json:"observedGeneration,omitempty"`
}
