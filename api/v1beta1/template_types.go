// SPDX-License-Identifier: MIT

package v1beta1

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
	"github.com/cloudbase/garm/params"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mercedes-benz/garm-operator/pkg/conditions"
)

// TemplateSpec defines the desired state of Template
type TemplateSpec struct {
	Description string              `json:"description,omitempty"`
	OSType      commonParams.OSType `json:"osType"`
	ForgeType   params.EndpointType `json:"forgeType,omitempty"`

	// DataSecretRef references a secret containing the template data under the specified key
	DataSecretRef SecretRef `json:"dataSecretRef"`
}

// TemplateStatus defines the observed state of Template
type TemplateStatus struct {
	ID uint `json:"id"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (t *Template) InitializeConditions() {
	if conditions.Get(t, conditions.ReadyCondition) == nil {
		conditions.MarkUnknown(t, conditions.ReadyCondition, conditions.UnknownReason, conditions.GarmServerNotReconciledYetMsg)
	}

	if conditions.Get(t, conditions.WebhookSecretReference) == nil {
		conditions.MarkUnknown(t, conditions.WebhookSecretReference, conditions.UnknownReason, conditions.WebhookSecretNotReconciledYetMsg)
	}
}

func (t *Template) SetConditions(conditions []metav1.Condition) {
	t.Status.Conditions = conditions
}

func (t *Template) GetConditions() []metav1.Condition {
	return t.Status.Conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:resource:path=templates,scope=Namespaced,categories=garm,shortName=tmpl
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="ID",type="integer",JSONPath=".status.id",description="Template ID"
//+kubebuilder:printcolumn:name="OSType",type="string",JSONPath=`.spec.osType`,description="OS type"
//+kubebuilder:printcolumn:name="ForgeType",type="string",JSONPath=`.spec.forgeType`,description="Forge type"
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp",description="Time duration since creation of Template"

// Template is the Schema for the templates API
type Template struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TemplateSpec   `json:"spec,omitempty"`
	Status TemplateStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// TemplateList contains a list of Template
type TemplateList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Template `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Template{}, &TemplateList{})
}
