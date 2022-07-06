package v1

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/etag_cache"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// CustomizeHookRequest is a request send to customize hook
type CustomizeHookRequest struct {
	Controller v1alpha1.CustomizableController `json:"controller"`
	Parent     *unstructured.Unstructured      `json:"parent"`
}

func (r *CustomizeHookRequest) GetCacheKey() string {
	return etag_cache.GetKeyFromObject(r.Parent)
}

// CustomizeHookResponse is a response from customize hook
type CustomizeHookResponse struct {
	RelatedResourceRules []*v1alpha1.RelatedResourceRule `json:"relatedResources,omitempty"`
}
