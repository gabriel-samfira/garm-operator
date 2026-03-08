// SPDX-License-Identifier: MIT

package version

import "golang.org/x/mod/semver"

const MinVersion = "v0.2.0-alpha"

// EnsureMinimalVersion checks if the given version is greater than or equal to the minimum required version
func EnsureMinimalVersion(version string) bool {
	return semver.Compare(version, MinVersion) >= 0
}
