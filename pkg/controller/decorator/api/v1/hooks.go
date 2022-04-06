package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	v1 "metacontroller/pkg/controller/common/api/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// DecoratorHookRequest is the object sent as JSON to the sync hook.
type DecoratorHookRequest struct {
	Controller  *v1alpha1.DecoratorController `json:"controller"`
	Object      *unstructured.Unstructured    `json:"object"`
	Attachments v1.RelativeObjectMap          `json:"attachments"`
	Related     v1.RelativeObjectMap          `json:"related"`
	Finalizing  bool                          `json:"finalizing"`
}

// DecoratorHookResponse is the expected format of the JSON response from the sync hook.
type DecoratorHookResponse struct {
	Labels      map[string]*string           `json:"labels"`
	Annotations map[string]*string           `json:"annotations"`
	Status      map[string]interface{}       `json:"status"`
	Attachments []*unstructured.Unstructured `json:"attachments"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}
