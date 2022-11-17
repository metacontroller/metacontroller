package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	v1 "metacontroller/pkg/controller/common/api/v1"
	"metacontroller/pkg/controller/decorator/api/common"

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

func (r *DecoratorHookRequest) GetRootObject() *unstructured.Unstructured {
	return r.Object
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

type requestBuilder struct {
	controller  *v1alpha1.DecoratorController
	object      *unstructured.Unstructured
	attachments v1.RelativeObjectMap
	related     v1.RelativeObjectMap
	finalizing  bool
}

func NewRequestBuilder() common.WebhookRequestBuilder {
	return &requestBuilder{}
}

func (r *requestBuilder) WithController(controller *v1alpha1.DecoratorController) common.WebhookRequestBuilder {
	r.controller = controller
	return r
}

func (r *requestBuilder) WithParet(object *unstructured.Unstructured) common.WebhookRequestBuilder {
	r.object = object
	return r
}

func (r *requestBuilder) WithAttachments(attachments v1.RelativeObjectMap) common.WebhookRequestBuilder {
	r.attachments = attachments
	return r
}

func (r *requestBuilder) WithRelatedObjects(related v1.RelativeObjectMap) common.WebhookRequestBuilder {
	r.related = related
	return r
}

func (r *requestBuilder) IsFinalizing() common.WebhookRequestBuilder {
	r.finalizing = true
	return r
}

func (r *requestBuilder) Build() api.WebhookRequest {
	return &DecoratorHookRequest{
		Controller:  r.controller,
		Object:      r.object,
		Attachments: r.attachments,
		Related:     r.related,
		Finalizing:  r.finalizing,
	}
}
