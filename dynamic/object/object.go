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
	k8s "k8s.io/metacontroller/third_party/kubernetes"
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

func GetStatusCondition(obj map[string]interface{}, conditionType string) *StatusCondition {
	conditions := k8s.GetNestedArray(obj, "status", "conditions")
	for _, item := range conditions {
		if obj, ok := item.(map[string]interface{}); ok {
			if ctype, ok := obj["type"].(string); ok && ctype == conditionType {
				return NewStatusCondition(obj)
			}
		}
	}
	return nil
}

func SetCondition(status map[string]interface{}, condition *StatusCondition) {
	conditions := k8s.GetNestedArray(status, "conditions")
	// If the condition is already there, update it.
	for i, item := range conditions {
		if cobj, ok := item.(map[string]interface{}); ok {
			if ctype, ok := cobj["type"].(string); ok && ctype == condition.Type {
				conditions[i] = condition.Object()
				return
			}
		}
	}
	// The condition wasn't found. Append it.
	conditions = append(conditions, condition.Object())
	k8s.SetNestedField(status, conditions, "conditions")
}

func SetStatusCondition(obj map[string]interface{}, condition *StatusCondition) {
	status := k8s.GetNestedObject(obj, "status")
	if status == nil {
		status = make(map[string]interface{})
	}
	SetCondition(status, condition)
	k8s.SetNestedField(obj, status, "status")
}

func GetObservedGeneration(obj map[string]interface{}) int64 {
	return k8s.GetNestedInt64(obj, "status", "observedGeneration")
}
