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

	iotv1alpha1 "github.com/hauke-cloud/kubernetes-iot-api/api/v1alpha1"
)

func TestApplyCorrectionToFloat(t *testing.T) {
	tests := []struct {
		name          string
		value         float64
		correctionKey string
		device        *iotv1alpha1.Device
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
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: nil,
				},
			},
			expected: 20.0,
		},
		{
			name:          "Empty corrections map",
			value:         20.0,
			correctionKey: "temperature",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{},
				},
			},
			expected: 20.0,
		},
		{
			name:          "Correction key not found",
			value:         20.0,
			correctionKey: "temperature",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"humidity": "5.0",
					},
				},
			},
			expected: 20.0,
		},
		{
			name:          "Positive correction",
			value:         20.0,
			correctionKey: "temperature",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"temperature": "2.5",
					},
				},
			},
			expected: 22.5,
		},
		{
			name:          "Negative correction",
			value:         20.0,
			correctionKey: "temperature",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"temperature": "-3.2",
					},
				},
			},
			expected: 16.8,
		},
		{
			name:          "Multiple corrections, using correct key",
			value:         50.0,
			correctionKey: "humidity",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"temperature": "2.0",
						"humidity":    "-10.5",
						"pressure":    "5.0",
					},
				},
			},
			expected: 39.5,
		},
		{
			name:          "Invalid correction string",
			value:         20.0,
			correctionKey: "temperature",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"temperature": "invalid",
					},
				},
			},
			expected: 20.0, // Should return original value if parsing fails
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
		device        *iotv1alpha1.Device
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
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: nil,
				},
			},
			expected: 100,
		},
		{
			name:          "Positive correction",
			value:         95,
			correctionKey: "battery_percentage",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"battery_percentage": "5.0",
					},
				},
			},
			expected: 100,
		},
		{
			name:          "Negative correction",
			value:         100,
			correctionKey: "link_quality",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"link_quality": "-10.0",
					},
				},
			},
			expected: 90,
		},
		{
			name:          "Fractional correction (truncated)",
			value:         100,
			correctionKey: "battery_percentage",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"battery_percentage": "2.7",
					},
				},
			},
			expected: 102, // int(2.7) = 2, so 100 + 2 = 102
		},
		{
			name:          "Invalid correction string",
			value:         100,
			correctionKey: "battery_percentage",
			device: &iotv1alpha1.Device{
				Spec: iotv1alpha1.DeviceSpec{
					Corrections: map[string]string{
						"battery_percentage": "not_a_number",
					},
				},
			},
			expected: 100, // Should return original value if parsing fails
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

// Helper function to create string pointers
func strPtr(s string) *string {
	return &s
}
