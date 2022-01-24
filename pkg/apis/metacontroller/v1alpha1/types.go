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

// +groupName=metacontroller.k8s.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CompositeController
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=compositecontrollers,scope=Cluster,shortName=cc;cctl
type CompositeController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   CompositeControllerSpec   `json:"spec"`
	Status CompositeControllerStatus `json:"status,omitempty"`
}

func (cc *CompositeController) GetCustomizeHook() *Hook {
	if cc.Spec.Hooks == nil {
		return nil
	}
	return cc.Spec.Hooks.Customize
}

type CompositeControllerSpec struct {
	ParentResource CompositeControllerParentResourceRule  `json:"parentResource"`
	ChildResources []CompositeControllerChildResourceRule `json:"childResources,omitempty"`

	Hooks *CompositeControllerHooks `json:"hooks,omitempty"`

	ResyncPeriodSeconds *int32 `json:"resyncPeriodSeconds,omitempty"`
	GenerateSelector    *bool  `json:"generateSelector,omitempty"`
}

type ResourceRule struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
}

type CompositeControllerParentResourceRule struct {
	ResourceRule    `json:",inline"`
	RevisionHistory *CompositeControllerRevisionHistory `json:"revisionHistory,omitempty"`
}

type CompositeControllerRevisionHistory struct {
	FieldPaths []string `json:"fieldPaths,omitempty"`
}

type ChildUpdateMethod string

const (
	ChildUpdateOnDelete        ChildUpdateMethod = "OnDelete"
	ChildUpdateRecreate        ChildUpdateMethod = "Recreate"
	ChildUpdateInPlace         ChildUpdateMethod = "InPlace"
	ChildUpdateRollingRecreate ChildUpdateMethod = "RollingRecreate"
	ChildUpdateRollingInPlace  ChildUpdateMethod = "RollingInPlace"
)

type CompositeControllerChildResourceRule struct {
	ResourceRule   `json:",inline"`
	UpdateStrategy *CompositeControllerChildUpdateStrategy `json:"updateStrategy,omitempty"`
}

type CompositeControllerChildUpdateStrategy struct {
	Method       ChildUpdateMethod       `json:"method,omitempty"`
	StatusChecks ChildUpdateStatusChecks `json:"statusChecks,omitempty"`
}

type ChildUpdateStatusChecks struct {
	Conditions []StatusConditionCheck `json:"conditions,omitempty"`
}

type StatusConditionCheck struct {
	Type   string  `json:"type"`
	Status *string `json:"status,omitempty"`
	Reason *string `json:"reason,omitempty"`
}

type ServiceReference struct {
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	Port      *int32  `json:"port,omitempty"`
	Protocol  *string `json:"protocol,omitempty"`
}

type CompositeControllerHooks struct {
	Customize *Hook `json:"customize,omitempty"`
	Sync      *Hook `json:"sync,omitempty"`
	Finalize  *Hook `json:"finalize,omitempty"`

	PreUpdateChild  *Hook `json:"preUpdateChild,omitempty"`
	PostUpdateChild *Hook `json:"postUpdateChild,omitempty"`
}

type Hook struct {
	Webhook *Webhook `json:"webhook,omitempty"`
}

type Webhook struct {
	URL     *string          `json:"url,omitempty"`
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	Path    *string           `json:"path,omitempty"`
	Service *ServiceReference `json:"service,omitempty"`
}

type CompositeControllerStatus struct{}

// CompositeControllerList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CompositeControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CompositeController `json:"items"`
}

// ControllerRevision
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=controllerrevisions,scope=Namespaced
type ControllerRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	ParentPatch runtime.RawExtension         `json:"parentPatch"`
	Children    []ControllerRevisionChildren `json:"children,omitempty"`
}

type ControllerRevisionChildren struct {
	APIGroup string   `json:"apiGroup"`
	Kind     string   `json:"kind"`
	Names    []string `json:"names"`
}

// ControllerRevisionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ControllerRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ControllerRevision `json:"items"`
}

// DecoratorController
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=decoratorcontrollers,scope=Cluster,shortName=dec;decorators
type DecoratorController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DecoratorControllerSpec   `json:"spec"`
	Status DecoratorControllerStatus `json:"status,omitempty"`
}

func (dc *DecoratorController) GetCustomizeHook() *Hook {
	if dc.Spec.Hooks == nil {
		return nil
	}
	return dc.Spec.Hooks.Customize
}

type DecoratorControllerSpec struct {
	Resources   []DecoratorControllerResourceRule   `json:"resources"`
	Attachments []DecoratorControllerAttachmentRule `json:"attachments,omitempty"`

	Hooks *DecoratorControllerHooks `json:"hooks,omitempty"`

	ResyncPeriodSeconds *int32 `json:"resyncPeriodSeconds,omitempty"`
}

type DecoratorControllerResourceRule struct {
	ResourceRule       `json:",inline"`
	LabelSelector      *metav1.LabelSelector `json:"labelSelector,omitempty"`
	AnnotationSelector *AnnotationSelector   `json:"annotationSelector,omitempty"`
}

type AnnotationSelector struct {
	MatchAnnotations map[string]string                 `json:"matchAnnotations,omitempty"`
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

type DecoratorControllerAttachmentRule struct {
	ResourceRule   `json:",inline"`
	UpdateStrategy *DecoratorControllerAttachmentUpdateStrategy `json:"updateStrategy,omitempty"`
}

type DecoratorControllerAttachmentUpdateStrategy struct {
	Method ChildUpdateMethod `json:"method,omitempty"`
}

type DecoratorControllerHooks struct {
	Customize *Hook `json:"customize,omitempty"`
	Sync      *Hook `json:"sync,omitempty"`
	Finalize  *Hook `json:"finalize,omitempty"`
}

type DecoratorControllerStatus struct{}

// DecoratorControllerList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DecoratorControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DecoratorController `json:"items"`
}

type RelatedResourceRule struct {
	ResourceRule          `json:",inline"`
	*metav1.LabelSelector `json:"labelSelector"`
	Namespace             string   `json:"namespace,omitempty"`
	Names                 []string `json:"names"`
}

// CustomizableController is an interface representing Controller exposing customize hook
type CustomizableController interface {

	// GetCustomizeHook return v1alpha1.Hook or nil if not defined
	GetCustomizeHook() *Hook
}
