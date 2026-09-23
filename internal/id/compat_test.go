// SPDX-License-Identifier: AGPL-3.0-or-later

package id_test

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
)

// TestGeneratedIDsParseAsTaskRefs pins a cross-package invariant that is easy
func TestGeneratedIDsParseAsTaskRefs(t *testing.T) {
	for range 2000 {
		v := id.New()

		ref, err := core.ParseTaskRef(v)
		if err != nil {
			t.Fatalf("generated identifier %q does not parse as a task reference: %v", v, err)
		}
		if !ref.IsID() {
			t.Fatalf("generated identifier %q parsed as the human form %+v", v, ref)
		}
		if ref.ID != v {
			t.Fatalf("round trip changed the identifier: %q became %q", v, ref.ID)
		}
	}
}

// TestGeneratedIDsAreNeverMistakenForHumanRefs guards the other direction: a
func TestGeneratedIDsAreNeverMistakenForHumanRefs(t *testing.T) {
	for range 2000 {
		v := id.New()
		for _, c := range v {
			if c == '-' {
				t.Fatalf("generated identifier %q contains a hyphen, which collides with the human reference form", v)
			}
		}
	}
}
