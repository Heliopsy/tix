// SPDX-License-Identifier: AGPL-3.0-or-later

package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

func renderDeletion(t *testing.T, d *core.ProjectDeletion) string {
	t.Helper()
	var buf bytes.Buffer
	if err := output.New(output.FormatTable).Format(&buf, d); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()
	if strings.Contains(got, "map[") || strings.Contains(got, "{") {
		t.Fatalf("the deletion was dumped as a Go value:\n%s", got)
	}
	return got
}

// A move's mapping reads as one row per source status, "from -> to", with the
// count beside it, rather than as the slice of structs it is.
func TestAProjectDeletionPlanReadsAsAMapping(t *testing.T) {
	t.Parallel()
	got := renderDeletion(t, &core.ProjectDeletion{
		Project: "old", Tasks: 3, Deleted: 1, MoveTo: "lab", DryRun: true,
		Statuses: []core.StatusMove{{From: "todo", To: "backlog", Tasks: 2}, {From: "doing", To: "active", Tasks: 2}},
	})
	for _, want := range []string{
		"Dry run, planned only: deleting OLD would move its 4 tasks to LAB; nothing was written.",
		"todo -> backlog", "doing -> active",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "todo -> backlog") && !strings.Contains(line, "2") {
			t.Errorf("the row for todo does not carry its count: %q", line)
		}
	}
}

func TestAProjectDeletionStatesWhatHappened(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   core.ProjectDeletion
		want string
	}{
		{"deleted with its tasks", core.ProjectDeletion{Project: "old", Tasks: 3, DeleteTasks: true},
			"Deleted project OLD with its 3 tasks, permanently."},
		{"empty", core.ProjectDeletion{Project: "old"}, "Deleted project OLD, which held no tasks."},
		{"one task moved", core.ProjectDeletion{Project: "old", Tasks: 1, MoveTo: "ops",
			Statuses: []core.StatusMove{{From: "todo", To: "todo", Tasks: 1}}},
			"Deleted project OLD and moved its 1 task to OPS."},
		{"planned deletion", core.ProjectDeletion{Project: "old", Tasks: 2, DeleteTasks: true, DryRun: true},
			"Dry run, planned only: deleting OLD would delete its 2 tasks permanently; nothing was written."},
		{"planned without a choice", core.ProjectDeletion{Project: "old", Tasks: 2, DryRun: true},
			"Dry run, planned only: OLD holds 2 tasks; pass --move-tasks-to KEY or --delete-tasks; nothing was written."},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			d := tc.in
			if got := renderDeletion(t, &d); !strings.Contains(got, tc.want) {
				t.Errorf("output %q does not say %q", got, tc.want)
			}
		})
	}
}
