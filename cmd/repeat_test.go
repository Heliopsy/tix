// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func showTask(t *testing.T, c *cli, ref string) core.Task {
	t.Helper()
	var task core.Task
	out := c.mustRun("task", "show", ref, "-o", "json").out
	if err := json.Unmarshal([]byte(out), &task); err != nil {
		t.Fatalf("task show is not json: %v\n%s", err, out)
	}
	return task
}

func TestTaskRepeatFlags(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "--due", "2026-11-01", "--repeat", "monthly", "pay rent")
	if got := showTask(t, c, "default-1"); got.Recurrence != "every month" {
		t.Fatalf("added rule = %q, want every month", got.Recurrence)
	}

	c.mustRun("task", "edit", "default-1", "--repeat", "every 2 weeks on thu,mon")
	if got := showTask(t, c, "default-1"); got.Recurrence != "every 2 weeks on mon,thu" {
		t.Fatalf("edited rule = %q", got.Recurrence)
	}

	both := c.run("task", "edit", "default-1", "--repeat", "daily", "--no-repeat")
	if both.code != core.ExitUsage {
		t.Fatalf("--repeat with --no-repeat exited %d, want %d: %s", both.code, core.ExitUsage, both.err)
	}
	if got := showTask(t, c, "default-1"); got.Recurrence != "every 2 weeks on mon,thu" {
		t.Fatalf("a refused edit changed the rule to %q", got.Recurrence)
	}

	bad := c.run("task", "edit", "default-1", "--repeat", "every fortnight")
	if bad.code != core.ExitUsage || !strings.Contains(bad.err, "fortnight") {
		t.Fatalf("an unparsable rule exited %d: %s", bad.code, bad.err)
	}

	c.mustRun("task", "edit", "default-1", "--no-repeat")
	if got := showTask(t, c, "default-1"); got.Recurrence != "" {
		t.Fatalf("--no-repeat left %q", got.Recurrence)
	}
}

func TestFinishingFromTheCommandLineSpawnsAndShowNamesIt(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "--due", "2026-11-02", "--repeat", "weekly", "water plants")
	c.mustRun("task", "mv", "default-1", "doing")
	c.mustRun("task", "mv", "default-1", "done")

	next := showTask(t, c, "default-2")
	if next.Title != "water plants" || next.Recurrence != "every week" || next.Status != "todo" {
		t.Fatalf("successor = %+v", next)
	}
	if next.DueAt == nil || next.DueAt.Format("2006-01-02") != "2026-11-09" {
		t.Fatalf("successor due = %v, want 2026-11-09", next.DueAt)
	}
	if done := showTask(t, c, "default-1"); done.RecurrenceNextID != next.ID {
		t.Fatalf("recurrence_next_id = %q, want %q", done.RecurrenceNextID, next.ID)
	}

	shown := c.mustRun("task", "show", "default-1")
	if !strings.Contains(shown.out, "every week") {
		t.Fatalf("task show does not state the rule:\n%s", shown.out)
	}
	if !strings.Contains(shown.err, "next occurrence: DEFAULT-2") {
		t.Fatalf("task show does not name the successor: %q", shown.err)
	}

	recurring := refsOf(t, c.mustRun("task", "ls", "--recurring", "-o", "ndjson").out)
	if len(recurring) != 2 {
		t.Fatalf("--recurring listed %v, want both occurrences", recurring)
	}
	c.mustRun("task", "add", "one-off")
	if got := refsOf(t, c.mustRun("task", "ls", "--filter", "-is:recurring", "-o", "ndjson").out); len(got) != 1 || got[0] != "one-off" {
		t.Fatalf("-is:recurring listed %v, want only the one-off", got)
	}
}
