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

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// DeviceSpec defines the desired state of Device
type DeviceSpec struct {
	// BridgeRef references the MQTTBridge this device belongs to
	// This field is set by the operator and should not be modified by users
	// +kubebuilder:validation:Required
	BridgeRef BridgeReference `json:"bridgeRef"`

	// IEEEAddr is the unique IEEE address of the device (set by operator)
	// This is the immutable identifier from Zigbee/MQTT discovery
	// +kubebuilder:validation:Required
	IEEEAddr string `json:"ieeeAddr"`

	// SensorType specifies what type of measurements this device provides
	// This determines which database and GORM handler to use for storing data
	// Supported types: "valve", "moisture", "power", "solar", "temperature", "humidity", "pressure", "water_level", "room"
	// +kubebuilder:validation:Enum=valve;moisture;power;solar;temperature;humidity;pressure;water_level;room
	// +optional
	SensorType string `json:"sensorType,omitempty"`

	// Corrections is an optional value that can be used to apply correction factors to the device's measurements
	// Values are stored as strings to ensure cross-language compatibility (e.g., "2.5", "-1.8")
	// +optional
	Corrections map[string]string `json:"corrections,omitempty"`

	// AlertCondition defines a condition that triggers an alert when met
	// When the condition evaluates to true, status.alert is set to true
	// +optional
	AlertCondition *AlertCondition `json:"alertCondition,omitempty"`

	// FriendlyName is a user-configurable name for the device
	// +optional
	FriendlyName string `json:"friendlyName,omitempty"`

	// Location describes where the device is physically located
	// +optional
	Location string `json:"location,omitempty"`

	// Room for grouping devices by room
	// +optional
	Room string `json:"room,omitempty"`

	// Disabled indicates whether measurements from this device should be ignored
	// +kubebuilder:default=false
	// +optional
	Disabled bool `json:"disabled,omitempty"`

	// MetadataLabels are custom key-value pairs for user-defined metadata
	// +optional
	MetadataLabels map[string]string `json:"metadataLabels,omitempty"`
}

