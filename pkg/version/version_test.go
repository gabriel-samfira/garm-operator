// SPDX-License-Identifier: MIT

package version

import "testing"

func TestCompare(t *testing.T) {
	tests := []struct {
		name    string
		version string
		isValid bool
	}{
		{
			name:    "is equal",
			version: "v0.2.0-alpha",
			isValid: true,
		},
		{
			name:    "garm is newer",
			version: "v0.2.1",
			isValid: true,
		},
		{
			name:    "garm version too old",
			version: "v0.1.5",
			isValid: false,
		},
		{
			name:    "garm is based on custom build without using build tags",
			version: "v0.0.0-unknown.",
			isValid: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EnsureMinimalVersion(tt.version); got != tt.isValid {
				t.Errorf("Compare() = %v, want %v", got, tt.isValid)
			}
		})
	}
}
