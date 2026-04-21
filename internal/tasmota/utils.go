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
	"reflect"
	"strconv"
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
// Corrections are stored as strings and parsed to float64
func applyCorrectionToFloat(value float64, correctionKey string, device *mqttv1alpha1.Device) float64 {
	if device == nil || device.Spec.Corrections == nil {
		return value
	}

	if correctionStr, exists := device.Spec.Corrections[correctionKey]; exists {
		correction, err := strconv.ParseFloat(correctionStr, 64)
		if err != nil {
			// If parsing fails, return original value
			return value
		}
		return value + correction
	}

	return value
}

// applyCorrectionToInt applies a correction value to an int measurement
// If a correction exists for the given key in the device's corrections map, it will be added to the value
// Corrections are stored as strings and parsed to float64, then converted to int
func applyCorrectionToInt(value int, correctionKey string, device *mqttv1alpha1.Device) int {
	if device == nil || device.Spec.Corrections == nil {
		return value
	}

	if correctionStr, exists := device.Spec.Corrections[correctionKey]; exists {
		correction, err := strconv.ParseFloat(correctionStr, 64)
		if err != nil {
			// If parsing fails, return original value
			return value
		}
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

	// Parse the threshold value from string
	thresholdValue, err := strconv.ParseFloat(condition.Value, 64)
	if err != nil {
		// If parsing fails, condition cannot be evaluated
		return false
	}

	switch condition.Operator {
	case "above":
		return measurementValue > thresholdValue
	case "below":
		return measurementValue < thresholdValue
	case "is":
		return measurementValue == thresholdValue
	default:
		return false
	}
}

// checkAlertConditions evaluates alert conditions using the device's measurements map
// Returns true if any alert condition is met
// This function uses the corrected values from status.measurements for evaluation
func checkAlertConditions(device *mqttv1alpha1.Device) bool {
	if device == nil || device.Spec.AlertCondition == nil {
		return false
	}

	// Return false if measurements map is not populated yet
	if device.Status.Measurements == nil {
		return false
	}

	condition := device.Spec.AlertCondition

	// Get the measurement from the status.measurements map
	mv, exists := device.Status.Measurements[condition.Measurement]
	if !exists {
		return false
	}

	// Use corrected value if available, otherwise use raw value
	valueStr := mv.Value
	if mv.CorrectedValue != nil {
		valueStr = *mv.CorrectedValue
	}

	// Parse the measurement value to float64
	measurementValue, err := strconv.ParseFloat(valueStr, 64)
	if err != nil {
		// If parsing fails, cannot evaluate (e.g., boolean values)
		return false
	}

	return evaluateAlertCondition(measurementValue, condition)
}

// formatFloat formats a float64 value as a string
func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}

// formatInt formats an int value as a string
func formatInt(value int) string {
	return strconv.Itoa(value)
}

// formatBool formats a bool value as a string
func formatBool(value bool) string {
	return strconv.FormatBool(value)
}

// getFieldValue extracts a field value from ZigbeeDevice using the field name
// Returns the dereferenced value and true if the field exists and is non-nil
func getFieldValue(zbDevice *ZigbeeDevice, fieldName string) (any, bool) {
	if zbDevice == nil {
		return nil, false
	}

	// Use reflection to get field value
	v := reflect.ValueOf(*zbDevice)
	field := v.FieldByName(fieldName)

	// Check if field exists
	if !field.IsValid() {
		return nil, false
	}

	// Handle pointer types
	if field.Kind() == reflect.Ptr {
		if field.IsNil() {
			return nil, false
		}
		// Dereference pointer
		return field.Elem().Interface(), true
	}

	// Handle non-pointer types
	return field.Interface(), true
}
