// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func exportTasks(t *testing.T, l *Local, ctx context.Context, in core.TaskExportInput) string {
	t.Helper()
	var buf bytes.Buffer
	if err := l.ExportTasks(ctx, in, &buf); err != nil {
		t.Fatalf("export: %v", err)
	}
	return buf.String()
}

func csvRows(t *testing.T, raw string) [][]string {
	t.Helper()
	rows, err := csv.NewReader(strings.NewReader(raw)).ReadAll()
	if err != nil {
		t.Fatalf("reading csv: %v\n%s", err, raw)
	}
	return rows
}

func TestExportTasksWritesTheDefaultColumnsAndCustomFields(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	rows := csvRows(t, exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra"}))
	want := append(append([]string{}, core.DefaultTaskColumns...), "custom:points")
	if strings.Join(rows[0], ",") != strings.Join(want, ",") {
		t.Fatalf("header = %v, want %v", rows[0], want)
	}
	if len(rows) != 3 {
		t.Fatalf("got %d rows, want a header and two tasks", len(rows))
	}
	cell := func(row int, column string) string {
		for i, c := range rows[0] {
			if c == column {
				return rows[row][i]
			}
		}
		t.Fatalf("no column %q", column)
		return ""
	}
	first, second := 1, 2
	if cell(first, "title") != "first" || cell(first, "priority") != "high" || cell(first, "tags") != "ops" ||
		cell(first, "custom:points") != "3" {
		t.Errorf("first row = %v", rows[first])
	}
	if cell(second, "parent") != cell(first, "ref") || cell(second, "depends_on") != cell(first, "ref") {
		t.Errorf("second row's links = parent %q depends_on %q, want %q",
			cell(second, "parent"), cell(second, "depends_on"), cell(first, "ref"))
	}
}

func TestExportTasksTakesColumnsInTheOrderAsked(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	rows := csvRows(t, exportTasks(t, l, ctx, core.TaskExportInput{
		ProjectRef: "infra", Columns: []string{"title", "ref", "comments"}, Delimiter: ",",
	}))
	if strings.Join(rows[0], ",") != "title,ref,comments" {
		t.Fatalf("header = %v", rows[0])
	}
	if !strings.Contains(rows[1][2], "a comment") {
		t.Errorf("comments cell = %q, want the comment", rows[1][2])
	}
}

func TestExportTasksFiltersByQueryAndSubtasks(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")

	for _, tc := range []struct {
		name string
		in   core.TaskExportInput
		want []string
	}{
		{"query", core.TaskExportInput{Query: "tag:ops"}, []string{"first"}},
		{"no subtasks", core.TaskExportInput{ExcludeSubtasks: true}, []string{"first"}},
		{"everything", core.TaskExportInput{}, []string{"first", "second"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tc.in.ProjectRef = "infra"
			tc.in.Columns = []string{"title"}
			rows := csvRows(t, exportTasks(t, l, ctx, tc.in))
			var got []string
			for _, r := range rows[1:] {
				got = append(got, r[0])
			}
			if strings.Join(got, ",") != strings.Join(tc.want, ",") {
				t.Errorf("titles = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestExportTasksLeavesOutFinishedAndTrashedOnRequest(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	if _, err := l.TransitionTask(ctx, s.first, core.TransitionInput{To: "cancelled"}); err != nil {
		t.Fatalf("finishing: %v", err)
	}
	if err := l.DeleteTask(ctx, s.second, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("trashing: %v", err)
	}
	titles := func(in core.TaskExportInput) string {
		in.ProjectRef, in.Columns = "infra", []string{"title"}
		var got []string
		for _, r := range csvRows(t, exportTasks(t, l, ctx, in))[1:] {
			got = append(got, r[0])
		}
		return strings.Join(got, ",")
	}
	if got := titles(core.TaskExportInput{}); got != "first" {
		t.Errorf("default export = %q, want the finished task and not the trashed one", got)
	}
	if got := titles(core.TaskExportInput{ExcludeFinished: true}); got != "" {
		t.Errorf("export without finished = %q, want nothing", got)
	}
	if got := titles(core.TaskExportInput{IncludeTrashed: true}); got != "first,second" {
		t.Errorf("export with the trash = %q, want both", got)
	}
}

func TestExportTasksGuardsSpreadsheetFormulas(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	for _, title := range []string{"=HYPERLINK(\"x\")", "+1", "-2", "@cmd"} {
		if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: title}); err != nil {
			t.Fatalf("task %q: %v", title, err)
		}
	}
	rows := csvRows(t, exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra", Columns: []string{"title"}}))
	for _, r := range rows[3:] {
		if !strings.HasPrefix(r[0], "'") {
			t.Errorf("csv cell %q is not guarded", r[0])
		}
	}
	raw := exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra", Format: core.TaskFormatNDJSON, Columns: []string{"title"}})
	if !strings.Contains(raw, `"title":"@cmd"`) {
		t.Errorf("ndjson changed a title it should carry as is:\n%s", raw)
	}
}

func TestExportTasksJSONIsOneArrayInColumnOrder(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	raw := exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra", Format: core.TaskFormatJSON,
		Columns: []string{"title", "tags", "custom:points", "due"}})
	var got []map[string]any
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatalf("json: %v\n%s", err, raw)
	}
	if len(got) != 2 || got[0]["title"] != "first" || got[0]["custom:points"] != float64(3) || got[0]["due"] != nil {
		t.Errorf("json = %v", got)
	}
	if !strings.HasPrefix(raw, `[`+"\n"+`{"title":"first","tags":["ops"]`) {
		t.Errorf("keys are not in column order:\n%s", raw)
	}
}

func TestExportTasksMarkdownKeepsEachTaskOnItsRow(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "a | b", Body: "line\nnext"}); err != nil {
		t.Fatal(err)
	}
	raw := exportTasks(t, l, ctx, core.TaskExportInput{ProjectRef: "infra", Format: core.TaskFormatMarkdown,
		Columns: []string{"title", "body"}})
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	if len(lines) != 5 || lines[4] != `| a \| b | line<br>next |` {
		t.Errorf("markdown =\n%s", raw)
	}
}

