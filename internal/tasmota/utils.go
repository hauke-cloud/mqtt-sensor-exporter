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

package tasmota

import (
	"strings"

	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
)

// sanitizeDeviceName converts an IEEE address or device name to a valid Kubernetes resource name
func sanitizeDeviceName(name string) string {
	// Remove "0x" prefix if present
	name = strings.TrimPrefix(strings.ToLower(name), "0x")

	// Replace invalid characters with hyphens
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Remove leading/trailing hyphens
	name = strings.Trim(name, "-")

	// Ensure it starts with alphanumeric
	if len(name) > 0 && (name[0] < 'a' || name[0] > 'z') && (name[0] < '0' || name[0] > '9') {
		name = "device-" + name
	}

	// Add prefix to make it more readable
	if !strings.HasPrefix(name, "device-") {
		name = "device-" + name
	}

	// Limit length (Kubernetes resource names max 253 chars)
	if len(name) > 253 {
		name = name[:253]
	}

	return name
}

// sanitizeLabel converts a value to a valid Kubernetes label value
func sanitizeLabel(value string) string {
	// Remove "0x" prefix
	value = strings.TrimPrefix(strings.ToLower(value), "0x")

	// Labels must be alphanumeric, '-', '_', or '.'
	value = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			return r
		}
		return '-'
	}, value)

	// Max length 63 characters
	if len(value) > 63 {
		value = value[:63]
	}

	return value
}

// applyCorrectionToFloat applies a correction value to a float64 measurement
// If a correction exists for the given key in the device's corrections map, it will be added to the value
func applyCorrectionToFloat(value float64, correctionKey string, device *mqttv1alpha1.Device) float64 {
	if device == nil || device.Spec.Corrections == nil {
		return value
	}

	if correction, exists := device.Spec.Corrections[correctionKey]; exists {
		return value + correction
	}

	return value
}

// applyCorrectionToInt applies a correction value to an int measurement
// If a correction exists for the given key in the device's corrections map, it will be added to the value
func applyCorrectionToInt(value int, correctionKey string, device *mqttv1alpha1.Device) int {
	if device == nil || device.Spec.Corrections == nil {
		return value
	}

	if correction, exists := device.Spec.Corrections[correctionKey]; exists {
		return value + int(correction)
	}

	return value
}

// evaluateAlertCondition checks if an alert condition is met for a given measurement
// Returns true if the condition is met, false otherwise
func evaluateAlertCondition(measurementValue float64, condition *mqttv1alpha1.AlertCondition) bool {
	if condition == nil {
		return false
	}

	switch condition.Operator {
	case "above":
		return measurementValue > condition.Value
	case "below":
		return measurementValue < condition.Value
	case "is":
		return measurementValue == condition.Value
	default:
		return false
	}
}

// checkAlertConditions evaluates alert conditions for all measurements in a device
// Returns true if any alert condition is met
func checkAlertConditions(measurements map[string]any, device *mqttv1alpha1.Device) bool {
	if device == nil || device.Spec.AlertCondition == nil {
		return false
	}

	condition := device.Spec.AlertCondition

	// Get the measurement value for the specified measurement
	measurementValue, exists := measurements[condition.Measurement]
	if !exists {
		return false
	}

	// Convert measurement value to float64
	var floatValue float64
	switch v := measurementValue.(type) {
	case float64:
		floatValue = v
	case int:
		floatValue = float64(v)
	case int32:
		floatValue = float64(v)
	case int64:
		floatValue = float64(v)
	default:
		// Can't evaluate non-numeric values
		return false
	}

	return evaluateAlertCondition(floatValue, condition)
}
