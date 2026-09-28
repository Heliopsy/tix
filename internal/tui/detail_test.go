// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"
	"testing"
)

// bodySection returns the lines of the detail pane's body section and nothing
// else. Reading the section rather than the frame is what keeps "the last word
// reached the screen" from passing on the word appearing in the title bar.
func bodySection(t *testing.T, frame string) []string {
	t.Helper()
	var out []string
	in := false
	for _, l := range strings.Split(stripANSI(frame), "\n") {
		switch {
		case strings.HasPrefix(l, "body:"):
			in = true
		case in && strings.TrimSpace(l) == "":
			in = false
		case in:
			out = append(out, strings.TrimSpace(l))
		}
	}
	return out
}

// TestTheDetailBodyWrapsRatherThanClipping pins the reported defect: a long
// body was cut at the right edge, so a sentence ended mid-word with nothing
// saying it had been cut. Every word of the body now reaches the screen.
func TestTheDetailBodyWrapsRatherThanClipping(t *testing.T) {
	body := "Everything can talk to everything, which means a compromised sidecar reaches " +
		"the database directly. Start with staging, write the allow rules from observed " +
		"traffic, then repeat in production."
	for _, width := range []int{60, 100, 150} {
		t.Run(fmt.Sprintf("width %d", width), func(t *testing.T) {
			m := boardModel(t)
			m.width, m.height = width, 40
			task, _ := TaskAt(m.columns, m.sel)
			task.Body = body
			m, _ = m.reduce(detailMsg{task: task})
			got := bodySection(t, m.Frame())
			if len(got) == 0 {
				t.Fatalf("width %d: the detail pane drew no body:\n%s", width, m.Frame())
			}
			if joined := strings.Join(got, " "); joined != body {
				t.Fatalf("width %d: the body reached the screen as %q, want %q", width, joined, body)
			}
			for _, l := range got {
				if strings.HasSuffix(l, "…") {
					t.Fatalf("width %d: a body line was truncated: %q", width, l)
				}
			}
		})
	}
}
