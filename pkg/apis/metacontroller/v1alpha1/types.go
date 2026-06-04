/*
Copyright 2017 Google Inc.

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

// +groupName=metacontroller.k8s.io
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CompositeController
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=compositecontrollers,scope=Cluster,shortName=cc;cctl
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=unapproved, request not yet submitted"
type CompositeController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   CompositeControllerSpec   `json:"spec"`
	Status CompositeControllerStatus `json:"status,omitempty"`
}

func (cc *CompositeController) GetCustomizeHook() *Hook {
	if cc.Spec.Hooks == nil {
		return nil
	}
	return cc.Spec.Hooks.Customize
}

func (cc *CompositeController) GetConnections() []WebhookConnection {
	return cc.Spec.Connections
}

type CompositeControllerSpec struct {
	ParentResource CompositeControllerParentResourceRule  `json:"parentResource"`
	ChildResources []CompositeControllerChildResourceRule `json:"childResources,omitempty"`

	Hooks *CompositeControllerHooks `json:"hooks,omitempty"`

	// Connections defines connection settings (TLS, authentication) keyed by
	// webhook host. When a webhook URL's host matches a connection entry, the
	// entry's settings are used unless the webhook overrides them directly.
	// +optional
	Connections []WebhookConnection `json:"connections,omitempty"`

	ResyncPeriodSeconds *int32 `json:"resyncPeriodSeconds,omitempty"`
	GenerateSelector    *bool  `json:"generateSelector,omitempty"`
}

type ResourceRule struct {
	APIVersion string `json:"apiVersion"`
	Resource   string `json:"resource"`
}

type CompositeControllerParentResourceRule struct {
	ResourceRule        `json:",inline"`
	RevisionHistory     *CompositeControllerRevisionHistory `json:"revisionHistory,omitempty"`
	LabelSelector       *metav1.LabelSelector               `json:"labelSelector,omitempty"`
	IgnoreStatusChanges *bool                               `json:"ignoreStatusChanges,omitempty"`
}

type CompositeControllerRevisionHistory struct {
	FieldPaths []string `json:"fieldPaths,omitempty"`
}

// +kubebuilder:validation:Enum={"OnDelete","Recreate","InPlace","RollingRecreate","RollingInPlace"}
type ChildUpdateMethod string

const (
	ChildUpdateOnDelete        ChildUpdateMethod = "OnDelete"
	ChildUpdateRecreate        ChildUpdateMethod = "Recreate"
	ChildUpdateInPlace         ChildUpdateMethod = "InPlace"
	ChildUpdateRollingRecreate ChildUpdateMethod = "RollingRecreate"
	ChildUpdateRollingInPlace  ChildUpdateMethod = "RollingInPlace"
)

type CompositeControllerChildResourceRule struct {
	ResourceRule   `json:",inline"`
	UpdateStrategy *CompositeControllerChildUpdateStrategy `json:"updateStrategy,omitempty"`
}

type CompositeControllerChildUpdateStrategy struct {
	Method       ChildUpdateMethod       `json:"method,omitempty"`
	StatusChecks ChildUpdateStatusChecks `json:"statusChecks,omitempty"`
}

type ChildUpdateStatusChecks struct {
	Conditions []StatusConditionCheck `json:"conditions,omitempty"`
}

type StatusConditionCheck struct {
	Type   string  `json:"type"`
	Status *string `json:"status,omitempty"`
	Reason *string `json:"reason,omitempty"`
}

type ServiceReference struct {
	Name      string  `json:"name"`
	Namespace string  `json:"namespace"`
	Port      *int32  `json:"port,omitempty"`
	Protocol  *string `json:"protocol,omitempty"`
}

type CompositeControllerHooks struct {
	Customize *Hook `json:"customize,omitempty"`
	Sync      *Hook `json:"sync,omitempty"`
	Finalize  *Hook `json:"finalize,omitempty"`

	PreUpdateChild  *Hook `json:"preUpdateChild,omitempty"`
	PostUpdateChild *Hook `json:"postUpdateChild,omitempty"`
}

type Hook struct {
	// +kubebuilder:default:="v1"
	Version *HookVersion `json:"version,omitempty"`
	Webhook *Webhook     `json:"webhook,omitempty"`
}

// GetVersion returns the hook version, defaulting to v1 if not specified.
func (h *Hook) GetVersion() HookVersion {
	if h == nil || h.Version == nil || *h.Version == "" {
		return HookVersionV1
	}
	return *h.Version
}

// +kubebuilder:validation:Enum={"v1","v2"}
type HookVersion string

const (
	HookVersionV1 HookVersion = "v1"
	HookVersionV2 HookVersion = "v2"
)

type WebhookEtagConfig struct {
	Enabled             *bool  `json:"enabled,omitempty"`
	CacheTimeoutSeconds *int32 `json:"cacheTimeoutSeconds,omitempty"`
	CacheCleanupSeconds *int32 `json:"cacheCleanupSeconds,omitempty"`
}

type Webhook struct {
	URL *string `json:"url,omitempty"`
	// +kubebuilder:validation:Format:="duration"
	Timeout *metav1.Duration `json:"timeout,omitempty"`

	Etag    *WebhookEtagConfig `json:"etag,omitempty"`
	Path    *string            `json:"path,omitempty"`
	Service *ServiceReference  `json:"service,omitempty"`
	// Sets the json unmarshall mode. One of the 'loose' or 'strict'. In 'strict'
	// mode additional checks are performed to detect unknown and duplicated fields.
	ResponseUnmarshallMode *ResponseUnmarshallMode `json:"responseUnMarshallMode,omitempty"`
	// CABundle configures the CA certificate(s) used to verify the webhook server's
	// TLS certificate when the endpoint uses HTTPS with a private or self-signed CA.
	// If not specified, the system trust roots are used.
	// Exactly one of inline, secretRef, or configMapRef must be set when this field is present.
	// When set, overrides any caBundle from a matching connections entry.
	//
	// TODO(hot-reload): The CA bundle is resolved once when the controller CR is created or
	// updated, and is not reloaded automatically if the underlying Secret or ConfigMap changes.
	// To pick up a rotated CA, update the controller CR (e.g. add/change an annotation) to
	// trigger re-creation of the webhook executor with the new certificate data.
	// +optional
	CABundle *CABundle `json:"caBundle,omitempty"`

	// ClientTLS configures a client certificate for mutual TLS authentication with the
	// webhook server. When set, overrides any clientTLS from a matching connections entry.
	// +optional
	ClientTLS *ClientTLS `json:"clientTLS,omitempty"`

	// Authorization configures the Authorization header credential sent with every
	// request to this webhook. Mutually exclusive with basicAuth.
	// When set, overrides any authorization or basicAuth from a matching connections entry.
	// +optional
	Authorization *Authorization `json:"authorization,omitempty"`

	// BasicAuth configures HTTP Basic Authentication credentials sent with every
	// request to this webhook. Mutually exclusive with authorization.
	// When set, overrides any authorization or basicAuth from a matching connections entry.
	// +optional
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
}

// SecretRef identifies a Kubernetes Secret by name and namespace.
type SecretRef struct {
	// Name is the metadata.name of the target Secret.
	Name string `json:"name"`
	// Namespace is the metadata.namespace of the target Secret.
	Namespace string `json:"namespace"`
}

// SecretKeyRef identifies a specific key within a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the metadata.name of the target Secret.
	Name string `json:"name"`
	// Namespace is the metadata.namespace of the target Secret.
	Namespace string `json:"namespace"`
	// Key is the key within the Secret's data map.
	Key string `json:"key"`
}

// Authorization configures the Authorization header credentials for webhook requests.
// Exactly one of secretRef must be set.
type Authorization struct {
	// Type is the Authorization header scheme. The value is case-insensitive.
	// "Basic" is not a supported value; use the basicAuth field instead.
	// +kubebuilder:default="Bearer"
	// +optional
	Type string `json:"type,omitempty"`

	// SecretRef selects the key from a Secret containing the credential value.
	SecretRef SecretKeyRef `json:"secretRef"`
}

// BasicAuth configures HTTP Basic Authentication credentials for webhook requests.
// The username and password are read from the same Secret.
type BasicAuth struct {
	// SecretRef references the Secret containing the username and password.
	SecretRef SecretRef `json:"secretRef"`

	// UsernameKey is the key within the Secret for the username.
	// +kubebuilder:default="username"
	// +optional
	UsernameKey string `json:"usernameKey,omitempty"`

	// PasswordKey is the key within the Secret for the password.
	// +kubebuilder:default="password"
	// +optional
	PasswordKey string `json:"passwordKey,omitempty"`
}

// ClientTLS configures mutual TLS client credentials for webhook requests.
// The certificate and private key are read from the same Secret.
type ClientTLS struct {
	// SecretRef references the Secret containing the client certificate and private key.
	SecretRef SecretRef `json:"secretRef"`

	// CertKey is the key within the Secret for the PEM-encoded client certificate.
	// +kubebuilder:default="tls.crt"
	// +optional
	CertKey string `json:"certKey,omitempty"`

	// PrivateKeyKey is the key within the Secret for the PEM-encoded client private key.
	// +kubebuilder:default="tls.key"
	// +optional
	PrivateKeyKey string `json:"privateKeyKey,omitempty"`
}

// WebhookConnection defines connection settings applied to webhooks whose URL
// host matches the entry's host field. Settings are resolved once when the
// controller CR is created or updated.
type WebhookConnection struct {
	// Host is the host or host:port to match against webhook URLs.
	// Examples: "my-webhook.svc", "my-webhook.svc:8443"
	Host string `json:"host"`

	// CABundle configures the CA certificate(s) used to verify the webhook server's
	// TLS certificate.
	// +optional
	CABundle *CABundle `json:"caBundle,omitempty"`

	// ClientTLS configures a client certificate for mutual TLS authentication.
	// +optional
	ClientTLS *ClientTLS `json:"clientTLS,omitempty"`

	// Authorization configures the Authorization header credential. Mutually exclusive
	// with basicAuth.
	// +optional
	Authorization *Authorization `json:"authorization,omitempty"`

	// BasicAuth configures HTTP Basic Authentication credentials. Mutually exclusive
	// with authorization.
	// +optional
	BasicAuth *BasicAuth `json:"basicAuth,omitempty"`
}

// CABundle specifies the source of PEM-encoded CA certificate(s) used to verify
// the TLS certificate presented by a webhook server.
// Exactly one of inline, secretRef, or configMapRef must be set.
type CABundle struct {
	// Inline contains PEM-encoded CA certificate(s) directly embedded in the spec.
	// +optional
	Inline *string `json:"inline,omitempty"`

	// SecretRef references a key in a Kubernetes Secret containing PEM-encoded
	// CA certificate(s).
	// +optional
	SecretRef *ResourceKeyRef `json:"secretRef,omitempty"`

	// ConfigMapRef references a key in a Kubernetes ConfigMap containing
	// PEM-encoded CA certificate(s).
	// +optional
	ConfigMapRef *ResourceKeyRef `json:"configMapRef,omitempty"`
}

// ResourceKeyRef identifies a specific key within a Kubernetes Secret or ConfigMap.
type ResourceKeyRef struct {
	// Name is the metadata.name of the target Secret or ConfigMap.
	Name string `json:"name"`
	// Namespace is the metadata.namespace of the target Secret or ConfigMap.
	Namespace string `json:"namespace"`
	// Key is the key within the Secret's or ConfigMap's data map.
	// +kubebuilder:default="ca.crt"
	// +optional
	Key string `json:"key,omitempty"`
}

// +kubebuilder:validation:Enum:={"loose","strict"}
type ResponseUnmarshallMode string

const (
	ResponseUnmarshallModeLoose  ResponseUnmarshallMode = "loose"
	ResponseUnmarshallModeStrict ResponseUnmarshallMode = "strict"
)

type CompositeControllerStatus struct{}

// CompositeControllerList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CompositeControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []CompositeController `json:"items"`
}

// ControllerRevision
// +genclient
// +genclient:noStatus
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=controllerrevisions,scope=Namespaced
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=unapproved, request not yet submitted"
type ControllerRevision struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	ParentPatch runtime.RawExtension         `json:"parentPatch"`
	Children    []ControllerRevisionChildren `json:"children,omitempty"`
}

type ControllerRevisionChildren struct {
	APIGroup string   `json:"apiGroup"`
	Kind     string   `json:"kind"`
	Names    []string `json:"names"`
}

// ControllerRevisionList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type ControllerRevisionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []ControllerRevision `json:"items"`
}

// DecoratorController
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:path=decoratorcontrollers,scope=Cluster,shortName=dec;decorators
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=unapproved, request not yet submitted"
type DecoratorController struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`

	Spec   DecoratorControllerSpec   `json:"spec"`
	Status DecoratorControllerStatus `json:"status,omitempty"`
}

func (dc *DecoratorController) GetCustomizeHook() *Hook {
	if dc.Spec.Hooks == nil {
		return nil
	}
	return dc.Spec.Hooks.Customize
}

func (dc *DecoratorController) GetConnections() []WebhookConnection {
	return dc.Spec.Connections
}

type DecoratorControllerSpec struct {
	Resources   []DecoratorControllerResourceRule   `json:"resources"`
	Attachments []DecoratorControllerAttachmentRule `json:"attachments,omitempty"`

	Hooks *DecoratorControllerHooks `json:"hooks,omitempty"`

	// Connections defines connection settings (TLS, authentication) keyed by
	// webhook host. When a webhook URL's host matches a connection entry, the
	// entry's settings are used unless the webhook overrides them directly.
	// +optional
	Connections []WebhookConnection `json:"connections,omitempty"`

	ResyncPeriodSeconds *int32 `json:"resyncPeriodSeconds,omitempty"`
}

type DecoratorControllerResourceRule struct {
	ResourceRule        `json:",inline"`
	LabelSelector       *metav1.LabelSelector `json:"labelSelector,omitempty"`
	AnnotationSelector  *AnnotationSelector   `json:"annotationSelector,omitempty"`
	IgnoreStatusChanges *bool                 `json:"ignoreStatusChanges,omitempty"`
}

type AnnotationSelector struct {
	MatchAnnotations map[string]string                 `json:"matchAnnotations,omitempty"`
	MatchExpressions []metav1.LabelSelectorRequirement `json:"matchExpressions,omitempty"`
}

type DecoratorControllerAttachmentRule struct {
	ResourceRule   `json:",inline"`
	UpdateStrategy *DecoratorControllerAttachmentUpdateStrategy `json:"updateStrategy,omitempty"`
}

type DecoratorControllerAttachmentUpdateStrategy struct {
	Method ChildUpdateMethod `json:"method,omitempty"`
}

type DecoratorControllerHooks struct {
	Customize *Hook `json:"customize,omitempty"`
	Sync      *Hook `json:"sync,omitempty"`
	Finalize  *Hook `json:"finalize,omitempty"`
}

type DecoratorControllerStatus struct{}

// DecoratorControllerList
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type DecoratorControllerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []DecoratorController `json:"items"`
}

type RelatedResourceRule struct {
	ResourceRule          `json:",inline"`
	*metav1.LabelSelector `json:"labelSelector"`
	NamespaceSelector     *metav1.LabelSelector `json:"namespaceSelector,omitempty"`
	Namespace             string                `json:"namespace,omitempty"`
	Names                 []string              `json:"names"`
}

// CustomizableController is an interface representing Controller exposing customize hook
type CustomizableController interface {

	// GetCustomizeHook returns the customize Hook or nil if not defined.
	GetCustomizeHook() *Hook

	// GetConnections returns the webhook connection entries defined on the controller.
	GetConnections() []WebhookConnection
}