// BridgeReference contains information to reference an MQTTBridge
type BridgeReference struct {
	// Name of the MQTTBridge
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace of the MQTTBridge. If not specified, uses the same namespace as the Device
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// AlertCondition defines a condition that triggers an alert
type AlertCondition struct {
	// Measurement is the name of the measurement to check (e.g., "temperature", "humidity")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Measurement string `json:"measurement"`

	// Operator defines the comparison operator
	// Supported operators: "above", "below", "is"
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=above;below;is
	Operator string `json:"operator"`

	// Value is the threshold value to compare against
	// Stored as string for cross-language compatibility (e.g., "25.0", "30.5")
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^-?\d+(\.\d+)?$`
	Value string `json:"value"`
}

// MeasurementValue represents a single measurement with its value, timestamp, and optional corrections
type MeasurementValue struct {
	// Value is the raw measurement value received from the device
	// Stored as string for cross-language compatibility
	// +kubebuilder:validation:Required
	Value string `json:"value"`

	// LastSeen is the timestamp when this measurement was last received
	// +kubebuilder:validation:Required
	LastSeen metav1.Time `json:"lastSeen"`

	// Correction is the correction value applied to this measurement (if any)
	// Only set if a correction was configured for this measurement
	// +optional
	Correction *string `json:"correction,omitempty"`

	// CorrectedValue is the value after applying the correction
	// Only set if a correction was applied
	// +optional
	CorrectedValue *string `json:"correctedValue,omitempty"`
}

// DeviceStatus defines the observed state of Device.
type DeviceStatus struct {
	// Alert show if this device triggered the alert condition
	// +optional
	Alert bool `json:"alert,omitempty"`

	// ShortAddr is the short Zigbee address (e.g., "0x4F2E")
	// This is used to map MQTT messages to devices
	// +optional
	ShortAddr string `json:"shortAddr,omitempty"`

	// ModelID is the device model identifier discovered from MQTT
	// +optional
	ModelID string `json:"modelId,omitempty"`

	// Manufacturer is the device manufacturer
	// +optional
	Manufacturer string `json:"manufacturer,omitempty"`

	// DeviceType indicates the type of device (e.g., sensor, actuator, switch)
	// +optional
	DeviceType string `json:"deviceType,omitempty"`

	// Capabilities lists what the device can measure or control
	// Examples: temperature, humidity, pressure, battery, water_leak, occupancy
	// +optional
	Capabilities []string `json:"capabilities,omitempty"`

	// LastSeen is the timestamp when the device last sent data
	// +optional
	LastSeen *metav1.Time `json:"lastSeen,omitempty"`

	// Reachable indicates whether the device is currently reachable
	// +optional
	Reachable *bool `json:"reachable,omitempty"`

	// Available indicates whether the device is currently reachable (deprecated, use Reachable)
	// +optional
	Available bool `json:"available,omitempty"`

	// LastMeasurement contains the most recent measurement data as JSON string
	// Deprecated: Use Measurements map instead for detailed measurement tracking
	// +optional
	LastMeasurement string `json:"lastMeasurement,omitempty"`

	// Measurements contains individual measurements with their values, timestamps, and corrections
	// Key is the measurement name (e.g., "temperature", "humidity", "pressure")
	// Value contains the measurement details including corrections if applied
	// +optional
	Measurements map[string]MeasurementValue `json:"measurements,omitempty"`

	// BatteryPercentage indicates the battery percentage (0-100) if applicable
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	BatteryPercentage *int32 `json:"batteryPercentage,omitempty"`

	// BatteryLevel indicates the battery percentage (0-100) if applicable (deprecated, use BatteryPercentage)
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=100
	BatteryLevel *int32 `json:"batteryLevel,omitempty"`

	// BatteryLastSeenEpoch is the Unix epoch timestamp when battery level was last updated
	// +optional
	BatteryLastSeenEpoch *int64 `json:"batteryLastSeenEpoch,omitempty"`

	// LastSeenSeconds is the number of seconds since the device was last seen
	// +optional
	LastSeenSeconds *int32 `json:"lastSeenSeconds,omitempty"`

	// LastSeenEpoch is the Unix epoch timestamp when the device was last seen
	// +optional
	LastSeenEpoch *int64 `json:"lastSeenEpoch,omitempty"`

	// LinkQuality indicates the signal quality (0-255)
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:validation:Maximum=255
	LinkQuality *int32 `json:"linkQuality,omitempty"`

	// LastPowerState contains the most recent power state for devices that support power control/sensing
	// This field provides a stable reference for monitoring valve states and other power-controlled devices
	// Typically 0 (off) or 1 (on), but may vary by device type
	// +optional
	LastPowerState *int32 `json:"lastPowerState,omitempty"`

	// Conditions represent the current state of the Device resource.
	// +listType=map
	// +listMapKey=type
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Short Addr",type=string,JSONPath=`.status.shortAddr`
// +kubebuilder:printcolumn:name="Friendly Name",type=string,JSONPath=`.spec.friendlyName`
// +kubebuilder:printcolumn:name="Sensor Type",type=string,JSONPath=`.spec.sensorType`
// +kubebuilder:printcolumn:name="Power",type=integer,JSONPath=`.status.lastPowerState`,priority=1
// +kubebuilder:printcolumn:name="Battery",type=integer,JSONPath=`.status.batteryPercentage`
// +kubebuilder:printcolumn:name="Link Quality",type=integer,JSONPath=`.status.linkQuality`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`
// +kubebuilder:printcolumn:name="Type",type=string,JSONPath=`.status.deviceType`
// +kubebuilder:printcolumn:name="Available",type=boolean,JSONPath=`.status.available`
// +kubebuilder:printcolumn:name="Battery",type=integer,JSONPath=`.status.batteryLevel`
// +kubebuilder:printcolumn:name="Last Seen",type=date,JSONPath=`.status.lastSeen`

// Device is the Schema for the devices API
type Device struct {
	metav1.TypeMeta `json:",inline"`

	// metadata is a standard object metadata
	// +optional
	metav1.ObjectMeta `json:"metadata,omitzero"`

	// spec defines the desired state of Device
	// +required
	Spec DeviceSpec `json:"spec"`

	// status defines the observed state of Device
	// +optional
	Status DeviceStatus `json:"status,omitzero"`
}

// +kubebuilder:object:root=true

// DeviceList contains a list of Device
type DeviceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitzero"`
	Items           []Device `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Device{}, &DeviceList{})
}
