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

package common

import (
	"metacontroller/pkg/apis/metacontroller/v1alpha1"
	"metacontroller/pkg/controller/common/api"
	commonv1 "metacontroller/pkg/controller/common/api/v1"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

type WebhookRequestBuilder interface {
	WithController(controller *v1alpha1.DecoratorController) WebhookRequestBuilder
	WithParet(object *unstructured.Unstructured) WebhookRequestBuilder
	WithAttachments(attachments commonv1.RelativeObjectMap) WebhookRequestBuilder
	WithRelatedObjects(related commonv1.RelativeObjectMap) WebhookRequestBuilder
	IsFinalizing() WebhookRequestBuilder
	Build() api.WebhookRequest
}
