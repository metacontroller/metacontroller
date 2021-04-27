/*
Copyright 2018 Google Inc.

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

package object

import (
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type StatusCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
	Message string `json:"message,omitempty"`
}

func (c *StatusCondition) Object() map[string]interface{} {
	obj := map[string]interface{}{
		"type":   c.Type,
		"status": c.Status,
	}
	if c.Reason != "" {
		obj["reason"] = c.Reason
	}
	if c.Message != "" {
		obj["message"] = c.Message
	}
	return obj
}

func NewStatusCondition(obj map[string]interface{}) *StatusCondition {
	cond := &StatusCondition{}
	if ctype, ok := obj["type"].(string); ok {
		cond.Type = ctype
	}
	if cstatus, ok := obj["status"].(string); ok {
		cond.Status = cstatus
	}
	if creason, ok := obj["reason"].(string); ok {
		cond.Reason = creason
	}
	if cmessage, ok := obj["message"].(string); ok {
		cond.Message = cmessage
	}
	return cond
}

func GetStatusCondition(obj map[string]interface{}, conditionType string) (*StatusCondition, error) {
	conditions, found, err := unstructured.NestedSlice(obj, "status", "conditions")
	if !found || err != nil {
		return nil, err
	}
	for _, item := range conditions {
		if obj, ok := item.(map[string]interface{}); ok {
			if ctype, ok := obj["type"].(string); ok && ctype == conditionType {
				return NewStatusCondition(obj), nil
			}
		}
	}
	return nil, nil
}

func SetCondition(status map[string]interface{}, condition *StatusCondition) error {
	conditions, found, err := unstructured.NestedSlice(status, "conditions")
	if err != nil {
		return err
	}
	// If the condition is already there, update it.
	if found {
		for i, item := range conditions {
			if cobj, ok := item.(map[string]interface{}); ok {
				if ctype, ok := cobj["type"].(string); ok && ctype == condition.Type {
					conditions[i] = condition.Object()
					return nil
				}
			}
		}
	}
	// The condition wasn't found. Append it.
	conditions = append(conditions, condition.Object())
	if err := unstructured.SetNestedField(status, conditions, "conditions"); err != nil {
		return err
	}
	return nil
}

func SetStatusCondition(obj map[string]interface{}, condition *StatusCondition) error {
	status, found, err := unstructured.NestedMap(obj, "status")
	if err != nil {
		return err
	}
	if !found {
		status = make(map[string]interface{})
	}
	SetCondition(status, condition)
	if err := unstructured.SetNestedField(obj, status, "status"); err != nil {
		return err
	}
	return nil
}

func GetObservedGeneration(obj map[string]interface{}) (int64, bool, error) {
	return unstructured.NestedInt64(obj, "status", "observedGeneration")
}
