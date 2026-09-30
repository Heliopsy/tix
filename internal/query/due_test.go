// SPDX-License-Identifier: AGPL-3.0-or-later

package query_test

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
)

// parseDue is the present every case below is answered against, so a test of
// "overdue" is not a test of what time it happens to be.
var parseDueNow = time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)

// TestDueTermBoundsTheListing covers the term the command line, the board and
// the browser all reach "what is overdue" through. Each case asserts the bound
// the term produces rather than that it parsed, because a term accepted and
// then dropped is exactly the defect this exists to prevent.
func TestDueTermBoundsTheListing(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		expr   string
		before *time.Time
		after  *time.Time
	}{
		{"overdue is a bound at now", "due:overdue", &parseDueNow, nil},
		{"late says the same thing", "due:late", &parseDueNow, nil},
		{"today runs to the end of the day", "due:today",
			ptr(time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)), nil},
		{"a week runs seven days out", "due:week",
			ptr(parseDueNow.AddDate(0, 0, 7)), nil},
		{"a month runs a month out", "due:month",
			ptr(parseDueNow.AddDate(0, 1, 0)), nil},
		{"the bounded form takes a date", "due:<2026-04-01",
			ptr(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)), nil},
		{"and the other way", "due:>2026-04-01", nil,
			ptr(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC))},
		{"both ends at once", "due:>2026-03-01 due:<2026-04-01",
			ptr(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)),
			ptr(time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC))},
		{"the older spelling still works", "due-before:2026-04-01",
			ptr(time.Date(2026, 4, 1, 0, 0, 0, 0, time.UTC)), nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f, err := query.ParseAt(tc.expr, parseDueNow)
			if err != nil {
				t.Fatalf("ParseAt(%q): %v", tc.expr, err)
			}
			if !sameTime(f.DueBefore, tc.before) {
				t.Errorf("%q: DueBefore = %v, want %v", tc.expr, show(f.DueBefore), show(tc.before))
			}
			if !sameTime(f.DueAfter, tc.after) {
				t.Errorf("%q: DueAfter = %v, want %v", tc.expr, show(f.DueAfter), show(tc.after))
			}
		})
	}
}

// TestDueTermRefusesWhatItCannotAnswer keeps the term from silently selecting
// something other than what was asked, which is how the web's second parser
// used to answer "title~api" with an empty board and no error.
func TestDueTermRefusesWhatItCannotAnswer(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{
		"due:tomorrow", "due:", "due:<not-a-date", "due:>whenever", "-due:overdue",
	} {
		if _, err := query.ParseAt(expr, parseDueNow); err == nil {
			t.Errorf("ParseAt(%q) was accepted", expr)
		} else if core.KindOf(err) != core.KindInvalid {
			t.Errorf("ParseAt(%q) failed as %v, want an invalid-input error", expr, core.KindOf(err))
		}
	}
}

// TestDueTermIsListedAsAKey is what puts it in `?`, in the web filter bar's
// hint and in the CLI's help, all of which are rendered from Keys.
func TestDueTermIsListedAsAKey(t *testing.T) {
	t.Parallel()
	var found bool
	for _, k := range query.Keys {
		if k == "due" {
			found = true
		}
	}
	if !found {
		t.Fatalf("query.Keys does not list due, so no surface offers it: %v", query.Keys)
	}
	if len(query.DueWindows) == 0 {
		t.Fatal("DueWindows is empty, so the web control offers nothing")
	}
	for _, w := range query.DueWindows {
		if _, err := query.ParseAt("due:"+w, parseDueNow); err != nil {
			t.Errorf("DueWindows offers %q, which the parser refuses: %v", w, err)
		}
	}
}

// TestParseReadsTheWallClock is the one thing ParseAt cannot show: Parse has
// to answer a relative term against now rather than against the zero time.
func TestParseReadsTheWallClock(t *testing.T) {
	t.Parallel()
	before := time.Now().UTC()
	f, err := query.Parse("due:overdue")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if f.DueBefore == nil {
		t.Fatal("due:overdue set no bound")
	}
	if f.DueBefore.Before(before) || f.DueBefore.After(time.Now().UTC()) {
		t.Fatalf("due:overdue bound at %v, which is not the present", f.DueBefore)
	}
}

// TestTheDueBoundsAreInclusiveInMemoryAsWellAsInSQL closes the gap between the
// two answers to one filter. The stores keep a task whose deadline is exactly
// the bound ("due_at <= ?"), and the board filters the page it was handed with
// Matches, so a task the store returned was being dropped on screen.
func TestTheDueBoundsAreInclusiveInMemoryAsWellAsInSQL(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	task := core.Task{Ref: "infra-1", DueAt: &at}
	if !query.Matches(core.TaskFilter{DueBefore: &at}, task, "infra", parseDueNow) {
		t.Error("a deadline exactly at due_before was dropped; the stores keep it")
	}
	if !query.Matches(core.TaskFilter{DueAfter: &at}, task, "infra", parseDueNow) {
		t.Error("a deadline exactly at due_after was dropped; the stores keep it")
	}
	outside := at.Add(time.Second)
	if query.Matches(core.TaskFilter{DueBefore: &at}, core.Task{DueAt: &outside}, "infra", parseDueNow) {
		t.Error("a deadline past due_before was kept")
	}
	if query.Matches(core.TaskFilter{DueBefore: &at}, core.Task{}, "infra", parseDueNow) {
		t.Error("a task with no deadline answered a deadline filter")
	}
}

func ptr(t time.Time) *time.Time { return &t }

func sameTime(a, b *time.Time) bool {
	switch {
	case a == nil && b == nil:
		return true
	case a == nil || b == nil:
		return false
	default:
		return a.Equal(*b)
	}
}

func show(t *time.Time) string {
	if t == nil {
		return "none"
	}
	return t.Format(time.RFC3339)
}
