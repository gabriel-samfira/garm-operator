// SPDX-License-Identifier: MIT

package v1beta1

import (
	"github.com/cloudbase/garm/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mercedes-benz/garm-operator/pkg/conditions"
)

// GiteaCredentialSpec defines the desired state of GiteaCredential
type GiteaCredentialSpec struct {
	Description string                           `json:"description"`
	EndpointRef corev1.TypedLocalObjectReference `json:"endpointRef"`

	// +kubebuilder:validation:Enum=pat
	AuthType params.ForgeAuthType `json:"authType"`

	// containing pat token
	SecretRef SecretRef `json:"secretRef,omitempty"`
}

// GiteaCredentialStatus defines the observed state of GiteaCredential
type GiteaCredentialStatus struct {
	ID            int64    `json:"id"`
	APIBaseURL    string   `json:"apiBaseUrl"`
	BaseURL       string   `json:"baseUrl"`
	Repositories  []string `json:"repositories,omitempty"`
	Organizations []string `json:"organizations,omitempty"`
	Enterprises   []string `json:"enterprises,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=giteacredentials,scope=Namespaced,categories=garm,shortName=gcred
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="ID",type="string",JSONPath=".status.id",description="Credentials ID"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
//+kubebuilder:printcolumn:name="AuthType",type="string",JSONPath=`.spec.authType`,description="Authentication type"
//+kubebuilder:printcolumn:name="GiteaEndpoint",type="string",JSONPath=`.spec.endpointRef.name`,description="GiteaEndpoint name these credentials are tied to"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of GiteaCredential"

// GiteaCredential is the Schema for the giteacredential API
type GiteaCredential struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GiteaCredentialSpec   `json:"spec,omitempty"`
	Status GiteaCredentialStatus `json:"status,omitempty"`
}

func (g *GiteaCredential) InitializeConditions() {
	if conditions.Get(g, conditions.ReadyCondition) == nil {
		conditions.MarkUnknown(g, conditions.ReadyCondition, conditions.UnknownReason, conditions.GarmServerNotReconciledYetMsg)
	}

	if conditions.Get(g, conditions.GiteaEndpointReference) == nil {
		conditions.MarkUnknown(g, conditions.GiteaEndpointReference, conditions.UnknownReason, conditions.GiteaEndpointNotReconciledYetMsg)
	}

	if conditions.Get(g, conditions.WebhookSecretReference) == nil {
		conditions.MarkUnknown(g, conditions.WebhookSecretReference, conditions.UnknownReason, conditions.WebhookSecretNotReconciledYetMsg)
	}
}

func (g *GiteaCredential) SetConditions(conditions []metav1.Condition) {
	g.Status.Conditions = conditions
}

func (g *GiteaCredential) GetConditions() []metav1.Condition {
	return g.Status.Conditions
}

//+kubebuilder:object:root=true

// GiteaCredentialList contains a list of GiteaCredential
type GiteaCredentialList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GiteaCredential `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GiteaCredential{}, &GiteaCredentialList{})
}
