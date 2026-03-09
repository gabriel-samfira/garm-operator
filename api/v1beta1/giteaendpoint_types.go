// SPDX-License-Identifier: MIT

package v1beta1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mercedes-benz/garm-operator/pkg/conditions"
)

// GiteaEndpointSpec defines the desired state of GiteaEndpoint
type GiteaEndpointSpec struct {
	Description              string    `json:"description,omitempty"`
	APIBaseURL               string    `json:"apiBaseUrl,omitempty"`
	BaseURL                  string    `json:"baseUrl,omitempty"`
	CACertBundleSecretRef    SecretRef `json:"caCertBundleSecretRef,omitempty"`
	ToolsMetadataURL         string    `json:"toolsMetadataUrl,omitempty"`
	UseInternalToolsMetadata *bool     `json:"useInternalToolsMetadata,omitempty"`
}

// GiteaEndpointStatus defines the observed state of GiteaEndpoint
type GiteaEndpointStatus struct {
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=giteaendpoints,scope=Namespaced,categories=garm,shortName=gep
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="URL",type="string",JSONPath=".spec.apiBaseUrl",description="API Base URL"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of GiteaEndpoint"

// GiteaEndpoint is the Schema for the giteaendpoints API
type GiteaEndpoint struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaEndpointSpec   `json:"spec,omitempty"`
	Status GiteaEndpointStatus `json:"status,omitempty"`
}

func (e *GiteaEndpoint) InitializeConditions() {
	if conditions.Get(e, conditions.ReadyCondition) == nil {
		conditions.MarkUnknown(e, conditions.ReadyCondition, conditions.UnknownReason, conditions.GarmServerNotReconciledYetMsg)
	}

	if conditions.Get(e, conditions.WebhookSecretReference) == nil {
		conditions.MarkUnknown(e, conditions.WebhookSecretReference, conditions.UnknownReason, conditions.WebhookSecretNotReconciledYetMsg)
	}
}

func (e *GiteaEndpoint) SetConditions(conditions []metav1.Condition) {
	e.Status.Conditions = conditions
}

func (e *GiteaEndpoint) GetConditions() []metav1.Condition {
	return e.Status.Conditions
}

//+kubebuilder:object:root=true

// GiteaEndpointList contains a list of GiteaEndpoint
type GiteaEndpointList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaEndpoint `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaEndpoint{}, &GiteaEndpointList{})
}
