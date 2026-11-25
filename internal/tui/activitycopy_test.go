// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// ^ on the activity view copies the selected event as the JSON the API
// returns for it, not the newest one or the whole tail.
func TestTheActivityCopyKeyCopiesTheSelectedEventAsJSON(t *testing.T) {
	m := boardModel(t)
	for i := int64(1); i <= 3; i++ {
		m, _ = m.reduce(eventMsg{event: activityEvent(i, "alice")})
	}
	m, _ = m.reduce(pressKey("v"))
	m, _ = m.reduce(pressKey("k"))
	m, cmd := m.reduce(pressKey("^"))
	got := clipboardText(t, cmd)
	var e core.Event
	if err := json.Unmarshal([]byte(got), &e); err != nil {
		t.Fatalf("copied text is not an event: %v\n%s", err, got)
	}
	if e.Seq != 2 || !strings.Contains(got, "\n  \"seq\": 2") {
		t.Fatalf("copied event %d, want the selected one, 2, pretty-printed:\n%s", e.Seq, got)
	}
	if !strings.Contains(stripANSI(m.Frame()), `"seq": 2`) {
		t.Fatalf("the frame does not show the copied JSON")
	}
}

func TestTheActivityCopyKeyIsInTheHelp(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("v"))
	m, _ = m.reduce(pressKey("?"))
	if !strings.Contains(stripANSI(m.Frame()), "copy as JSON") {
		t.Fatalf("the activity help does not offer the copy key")
	}
}

func TestTheActivityCopyKeyOnAnEmptyTailSaysSo(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("v"))
	m, cmd := m.reduce(pressKey("^"))
	if cmd != nil || m.err == "" {
		t.Fatalf("copying from an empty tail sent %v and said %q", cmd, m.err)
	}
}