func TestExportTasksRefusals(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	for _, tc := range []struct {
		name string
		in   core.TaskExportInput
		kind core.Kind
	}{
		{"no project", core.TaskExportInput{}, core.KindInvalid},
		{"unknown project", core.TaskExportInput{ProjectRef: "nope"}, core.KindNotFound},
		{"unknown column", core.TaskExportInput{ProjectRef: "infra", Columns: []string{"nope"}}, core.KindInvalid},
		{"bad query", core.TaskExportInput{ProjectRef: "infra", Query: "priority:urgentest"}, core.KindInvalid},
		{"other project in query", core.TaskExportInput{ProjectRef: "infra", Query: "project:ops"}, core.KindInvalid},
		{"relative dates", core.TaskExportInput{ProjectRef: "infra", Dates: core.TaskDatesDisplay, TimeFormat: "relative"}, core.KindInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := l.ExportTasks(ctx, tc.in, &buf)
			if !core.IsKind(err, tc.kind) {
				t.Fatalf("err = %v, want %s", err, tc.kind)
			}
			if buf.Len() != 0 {
				t.Errorf("a refused export wrote %q", buf.String())
			}
		})
	}
}

func TestExportTasksNeedsTheExportScopeAndStaysInAPinnedProject(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seed(t, l, ctx, "infra")
	seed2, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops", WorkflowKey: "flow"})
	if err != nil {
		t.Fatal(err)
	}
	infra, err := l.GetProject(ctx, "infra")
	if err != nil {
		t.Fatal(err)
	}

	reader := *actor
	reader.Role, reader.Scopes = "", []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}
	err = l.ExportTasks(core.WithActor(context.Background(), &reader), core.TaskExportInput{ProjectRef: "infra"}, &bytes.Buffer{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a reader without export exported: %v", err)
	}

	pinned := *actor
	pinned.Role, pinned.Scopes, pinned.ProjectID = "", []core.Scope{core.ScopeExport}, infra.ID
	pctx := core.WithActor(context.Background(), &pinned)
	if err := l.ExportTasks(pctx, core.TaskExportInput{ProjectRef: "infra"}, &bytes.Buffer{}); err != nil {
		t.Errorf("a token pinned to infra could not export infra: %v", err)
	}
	err = l.ExportTasks(pctx, core.TaskExportInput{ProjectRef: seed2.Key}, &bytes.Buffer{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a token pinned to infra exported ops: %v", err)
	}
}
