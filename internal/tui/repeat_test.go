// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"
)

func TestARecurringCardCarriesTheRepeatMarker(t *testing.T) {
	now := time.Date(2026, 11, 2, 9, 0, 0, 0, time.UTC)
	plain := task("a", "todo", 1, 3)
	repeating := task("b", "todo", 2, 3)
	repeating.Recurrence = "every week"

	if flags := CardFlags(repeating, now, ""); !strings.Contains(flags, RepeatMarker) {
		t.Fatalf("a recurring card draws %q, want the repeat marker", flags)
	}
	if flags := CardFlags(plain, now, ""); strings.Contains(flags, RepeatMarker) {
		t.Fatalf("a one-off card draws the repeat marker: %q", flags)
	}
	if badges := strings.Join(TaskBadges(repeating, now), " "); !strings.Contains(badges, "repeats") {
		t.Fatalf("badges %q do not say the task repeats", badges)
	}
}

func TestTheDetailViewStatesTheRule(t *testing.T) {
	m := boardModel(t)
	m.width, m.height = 100, 40
	open, _ := TaskAt(m.columns, m.sel)
	open.Recurrence = "every 2 weeks on mon,thu"
	m, _ = m.reduce(detailMsg{task: open})
	if frame := stripANSI(m.Frame()); !strings.Contains(frame, "repeats: every 2 weeks on mon,thu") {
		t.Fatalf("the detail view does not state the rule:\n%s", frame)
	}

	open.Recurrence = ""
	m, _ = m.reduce(detailMsg{task: open})
	if frame := stripANSI(m.Frame()); strings.Contains(frame, "repeats:") {
		t.Fatalf("the detail view spends a line on a task that does not repeat:\n%s", frame)
	}
}
