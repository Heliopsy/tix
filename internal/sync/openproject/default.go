// SPDX-License-Identifier: AGPL-3.0-or-later

package openproject

import _ "embed"

//go:embed default.yaml
var defaultMapping []byte

// DefaultMapping returns a starting mapping for OpenProject work packages; its
// project is the placeholder CHANGE-ME, which must be replaced before a run.
func DefaultMapping() []byte {
	return append([]byte(nil), defaultMapping...)
}
