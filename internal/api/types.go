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

package api

import "time"

// AlertDevice represents a device that has triggered an alert threshold
type AlertDevice struct {
	// DeviceID is the unique identifier for the device (Kubernetes resource name)
	DeviceID string `json:"deviceId"`

	// DeviceName is the friendly name of the device
	DeviceName string `json:"deviceName,omitempty"`

	// SensorType specifies what type of sensor this is
	SensorType string `json:"sensorType"`

	// Location describes where the device is physically located
	Location string `json:"location,omitempty"`

	// Room for grouping devices by room
	Room string `json:"room,omitempty"`

	// IEEEAddr is the IEEE address of the device
	IEEEAddr string `json:"ieeeAddr,omitempty"`

	// ShortAddr is the Zigbee short address
	ShortAddr string `json:"shortAddr,omitempty"`

	// AlertCondition describes the alert threshold that was triggered
	AlertCondition AlertConditionInfo `json:"alertCondition"`

	// CurrentValue is the current measurement value that triggered the alert
	CurrentValue *float64 `json:"currentValue,omitempty"`

	// LastMeasurement is when the last measurement was recorded
	LastMeasurement *time.Time `json:"lastMeasurement,omitempty"`

	// TriggeredAt is when the alert was first triggered
	TriggeredAt *time.Time `json:"triggeredAt,omitempty"`
}

// AlertConditionInfo contains information about the alert condition
type AlertConditionInfo struct {
	// Measurement is the name of the measurement being monitored
	Measurement string `json:"measurement"`

	// Operator defines the comparison operator (above, below, is)
	Operator string `json:"operator"`

	// Threshold is the value that triggers the alert
	Threshold string `json:"threshold"`
}

// AlertsResponse is the response for the alerts endpoint
type AlertsResponse struct {
	// Devices is the list of devices with triggered alerts
	Devices []AlertDevice `json:"devices"`

	// Count is the total number of devices with alerts
	Count int `json:"count"`

	// Timestamp is when this response was generated
	Timestamp time.Time `json:"timestamp"`
}

// ErrorResponse represents an error response
type ErrorResponse struct {
	// Error is the error message
	Error string `json:"error"`

	// Code is the HTTP status code
	Code int `json:"code"`

	// Timestamp is when the error occurred
	Timestamp time.Time `json:"timestamp"`
}
