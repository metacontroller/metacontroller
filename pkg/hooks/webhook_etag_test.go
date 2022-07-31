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

package hooks

import (
	"bytes"
	"fmt"
	"io"
	"metacontroller/pkg/cache"
	compositev1 "metacontroller/pkg/controller/composite/api/v1"
	"metacontroller/pkg/logging"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

type MockResponse struct {
	resp *http.Response
	err  error
}

type CallResult struct {
	res compositev1.CompositeHookResponse
	err error
}

type TestStep struct {
	httpResponse    *MockResponse
	expectedHeaders map[string]string
	hookResult      *CallResult
}

type TestSteps []TestStep

type httpClientMock struct {
	doResponses     []*MockResponse
	expectedHeaders []map[string]string
	doIdx           int
}

func (m *httpClientMock) Do(req *http.Request) (*http.Response, error) {
	if m.doIdx < len(m.doResponses) {
		m.doIdx++
	}

	if m.expectedHeaders != nil && m.expectedHeaders[m.doIdx-1] != nil {
		for name, expectedValue := range m.expectedHeaders[m.doIdx-1] {
			if expectedValue == "*" {
				if req.Header.Values(name) == nil {
					panic("expected header is missing")
				}
			} else {
				headerValue := req.Header.Get(name)
				if headerValue != expectedValue {
					panic(fmt.Sprintf("expected header %s has value %s while expected to be %s", name, headerValue, expectedValue))
				}
			}
		}
	}

	data := m.doResponses[m.doIdx-1]
	return data.resp, data.err
}

func httpResponse304NotModified() *MockResponse {
	return &MockResponse{
		&http.Response{
			Status:     "304 Not modified",
			StatusCode: 304,
			Header:     map[string][]string{},
			Body:       io.NopCloser(bytes.NewBufferString("")),
		},
		nil,
	}
}

func httpResponse200(body string, etag string) *MockResponse {
	t := http.Response{
		Status:     "200 OK",
		StatusCode: 200,
		Header:     map[string][]string{},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
	}

	switch etag {
	case "":
		break
	default:
		t.Header.Set("Etag", etag)
	}

	return &MockResponse{
		&t,
		nil,
	}
}

func hookResultFromJson(body string) *CallResult {
	var res compositev1.CompositeHookResponse
	err := json.Unmarshal([]byte(body), &res)
	if err != nil {
		panic("unmarshal failed")
	}
	return &CallResult{
		res: res,
		err: err,
	}
}

func TestHookETag(t *testing.T) {
	body1 := `
		{
		  "resyncAfterSeconds": 30,
		  "children": [
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name"},
			  "spec": {"key": "val"}
			},
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name2"},
			  "spec": {"key": "val2"}
			}
		  ]
		}`
	body2 := `
		{
		  "resyncAfterSeconds": 30,
		  "children": [
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name"},
			  "spec": {"key": "val2"}
			},
			{
			  "apiVersion": "apps/metav1",
			  "kind": "Deployment",
			  "metadata": {"name": "res-name2"},
			  "spec": {"key": "val2","key2": "val3"}
			}
		  ]
		}`
	RunHookTest(t, TestSteps{
		{
			httpResponse: httpResponse200(body1, "000-000-000-001"),
			hookResult:   hookResultFromJson(body1),
		},
		{
			httpResponse: httpResponse304NotModified(),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: hookResultFromJson(body1),
		},
		{
			httpResponse: httpResponse200(body2, "000-000-000-002"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-001",
			},
			hookResult: hookResultFromJson(body2),
		},
		{
			httpResponse: httpResponse200(body2, "000-000-000-002"),
			expectedHeaders: map[string]string{
				"If-None-Match": "000-000-000-002",
			},
			hookResult: hookResultFromJson(body2),
		},
		{
			httpResponse: httpResponse304NotModified(),
			hookResult:   hookResultFromJson(body2),
		},
	})
}

func RunHookTest(t *testing.T, steps TestSteps) {
	logging.InitLogging(&zap.Options{})
	responses := make([]*MockResponse, len(steps))
	expectedHeaders := make([]map[string]string, len(steps))
	for id := range steps {
		responses[id] = steps[id].httpResponse
		expectedHeaders[id] = steps[id].expectedHeaders
	}

	parent := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"kind": "Some",
			"metadata": map[string]interface{}{
				"name":      "name",
				"namespace": "default",
			},
		},
	}

	request := &compositev1.CompositeHookRequest{
		Controller: nil,
		Parent:     parent,
	}

	hook := &webhookExecutor{
		client: &httpClientMock{
			responses,
			expectedHeaders,
			0,
		},
		url:      "",
		hookType: "",
		webhookAbstract: &webhookExecutorEtag{
			etagCache: cache.New[eTagKey, *eTagEntry](0, 0)},
	}

	for id := range steps {
		expected := steps[id].hookResult
		response := compositev1.CompositeHookResponse{}
		err := hook.Call(request, &response)
		assert.Equal(t, expected.err, err)
		assert.Equal(t, expected.res, response)
	}
}
