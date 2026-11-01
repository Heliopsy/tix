// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// ShowKey renders a project key the way a reader sees it: upper case. Keys are
// stored, addressed and serialised in lower case; only rendering changes, so a
// URL, a JSON value or a filter written against the stored form keeps working.
func ShowKey(key string) string { return strings.ToUpper(key) }

// ShowRef renders a task reference with its project key upper case, as INFRA-8.
// Anything that is not a key-and-number reference, such as a stable identifier
// standing in for a ref the event did not record, is returned untouched.
func ShowRef(ref string) string {
	r, err := core.ParseTaskRef(ref)
	if err != nil || r.IsID() {
		return ref
	}
	return strings.ToUpper(ref)
}
