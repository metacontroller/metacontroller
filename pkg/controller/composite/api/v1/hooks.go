package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	v1 "metacontroller/pkg/controller/common/api/v1"
	"metacontroller/pkg/controller/composite/api/common"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CompositeHookRequest is the object sent as JSON to the sync and finalize hooks.
type CompositeHookRequest struct {
	Controller *v1alpha1.CompositeController `json:"controller"`
	Parent     *unstructured.Unstructured    `json:"parent"`
	Children   v1.RelativeObjectMap          `json:"children"`
	Related    v1.RelativeObjectMap          `json:"related"`
	Finalizing bool                          `json:"finalizing"`
}

type requestBuilder struct {
	controller *v1alpha1.CompositeController
	parent     *unstructured.Unstructured
	children   v1.RelativeObjectMap
	related    v1.RelativeObjectMap
	finalizing bool
}

func NewRequestBuilder() common.WebhookRequestBuilder {
	return &requestBuilder{}
}

func (r *requestBuilder) WithController(controller *v1alpha1.CompositeController) common.WebhookRequestBuilder {
	r.controller = controller
	return r
}

func (r *requestBuilder) WithParent(parent *unstructured.Unstructured) common.WebhookRequestBuilder {
	r.parent = parent
	return r
}

func (r *requestBuilder) WithChildren(children v1.RelativeObjectMap) common.WebhookRequestBuilder {
	r.children = children
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
	return &CompositeHookRequest{
		Controller: r.controller,
		Parent:     r.parent,
		Children:   r.children,
		Related:    r.related,
		Finalizing: r.finalizing,
	}
}

func (r *CompositeHookRequest) GetRootObject() *unstructured.Unstructured {
	return r.Parent
}

// CompositeHookResponse is the expected format of the JSON response from the sync and finalize hooks.
type CompositeHookResponse struct {
	Status   map[string]interface{}       `json:"status"`
	Children []*unstructured.Unstructured `json:"children"`

	ResyncAfterSeconds float64 `json:"resyncAfterSeconds"`

	// Finalized is only used by the finalize hook.
	Finalized bool `json:"finalized"`
}
