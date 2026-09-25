// SPDX-License-Identifier: AGPL-3.0-or-later

package output_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

func syncResult() *core.SyncResult {
	return &core.SyncResult{
		ImportResult: core.ImportResult{
			Created: map[string]int{"task": 3},
			Updated: map[string]int{"task": 1},
			Skipped: map[string]int{"task": 2},
		},
		System: "jira",
		Source: "platform",
		Cursor: "2026-09-25T09:00:00Z",
	}
}

// TestASyncResultIsPresentedRatherThanDumped pins the fix for a run that ended
// by printing its own Go struct.
//
// SyncResult had no case in the table renderer, so it fell through to the
// reflected fallback and an operator finishing an import read
// &{map[task:3] map[task:1] ... jira platform 2026-...}. The counts, the
// source and the watermark are the whole point of the command's output.
func TestASyncResultIsPresentedRatherThanDumped(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	if err := output.New(output.FormatTable).Format(&buf, syncResult()); err != nil {
		t.Fatalf("Format: %v", err)
	}
	got := buf.String()

	if strings.Contains(got, "map[") || strings.Contains(got, "&{") {
		t.Fatalf("the result was dumped as a Go value:\n%s", got)
	}
	for _, want := range []string{"jira", "platform", "2026-09-25T09:00:00Z"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing %q:\n%s", want, got)
		}
	}
	// One row per entity, carrying every count.
	for _, want := range []string{"3", "1", "2"} {
		if !strings.Contains(got, want) {
			t.Errorf("output is missing the count %q:\n%s", want, got)
		}
	}
}

func TestADryRunSaysNothingWasWritten(t *testing.T) {
	t.Parallel()
	r := syncResult()
	r.DryRun = true
	var buf bytes.Buffer
	if err := output.New(output.FormatTable).Format(&buf, r); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "nothing was written") {
		t.Errorf("a dry run did not say so:\n%s", got)
	}
}

// A run that matched nothing still has to say so rather than print an empty
// table with no explanation.
func TestASyncResultThatTouchedNothingSaysSo(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	empty := &core.SyncResult{
		ImportResult: core.ImportResult{
			Created: map[string]int{}, Updated: map[string]int{}, Skipped: map[string]int{},
		},
		System: "generic", Source: "file",
	}
	if err := output.New(output.FormatTable).Format(&buf, empty); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got := buf.String(); !strings.Contains(got, "Nothing to import") {
		t.Errorf("an empty run gave no account of itself:\n%s", got)
	}
}

// json and yaml went through the generic encoders already; this pins that the
// new table case did not divert them.
func TestSyncResultStillEncodesAsDataInJSONAndYAML(t *testing.T) {
	t.Parallel()
	for _, f := range []string{output.FormatJSON, output.FormatYAML} {
		var buf bytes.Buffer
		if err := output.New(f).Format(&buf, syncResult()); err != nil {
			t.Fatalf("Format(%s): %v", f, err)
		}
		got := buf.String()
		if !strings.Contains(got, "jira") || !strings.Contains(got, "cursor") {
			t.Errorf("%s output lost fields:\n%s", f, got)
		}
	}
}

// TestTheTableLabelsActorsAndTheMachineFormatsDoNot pins the split.
//
// A task list printed its assignee as a 26 character identifier, the widest
// column in the table, telling a reader nothing. The label is resolved per
// listing and handed to the renderer rather than carried on the task, because
// a handle is not a property of a task. The machine formats keep the
// identifier: that is what a consumer looks the actor up by, and a handle can
// be renamed.
func TestTheTableLabelsActorsAndTheMachineFormatsDoNot(t *testing.T) {
	t.Parallel()
	const id = "01KW4DM3ARD6SK4SFAF83K4431"
	tasks := []core.Task{{Ref: "infra-1", Title: "Rotate the certificates",
		Status: "todo", AssigneeActorID: id}}
	names := map[string]string{id: "tom"}

	var table bytes.Buffer
	if err := output.NewWithNames(output.FormatTable, output.ModeNever, output.TimeStyle{}, names).
		Format(&table, tasks); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if got := table.String(); !strings.Contains(got, "tom") || strings.Contains(got, id) {
		t.Errorf("the table did not label the actor:\n%s", got)
	}

	var asJSON bytes.Buffer
	if err := output.New(output.FormatJSON).Format(&asJSON, tasks); err != nil {
		t.Fatalf("Format json: %v", err)
	}
	if got := asJSON.String(); !strings.Contains(got, id) || strings.Contains(got, "tom") {
		t.Errorf("json should carry the identifier and no handle:\n%s", got)
	}
}

// An actor nothing resolves still has to print something.
func TestAnUnresolvedActorFallsBackToItsIdentifier(t *testing.T) {
	t.Parallel()
	const id = "01KW4DM3ARD6SK4SFAF83K4431"
	tasks := []core.Task{{Ref: "infra-1", Title: "x", Status: "todo", AssigneeActorID: id}}
	var buf bytes.Buffer
	if err := output.NewWithNames(output.FormatTable, output.ModeNever, output.TimeStyle{}, nil).
		Format(&buf, tasks); err != nil {
		t.Fatalf("Format: %v", err)
	}
	if !strings.Contains(buf.String(), id) {
		t.Errorf("an unresolved actor printed nothing:\n%s", buf.String())
	}
}
