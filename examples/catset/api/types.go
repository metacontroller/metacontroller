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
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CatSet
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=catsets,scope=Namespaced
type CatSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   CatSetSpec   `json:"spec"`
	Status CatSetStatus `json:"status,omitempty"`
}

type CatSetSpec struct {
	serviceName          string                             `json:"serviceName"`
	selector             metav1.LabelSelector               `json:"selector"`
	replicas             int                                `json:"replicas"`
	template             v1.PodTemplateSpec                 `json:"template"`
	volumeClaimTemplates []v1.PersistentVolumeClaimTemplate `json:"volumeClaimTemplates"`
}

type CatSetStatus struct {
	conditions         []v1.PodCondition `json:"conditions,omitempty"`
	observedGeneration int               `json:"observedGeneration,omitempty"`
	readyReplicas      int               `json:"readyReplicas,omitempty"`
	replicas           int               `json:"replicas,omitempty"`
}
