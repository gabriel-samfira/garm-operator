// SPDX-License-Identifier: MIT

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	DefaultGarmServerImage = "ghcr.io/cloudbase/garm:v0.1.4"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// PersistentVolumeClaimSpecs defines the web server volume specs
type PersistentVolumeClaimSpec struct {
	// Name of the persistent volume claim
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`

	// CreateIfNotExists defines if the persistent volume claim should be created in case not found
	// +kubebuilder:default=true
	CreateIfNotExists bool `json:"createIfNotExists,omitempty"`

	// StorageSize defines the size of the new persistent volume claim
	// +kubebuilder:validation:Optional
	// +kubebuilder:default="10Gi"
	StorageSize string `json:"storageSize,omitempty"`

	// StorageClass is the storageClassName used to create a new persistent volume claim
	// +kubebuilder:validation:Optional
	// +kubebuilder:default=default
	StorageClass string `json:"storageClass,omitempty"`

	// MountPath defines the volume path inside the web server
	// +kubebuilder:validation:Required
	MountPath string `json:"mountPath,omitempty"`
}

type GarmServerSQLLiteSpec struct {
	// +kubebuilder:validation:Required
	PersistentVolumeClaim PersistentVolumeClaimSpec `json:"persistentVolumeClaim,omitempty"`
	// +kubebuilder:validation:Required
	DatabaseFileName string `json:"databaseFileName,omitempty"`
}

type GarmServerDatabaseSpec struct {
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Default=false
	Debug bool `json:"debug,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=sqlite
	// +kubebuilder:validation:Default=sqlite
	// BackendType - The database backend to use. Currently only SQLite is supported.
	BackendType string `json:"backend,omitempty"`
	// +kubebuilder:validation:Required
	// PassphraseSecretRef - The secret name that holds the database encryption passphrase.
	// The passphrase is used to encrypt some values in the database at rest. Sensitive info
	// like credentials.
	PassphraseSecretRef SecretRef `json:"passphraseSecretRef,omitempty"`
	// +kubebuilder:validation:Required
	// SQLiteBackend - Parameters related to the SQLite backend. Currently it's marked as required,
	// mostly because it's the only backend supported. In the future, this will be marked as optional.
	SQLiteBackend GarmServerSQLLiteSpec `json:"sqliteBackend,omitempty"`
}

type GarmServerDefaultSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=true
	EnableWebhookManagement bool `json:"enableWebhookManagement,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=false
	DebugServer bool `json:"debugServer,omitempty"`
}

type GarmServerLoggingSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=false
	EnableLogStreamer bool `json:"enableLogStreamer,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Enum=text;json
	// +kubebuilder:validation:Default=json
	LogFormat string `json:"logFormat,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=false
	LogSource bool `json:"logSource,omitempty"`
}

type GarmServerMetricsSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=true
	Enable bool `json:"enable,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=false
	DisableAuth bool `json:"disableAuth,omitempty"`
}

type GarmServerJWTSpec struct {
	// +kubebuilder:validation:Required
	JWTSecretRef SecretRef `json:"jwtSecretRef,omitempty"`
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=30
	// +kubebuilder:validation:Default=30
	// TimeToLive - The time to live in minutes for the JWT token. Minimum is 30 minutes.
	TimeToLive int `json:"timeToLive,omitempty"`
}

type GarmServerAPIServerSpec struct {
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=0.0.0.0
	// BindAddress - The address to bind the server to.
	BindAddress string `json:"bindAddress,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=9997
	// Port - The port to bind the server to.
	Port int `json:"port,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default={*}
	CORSOrigins []string `json:"corsOrigins,omitempty"`
	// +kubebuilder:validation:Optional
	// +kubebuilder:validation:Default=false
	UseTLS bool `json:"useTLS,omitempty"`
	// +kubebuilder:validation:Optional
	// +operator-sdk:csv:customresourcedefinitions:type=spec
	// TLS - Parameters related to the TLS
	TLS TLSSecretRef `json:"tls,omitempty"`
}

type TLSSecretRef struct {
	// Name of the kubernetes secret to use
	Name string `json:"name"`
	// CertificateBundle is the key in the secret that holds the certificate bundle.
	// The bundle needs to have the entire certificate chain, CA included.
	CertificateBundle string `json:"certificate_bundle"`
	// PrivateKey is the key in the secret that holds the private key.
	PrivateKey string `json:"private_key"`
}

// GarmServerSpec defines the desired state of GarmServer
type GarmServerSpec struct {
	// +kubebuilder:validation:Required
	// ContainerImage - GARM Container Image URL (will be set to environmental default if empty)
	ContainerImage string `json:"containerImage"`

	Default  GarmServerDefaultSpec   `json:"default,omitempty"`
	Database GarmServerDatabaseSpec  `json:"database,omitempty"`
	Logging  GarmServerLoggingSpec   `json:"logging,omitempty"`
	Metrics  GarmServerMetricsSpec   `json:"metrics,omitempty"`
	JWT      GarmServerJWTSpec       `json:"jwt,omitempty"`
	API      GarmServerAPIServerSpec `json:"api,omitempty"`
}

// GarmServerStatus defines the observed state of GarmServer
type GarmServerStatus struct {
	ID                   string `json:"id"`
	MetadataURL          string `json:"metadataURL,omitempty"`
	CallbackURL          string `json:"callbackURL,omitempty"`
	WebhookURL           string `json:"webhookURL,omitempty"`
	Version              string `json:"version,omitempty"`
	MinimumJobAgeBackoff int    `json:"minimumJobAgeBackoff,omitempty"`

	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// GarmServer is the Schema for the garmservers API
type GarmServer struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GarmServerSpec   `json:"spec,omitempty"`
	Status GarmServerStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// GarmServerList contains a list of GarmServer
type GarmServerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GarmServer `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GarmServer{}, &GarmServerList{})
}
