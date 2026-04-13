/*
Copyright 2026 hauke.cloud.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DatabaseSpec defines the desired state of Database
type DatabaseSpec struct {
	// Host is the database hostname or IP address
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Host string `json:"host"`

	// Port is the database port
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default=5432
	Port int32 `json:"port"`

	// Database name to connect to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Database string `json:"database"`

	// Username for database authentication
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Username string `json:"username"`

	// PasswordSecretRef is a reference to a Kubernetes Secret containing the database password
	// The secret should contain key: "password"
	// Required if ClientCertSecretRef is not specified
	// +optional
	PasswordSecretRef *SecretReference `json:"passwordSecretRef,omitempty"`

	// ClientCertSecretRef is a reference to a Kubernetes Secret containing client certificate
	// The secret should contain keys: "tls.crt" (client certificate) and "tls.key" (client key)
	// Required if PasswordSecretRef is not specified
	// +optional
	ClientCertSecretRef *SecretReference `json:"clientCertSecretRef,omitempty"`

	// SSLMode determines the SSL/TLS connection mode
	// Supported modes: "disable", "require", "verify-ca", "verify-full"
	// +kubebuilder:validation:Enum=disable;require;verify-ca;verify-full
	// +kubebuilder:default="require"
	// +optional
	SSLMode string `json:"sslMode,omitempty"`

	// CASecretRef is a reference to a Kubernetes Secret containing the CA certificate
	// The secret should contain key: "ca.crt"
	// Required for sslMode "verify-ca" or "verify-full"
	// +optional
	CASecretRef *SecretReference `json:"caSecretRef,omitempty"`

	// SupportedSensorTypes specifies which sensor types this database can handle
	// Devices with matching sensorType will have their measurements stored in this database
	// Each sensor type will have its own GORM handler and table(s)
	// Supported types: "irrigation", "power", "solar", "temperature", "humidity", "pressure"
	// +kubebuilder:validation:MinItems=1
	// +optional
	SupportedSensorTypes []string `json:"supportedSensorTypes,omitempty"`

	// MaxConnections is the maximum number of connections in the pool
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	// +kubebuilder:default=10
	// +optional
	MaxConnections *int32 `json:"maxConnections,omitempty"`

	// MinConnections is the minimum number of connections in the pool
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=10
	// +kubebuilder:default=2
	// +optional
	MinConnections *int32 `json:"minConnections,omitempty"`

	// BatchSize is the number of measurements to batch before writing
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +kubebuilder:default=100
	// +optional
	BatchSize *int32 `json:"batchSize,omitempty"`

	// BatchTimeout is the maximum time to wait before writing a partial batch (in seconds)
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=300
	// +kubebuilder:default=10
	// +optional
	BatchTimeout *int32 `json:"batchTimeout,omitempty"`
}

// DatabaseStatus defines the observed state of Database
type DatabaseStatus struct {
	// ConnectionState indicates the current database connection state
	// +kubebuilder:validation:Enum=Connected;Disconnected;Error;Initializing
	// +optional
	ConnectionState string `json:"connectionState,omitempty"`

	// LastConnectedTime is the timestamp of the last successful connection
	// +optional
	LastConnectedTime *metav1.Time `json:"lastConnectedTime,omitempty"`

	// Message provides additional information about the connection state
	// +optional
	Message string `json:"message,omitempty"`

	// MeasurementsWritten is the total number of measurements written to this database
	// +optional
	MeasurementsWritten int64 `json:"measurementsWritten,omitempty"`

	// LastWriteTime is the timestamp of the last successful write
	// +optional
	LastWriteTime *metav1.Time `json:"lastWriteTime,omitempty"`

	// PendingBatch is the number of measurements waiting to be written
	// +optional
	PendingBatch int32 `json:"pendingBatch,omitempty"`

	// Conditions represent the latest available observations of the database's state
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:shortName=db;dbs
// +kubebuilder:printcolumn:name="Host",type=string,JSONPath=`.spec.host`
// +kubebuilder:printcolumn:name="Database",type=string,JSONPath=`.spec.database`
// +kubebuilder:printcolumn:name="Supports",type=string,JSONPath=`.spec.supportedSensorTypes`
// +kubebuilder:printcolumn:name="State",type=string,JSONPath=`.status.connectionState`
// +kubebuilder:printcolumn:name="Measurements",type=integer,JSONPath=`.status.measurementsWritten`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Database is the Schema for the databases API
type Database struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DatabaseSpec   `json:"spec,omitempty"`
	Status DatabaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// DatabaseList contains a list of Database
type DatabaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Database `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Database{}, &DatabaseList{})
}
