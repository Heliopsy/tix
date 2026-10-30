// SPDX-License-Identifier: AGPL-3.0-or-later

package output

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestStarredTaskIsMarkedInItsRefCell(t *testing.T) {
	starred := sampleTask()
	starred.Starred = true
	plain := sampleTask()
	plain.Ref = "ENG-8"

	rendered := render(t, FormatTable, []core.Task{starred, plain})
	cases := []struct {
		row  int
		want string
	}{
		{0, StarMarker + "ENG-7"},
		{1, "ENG-8"},
	}
	for _, tc := range cases {
		if got := cellAt(t, rendered, "REF", tc.row); got != strings.TrimSpace(tc.want) {
			t.Errorf("REF of row %d = %q, want %q", tc.row, got, tc.want)
		}
	}

	if js := render(t, FormatJSON, starred); !strings.Contains(js, `"starred": true`) {
		t.Errorf("json of a starred task lacks the flag:\n%s", js)
	}
	if js := render(t, FormatJSON, plain); strings.Contains(js, `"starred"`) {
		t.Errorf("json of an unstarred task carries the flag:\n%s", js)
	}
}
