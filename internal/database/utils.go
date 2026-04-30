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

import "strings"

// sanitizeString removes control characters that PostgreSQL cannot encode in text fields
func sanitizeString(s string) string {
	return strings.Map(func(r rune) rune {
		// Remove control characters (0x00-0x1F and 0x7F)
		if r < 0x20 || r == 0x7F {
			return -1
		}
		return r
	}, s)
}
