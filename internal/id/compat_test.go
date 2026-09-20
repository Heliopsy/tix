package id_test

import (
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/id"
)

// TestGeneratedIDsParseAsTaskRefs pins a cross-package invariant that is easy
// to break from either side: core decides what a stable identifier looks like,
// id decides what one contains. If the alphabet or length here ever drifted
// outside what core accepts, every generated identifier would be rejected the
// moment it was used as a task reference.
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
// human reference is "key-number", and an identifier containing a hyphen
// followed by digits would be parsed as one.
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
