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
	"testing"

	mqttv1alpha1 "github.com/hauke-cloud/mqtt-sensor-exporter/api/v1alpha1"
)

func TestApplyCorrectionToFloat(t *testing.T) {
	tests := []struct {
		name          string
		value         float64
		correctionKey string
		device        *mqttv1alpha1.Device
		expected      float64
	}{
		{
			name:          "No device",
			value:         20.0,
			correctionKey: "temperature",
			device:        nil,
			expected:      20.0,
		},
		{
			name:          "No corrections map",
			value:         20.0,
			correctionKey: "temperature",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: nil,
				},
			},
			expected: 20.0,
		},
		{
			name:          "Empty corrections map",
			value:         20.0,
			correctionKey: "temperature",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{},
				},
			},
			expected: 20.0,
		},
		{
			name:          "Correction key not found",
			value:         20.0,
			correctionKey: "temperature",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"humidity": 5.0,
					},
				},
			},
			expected: 20.0,
		},
		{
			name:          "Positive correction",
			value:         20.0,
			correctionKey: "temperature",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"temperature": 2.5,
					},
				},
			},
			expected: 22.5,
		},
		{
			name:          "Negative correction",
			value:         20.0,
			correctionKey: "temperature",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"temperature": -3.2,
					},
				},
			},
			expected: 16.8,
		},
		{
			name:          "Multiple corrections, using correct key",
			value:         50.0,
			correctionKey: "humidity",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"temperature": 2.0,
						"humidity":    -10.5,
						"pressure":    5.0,
					},
				},
			},
			expected: 39.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyCorrectionToFloat(tt.value, tt.correctionKey, tt.device)
			if result != tt.expected {
				t.Errorf("applyCorrectionToFloat() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestApplyCorrectionToInt(t *testing.T) {
	tests := []struct {
		name          string
		value         int
		correctionKey string
		device        *mqttv1alpha1.Device
		expected      int
	}{
		{
			name:          "No device",
			value:         100,
			correctionKey: "battery_percentage",
			device:        nil,
			expected:      100,
		},
		{
			name:          "No corrections map",
			value:         100,
			correctionKey: "battery_percentage",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: nil,
				},
			},
			expected: 100,
		},
		{
			name:          "Positive correction",
			value:         95,
			correctionKey: "battery_percentage",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"battery_percentage": 5.0,
					},
				},
			},
			expected: 100,
		},
		{
			name:          "Negative correction",
			value:         100,
			correctionKey: "link_quality",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"link_quality": -10.0,
					},
				},
			},
			expected: 90,
		},
		{
			name:          "Fractional correction (truncated)",
			value:         100,
			correctionKey: "battery_percentage",
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					Corrections: map[string]float64{
						"battery_percentage": 2.7,
					},
				},
			},
			expected: 102, // int(2.7) = 2, so 100 + 2 = 102
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := applyCorrectionToInt(tt.value, tt.correctionKey, tt.device)
			if result != tt.expected {
				t.Errorf("applyCorrectionToInt() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestSanitizeDeviceName(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Simple hex address",
			input:    "0x4F2E",
			expected: "device-4f2e",
		},
		{
			name:     "Without 0x prefix",
			input:    "abcd1234",
			expected: "device-abcd1234",
		},
		{
			name:     "With special characters",
			input:    "test_device@123",
			expected: "device-test-device-123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeDeviceName(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeDeviceName() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestEvaluateAlertCondition(t *testing.T) {
	tests := []struct {
		name             string
		measurementValue float64
		condition        *mqttv1alpha1.AlertCondition
		expected         bool
	}{
		{
			name:             "Nil condition",
			measurementValue: 25.0,
			condition:        nil,
			expected:         false,
		},
		{
			name:             "Above - condition met",
			measurementValue: 26.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "above",
				Value:       25.0,
			},
			expected: true,
		},
		{
			name:             "Above - condition not met",
			measurementValue: 24.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "above",
				Value:       25.0,
			},
			expected: false,
		},
		{
			name:             "Above - exactly at threshold",
			measurementValue: 25.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "above",
				Value:       25.0,
			},
			expected: false,
		},
		{
			name:             "Below - condition met",
			measurementValue: 3.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "below",
				Value:       4.0,
			},
			expected: true,
		},
		{
			name:             "Below - condition not met",
			measurementValue: 5.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "below",
				Value:       4.0,
			},
			expected: false,
		},
		{
			name:             "Below - exactly at threshold",
			measurementValue: 4.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "below",
				Value:       4.0,
			},
			expected: false,
		},
		{
			name:             "Is - condition met",
			measurementValue: 0.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "power",
				Operator:    "is",
				Value:       0.0,
			},
			expected: true,
		},
		{
			name:             "Is - condition not met",
			measurementValue: 1.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "power",
				Operator:    "is",
				Value:       0.0,
			},
			expected: false,
		},
		{
			name:             "Invalid operator",
			measurementValue: 25.0,
			condition: &mqttv1alpha1.AlertCondition{
				Measurement: "temperature",
				Operator:    "invalid",
				Value:       25.0,
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := evaluateAlertCondition(tt.measurementValue, tt.condition)
			if result != tt.expected {
				t.Errorf("evaluateAlertCondition() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestCheckAlertConditions(t *testing.T) {
	tests := []struct {
		name         string
		measurements map[string]any
		device       *mqttv1alpha1.Device
		expected     bool
	}{
		{
			name: "Nil device",
			measurements: map[string]any{
				"temperature": 26.0,
			},
			device:   nil,
			expected: false,
		},
		{
			name: "No alert condition",
			measurements: map[string]any{
				"temperature": 26.0,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: nil,
				},
			},
			expected: false,
		},
		{
			name: "Measurement not present",
			measurements: map[string]any{
				"humidity": 50.0,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "temperature",
						Operator:    "above",
						Value:       25.0,
					},
				},
			},
			expected: false,
		},
		{
			name: "Temperature above threshold - alert triggered",
			measurements: map[string]any{
				"temperature": 26.5,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "temperature",
						Operator:    "above",
						Value:       25.0,
					},
				},
			},
			expected: true,
		},
		{
			name: "Temperature below threshold - no alert",
			measurements: map[string]any{
				"temperature": 24.0,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "temperature",
						Operator:    "above",
						Value:       25.0,
					},
				},
			},
			expected: false,
		},
		{
			name: "Humidity below threshold - alert triggered",
			measurements: map[string]any{
				"humidity": 25.0,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "humidity",
						Operator:    "below",
						Value:       30.0,
					},
				},
			},
			expected: true,
		},
		{
			name: "Power state is 0 - alert triggered",
			measurements: map[string]any{
				"power": 0,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "power",
						Operator:    "is",
						Value:       0.0,
					},
				},
			},
			expected: true,
		},
		{
			name: "Integer value conversion",
			measurements: map[string]any{
				"link_quality": 50,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "link_quality",
						Operator:    "below",
						Value:       60.0,
					},
				},
			},
			expected: true,
		},
		{
			name: "Non-numeric value - no alert",
			measurements: map[string]any{
				"contact": true,
			},
			device: &mqttv1alpha1.Device{
				Spec: mqttv1alpha1.DeviceSpec{
					AlertCondition: &mqttv1alpha1.AlertCondition{
						Measurement: "contact",
						Operator:    "is",
						Value:       1.0,
					},
				},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := checkAlertConditions(tt.measurements, tt.device)
			if result != tt.expected {
				t.Errorf("checkAlertConditions() = %v, want %v", result, tt.expected)
			}
		})
	}
}
