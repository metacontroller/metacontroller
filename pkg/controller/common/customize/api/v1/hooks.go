package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CustomizeHookRequest is a request send to customize hook
type CustomizeHookRequest struct {
	Controller v1alpha1.CustomizableController `json:"controller"`
	Parent     *unstructured.Unstructured      `json:"parent"`
}

func (r *CustomizeHookRequest) GetParent() *unstructured.Unstructured {
	return r.Parent
}

// CustomizeHookResponse is a response from customize hook
type CustomizeHookResponse struct {
	RelatedResourceRules []*v1alpha1.RelatedResourceRule `json:"relatedResources,omitempty"`
}
