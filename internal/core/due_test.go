// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// TestDueStateOfIsTheOneAnswerEverySurfaceReads pins the boundaries, because
// each of them separates two surfaces agreeing from two surfaces disagreeing:
// the stores filter deadlines with "due_at <= ?", so a deadline exactly at now
// is returned by a listing and has to read as overdue everywhere it is drawn.
func TestDueStateOfIsTheOneAnswerEverySurfaceReads(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	at := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	tests := []struct {
		name string
		due  *time.Time
		want core.DueState
	}{
		{"no deadline", nil, core.DueNone},
		{"a minute past", at(-time.Minute), core.DueOverdue},
		{"exactly now", at(0), core.DueOverdue},
		{"a minute from now", at(time.Minute), core.DueSoon},
		{"the last instant of the window", at(core.DueSoonWindow), core.DueSoon},
		{"one second past the window", at(core.DueSoonWindow + time.Second), core.DueLater},
		{"next quarter", at(90 * 24 * time.Hour), core.DueLater},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := core.DueStateOf(tc.due, now); got != tc.want {
				t.Fatalf("DueStateOf = %v, want %v", got, tc.want)
			}
			task := core.Task{DueAt: tc.due}
			if got := task.DueState(now); got != tc.want {
				t.Fatalf("Task.DueState = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestOnlyAPressingDeadlineIsNotable is the rule every surface draws by. A
// due-date marker was removed from the board once for appearing on every card,
// so "has a deadline" must not be what any of them mark.
func TestOnlyAPressingDeadlineIsNotable(t *testing.T) {
	t.Parallel()
	notable := map[core.DueState]bool{
		core.DueNone: false, core.DueLater: false, core.DueSoon: true, core.DueOverdue: true,
	}
	for state, want := range notable {
		if got := state.Notable(); got != want {
			t.Errorf("%v.Notable() = %v, want %v", state, got, want)
		}
	}
	for state, want := range map[core.DueState]string{
		core.DueNone:    "no due date",
		core.DueLater:   "due later",
		core.DueSoon:    "due soon",
		core.DueOverdue: "overdue",
	} {
		if got := state.String(); got != want {
			t.Errorf("%d.String() = %q, want %q", state, got, want)
		}
	}
}
