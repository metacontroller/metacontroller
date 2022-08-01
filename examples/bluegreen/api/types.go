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

// +groupName=ctl.enisoc.com
package v1

import (
	appsv1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// BlueGreenDeployment
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=bluegreendeployments,scope=Namespaced
type BlueGreenDeployment struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   BlueGreenDeploymentSpec   `json:"spec"`
	Status BlueGreenDeploymentStatus `json:"status,omitempty"`
}

type BlueGreenDeploymentSpec struct {
	replicas        int                  `json:"replicas"`
	minReadySeconds int                  `json:"minReadySeconds"`
	selector        metav1.LabelSelector `json:"selector"`
	template        v1.PodTemplateSpec   `json:"template"`
	service         ServiceTemplateSpec  `json:"service"`
}

type BlueGreenDeploymentStatus struct {
	active             appsv1.ReplicaSetStatus `json:"active,omitempty"`
	activeColor        string                  `json:"activeColor,omitempty"`
	inactive           appsv1.ReplicaSetStatus `json:"inactive,omitempty"`
	observedGeneration int                     `json:"observedGeneration,omitempty"`
	readyReplicas      int                     `json:"readyReplicas,omitempty"`
	replicas           int                     `json:"replicas,omitempty"`
}

type ServiceTemplateSpec struct {
	metav1.ObjectMeta `json:"metadata,omitempty"`
	spec              v1.ServiceSpec `json:"spec"`
}
