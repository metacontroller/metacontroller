package decorator

import (
	v2 "metacontroller/pkg/controller/decorator/api/v2"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestConvertV2ToV1Response(t *testing.T) {
	c := &decoratorController{}
	v2Response := v2.DecoratorHookResponse{
		Labels: map[string]*string{
			"foo": ptr("bar"),
		},
		Annotations: map[string]*string{
			"baz": ptr("qux"),
		},
		Status: map[string]interface{}{
			"foo": "bar",
		},
		Attachments: []*unstructured.Unstructured{
			{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "Pod",
					"metadata": map[string]interface{}{
						"name": "child",
					},
				},
			},
		},
		ResyncAfterSeconds: 10.5,
		Finalized:          true,
	}

	v1Response := c.convertV2ToV1Response(v2Response)

	assert.Equal(t, v2Response.Labels, v1Response.Labels)
	assert.Equal(t, v2Response.Annotations, v1Response.Annotations)
	assert.Equal(t, v2Response.Status, v1Response.Status)
	assert.Equal(t, v2Response.Attachments, v1Response.Attachments)
	assert.Equal(t, v2Response.ResyncAfterSeconds, v1Response.ResyncAfterSeconds)
	assert.Equal(t, v2Response.Finalized, v1Response.Finalized)
}

func ptr(s string) *string {
	return &s
}
