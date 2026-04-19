// SPDX-License-Identifier: MIT

package v1beta1

import (
	commonParams "github.com/cloudbase/garm-provider-common/params"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/mercedes-benz/garm-operator/pkg/conditions"
)

// ScaleSetSpec defines the desired state of ScaleSet
// +kubebuilder:validation:Required
// +kubebuilder:validation:XValidation:rule="self.minIdleRunners <= self.maxRunners",message="minIdleRunners must be less than or equal to maxRunners"
type ScaleSetSpec struct {
	// Defines in which Scope Runners are registered. Has a reference to either an Enterprise, Org or Repo CRD
	GitHubScopeRef corev1.TypedLocalObjectReference `json:"githubScopeRef"`

	Name          string `json:"name"`
	DisableUpdate bool   `json:"disableUpdate,omitempty"`

	ProviderName   string              `json:"providerName"`
	MaxRunners     uint                `json:"maxRunners"`
	MinIdleRunners uint                `json:"minIdleRunners"`
	ImageName      string              `json:"imageName"`
	Flavor         string              `json:"flavor"`
	OSType         commonParams.OSType `json:"osType"`
	OSArch         commonParams.OSArch `json:"osArch"`
	Enabled        bool                `json:"enabled"`

	RunnerBootstrapTimeout uint `json:"runnerBootstrapTimeout"`

	// +optional
	ExtraSpecs string `json:"extraSpecs"`

	// +optional
	EnableShell bool `json:"enableShell,omitempty"`

	// +optional
	GitHubRunnerGroup string `json:"githubRunnerGroup"`

	// +optional
	RunnerPrefix string `json:"runnerPrefix"`

	// +optional
	TemplateID *uint `json:"templateId,omitempty"`
}

// ScaleSetStatus defines the observed state of ScaleSet
type ScaleSetStatus struct {
	ID         string `json:"id"`
	ScaleSetID int    `json:"scaleSetId,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

func (s *ScaleSet) InitializeConditions() {
	if conditions.Get(s, conditions.ReadyCondition) == nil {
		conditions.MarkUnknown(s, conditions.ReadyCondition, conditions.UnknownReason, conditions.GarmServerNotReconciledYetMsg)
	}
}

func (s *ScaleSet) SetConditions(conditions []metav1.Condition) {
	s.Status.Conditions = conditions
}

func (s *ScaleSet) GetConditions() []metav1.Condition {
	return s.Status.Conditions
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:resource:path=scalesets,scope=Namespaced,categories=garm,shortName=ss
//+kubebuilder:printcolumn:name="ID",type=string,JSONPath=`.status.id`
//+kubebuilder:printcolumn:name="MinIdleRunners",type=string,JSONPath=`.spec.minIdleRunners`
//+kubebuilder:printcolumn:name="MaxRunners",type=string,JSONPath=`.spec.maxRunners`
//+kubebuilder:printcolumn:name="ImageName",type=string,JSONPath=`.spec.imageName`,priority=1
//+kubebuilder:printcolumn:name="Flavor",type=string,JSONPath=`.spec.flavor`,priority=1
//+kubebuilder:printcolumn:name="Provider",type=string,JSONPath=`.spec.providerName`,priority=1
//+kubebuilder:printcolumn:name="ScopeType",type=string,JSONPath=`.spec.githubScopeRef.kind`,priority=1
//+kubebuilder:printcolumn:name="ScopeName",type=string,JSONPath=`.spec.githubScopeRef.name`,priority=1
//+kubebuilder:printcolumn:name="Ready",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].status"
//+kubebuilder:printcolumn:name="Error",type="string",JSONPath=".status.conditions[?(@.type=='Ready')].message",priority=1
//+kubebuilder:printcolumn:name="Enabled",type=boolean,JSONPath=`.spec.enabled`,priority=1
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// ScaleSet is the Schema for the scalesets API
type ScaleSet struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ScaleSetSpec   `json:"spec,omitempty"`
	Status ScaleSetStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// ScaleSetList contains a list of ScaleSet
type ScaleSetList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ScaleSet `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ScaleSet{}, &ScaleSetList{})
}
