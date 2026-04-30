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

package database

import (
	"testing"
)

func TestSanitizeString(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal string",
			input:    "Hello World",
			expected: "Hello World",
		},
		{
			name:     "string with carriage return",
			input:    "Hello\rWorld",
			expected: "HelloWorld",
		},
		{
			name:     "string with newline",
			input:    "Hello\nWorld",
			expected: "HelloWorld",
		},
		{
			name:     "string with tab",
			input:    "Hello\tWorld",
			expected: "HelloWorld",
		},
		{
			name:     "string with null character",
			input:    "Hello\x00World",
			expected: "HelloWorld",
		},
		{
			name:     "string with DEL character",
			input:    "Hello\x7FWorld",
			expected: "HelloWorld",
		},
		{
			name:     "hexadecimal address with control chars",
			input:    "0x\rB3CC",
			expected: "0xB3CC",
		},
		{
			name:     "string with multiple control characters",
			input:    "test\x01\x02\x03device",
			expected: "testdevice",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "only control characters",
			input:    "\x00\r\n\t",
			expected: "",
		},
		{
			name:     "unicode string",
			input:    "Hello 世界",
			expected: "Hello 世界",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeString(tt.input)
			if result != tt.expected {
				t.Errorf("sanitizeString(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
