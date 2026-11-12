// SPDX-License-Identifier: AGPL-3.0-or-later

package release

import (
	"fmt"
	"testing"

	"github.com/heliopsy/tix/internal/changelog"
)

// Notes written by hand are not touched: only a leading heading naming the
// release is redundant.
func TestOnlyTheLeadingReleaseHeadingIsDropped(t *testing.T) {
	t.Parallel()
	cases := []struct{ notes, latest, want string }{
		{"## 0.30.0 (2026-11-01)\n\n### Features", "0.30.0", "### Features"},
		{"## [0.30.0] notes\n\nbody", "0.30.0", "body"},
		{"## Highlights\n\nbody", "0.30.0", "## Highlights\n\nbody"},
		{"plain text about 0.30.0", "0.30.0", "plain text about 0.30.0"},
	}
	for _, tc := range cases {
		if got := notesBody(tc.notes, tc.latest); got != tc.want {
			t.Errorf("notesBody(%q) = %q, want %q", tc.notes, got, tc.want)
		}
	}
}

// Only ten earlier entries are listed, however long the changelog is.
func TestEarlierEntriesAreBounded(t *testing.T) {
	t.Parallel()
	var log []changelog.Entry
	for i := 40; i > 0; i-- {
		log = append(log, changelog.Entry{Version: fmt.Sprintf("0.%d.0", i), Body: "* x"})
	}
	if got := NewChanges("0.40.0", log); got.Running == nil || len(got.Earlier) != earlierChanges {
		t.Errorf("a tagged build lists %d earlier entries, want %d", len(got.Earlier), earlierChanges)
	}
}
