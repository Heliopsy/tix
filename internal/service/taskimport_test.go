// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// importFixture is a tenant with a source project, a target project with the
// same custom field, and the seeded tasks of the source.
type importFixture struct {
	l   *Local
	ctx context.Context
	s   seeded
}

func newImportFixture(t *testing.T) importFixture {
	t.Helper()
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	s := seed(t, l, ctx, "infra")
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops", WorkflowKey: "flow"}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.PutFieldDef(ctx, "ops", core.FieldDefInput{Key: "points", Label: "Points", Type: core.FieldInt, Position: 1}); err != nil {
		t.Fatal(err)
	}
	return importFixture{l: l, ctx: ctx, s: s}
}

func (f importFixture) importTasks(t *testing.T, content string, in core.TaskImportInput) *core.TaskImportResult {
	t.Helper()
	if in.ProjectRef == "" {
		in.ProjectRef = "ops"
	}
	res, err := f.l.ImportTasks(f.ctx, strings.NewReader(content), in)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	return res
}

func (f importFixture) count(t *testing.T, project string) int {
	t.Helper()
	page, err := f.l.ListTasks(f.ctx, core.TaskFilter{ProjectKeys: []string{project}, IncludeSnoozed: true,
		Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatal(err)
	}
	return len(page.Tasks)
}

// comparable is an export of everything a round trip must carry, references
// aside, which differ by project.
var comparable = []string{"title", "body", "status", "priority", "tags", "due", "parent", "depends_on",
	"recurrence", "custom:points"}

func TestImportRoundTripsAnExportInEveryFormat(t *testing.T) {
	for _, format := range []core.TaskFormat{core.TaskFormatCSV, core.TaskFormatJSON, core.TaskFormatNDJSON} {
		t.Run(string(format), func(t *testing.T) {
			f := newImportFixture(t)
			due := time.Date(2026, 12, 1, 0, 0, 0, 0, time.UTC)
			if _, err := f.l.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "=formula, with comma",
				Body: "two\nlines | pipe", Tags: []string{"a", "b"}, DueAt: &due, Recurrence: "every week",
				Status: "doing"}); err != nil {
				t.Fatal(err)
			}
			exported := exportTasks(t, f.l, f.ctx, core.TaskExportInput{ProjectRef: "infra", Format: format})
			res := f.importTasks(t, exported, core.TaskImportInput{})
			if !res.Committed || res.Created != 3 || res.Failed != 0 {
				t.Fatalf("import = %+v", res)
			}
			cols := core.TaskExportInput{Columns: comparable, Format: core.TaskFormatNDJSON}
			cols.ProjectRef = "infra"
			want := exportTasks(t, f.l, f.ctx, cols)
			cols.ProjectRef = "ops"
			got := exportTasks(t, f.l, f.ctx, cols)
			want = strings.ReplaceAll(want, `"infra-`, `"ops-`)
			if got != want {
				t.Errorf("round trip lost something:\nwant %s\ngot  %s", want, got)
			}
		})
	}
}

func TestImportReadsOtherToolsWithTheSuggestedMapping(t *testing.T) {
	for _, tc := range []struct {
		file, source string
		rows         int
		parent       bool
	}{
		{"jira.csv", "jira", 4, true},
		{"github.csv", "github", 3, false},
		{"github.json", "github", 2, false},
		{"trello.json", "trello", 3, false},
		{"linear.csv", "linear", 3, true},
	} {
		t.Run(tc.file, func(t *testing.T) {
			f := newImportFixture(t)
			raw, err := os.ReadFile(filepath.Join("..", "taskio", "testdata", tc.file))
			if err != nil {
				t.Fatal(err)
			}
			res := f.importTasks(t, string(raw), core.TaskImportInput{Filename: tc.file})
			if res.Source != tc.source || res.Created != tc.rows || res.Failed != 0 || !res.Committed {
				t.Fatalf("import = source %q created %d failed %d errors %v", res.Source, res.Created, res.Failed, res.Errors)
			}
			linked := false
			for _, row := range res.Preview {
				if row.Parent != "" {
					linked = true
				}
				if row.Status == "" || row.Title == "" {
					t.Errorf("row %d lacks a status or a title: %+v", row.Line, row)
				}
			}
			if tc.source == "jira" {
				var statuses []string
				for _, row := range res.Preview {
					statuses = append(statuses, row.Status)
				}
				if got := strings.Join(statuses, ","); got != "done,doing,todo,todo" {
					t.Errorf("Jira statuses became %s, want done,doing,todo,todo", got)
				}
			}
			if linked != tc.parent {
				t.Errorf("a subtask's parent was resolved = %v, want %v: %+v", linked, tc.parent, res.Preview)
			}
		})
	}
}

func TestImportMapsValuesAndAppliesUnknownPolicies(t *testing.T) {
	f := newImportFixture(t)
	file := "Summary,State,Prio,Owner\nOne,In Progress,P1,ghost\nTwo,Weird,Highest,\n"
	mapping := `
columns: {Summary: title, State: status, Prio: priority, Owner: assignee}
values:
  status: {"in progress": blocked}
unknown: {status: error}
`
	res := f.importTasks(t, file, core.TaskImportInput{Mapping: mapping, DryRun: true})
	if res.Failed != 1 || len(res.Errors) != 1 || !strings.Contains(res.Errors[0].Message, `"Weird"`) {
		t.Fatalf("errors = %+v", res.Errors)
	}
	if res.Preview[0].Status != "blocked" || res.Preview[0].Priority != core.PriorityHigh {
		t.Errorf("first row = %+v, want the mapped status and P1 as high", res.Preview[0])
	}
	if res.Preview[0].Assignee != "" {
		t.Errorf("an unknown assignee under the default policy was kept: %+v", res.Preview[0])
	}
	res = f.importTasks(t, file, core.TaskImportInput{Mapping: strings.Replace(mapping, "status: error", "status: skip_row", 1)})
	if !res.Committed || res.Created != 1 || res.Skipped != 1 {
		t.Errorf("skip_row = %+v", res)
	}
}

func TestImportModesMatchByExternalID(t *testing.T) {
	f := newImportFixture(t)
	file := "Key,Title\nX-1,first\nX-2,second\n"
	mapping := "columns: {Key: external_id, Title: title}\nsource: sheet\n"
	if res := f.importTasks(t, file, core.TaskImportInput{Mapping: mapping}); res.Created != 2 {
		t.Fatalf("first import = %+v", res)
	}
	changed := "Key,Title\nX-1,first renamed\nX-2,second\n"
	for _, tc := range []struct {
		mode                   core.TaskImportMode
		created, updated, skip int
		tasks                  int
	}{
		{core.TaskImportUpsert, 0, 1, 1, 2},
		{core.TaskImportSkip, 0, 0, 2, 2},
		{core.TaskImportCreate, 2, 0, 0, 4},
	} {
		res := f.importTasks(t, changed, core.TaskImportInput{Mapping: mapping, Mode: tc.mode})
		if res.Created != tc.created || res.Updated != tc.updated || res.Skipped != tc.skip {
			t.Errorf("%s = created %d updated %d skipped %d", tc.mode, res.Created, res.Updated, res.Skipped)
		}
		if n := f.count(t, "ops"); n != tc.tasks {
			t.Errorf("%s left %d tasks, want %d", tc.mode, n, tc.tasks)
		}
	}
}

func TestADryRunWritesNothing(t *testing.T) {
	f := newImportFixture(t)
	before, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatal(err)
	}
	res := f.importTasks(t, "Title,Tags\nA,new-tag\nB,\n", core.TaskImportInput{DryRun: true})
	if res.Committed || !res.DryRun || res.Created != 2 || len(res.Preview) != 2 || res.Preview[0].Ref == "" {
		t.Fatalf("dry run = %+v", res)
	}
	if n := f.count(t, "ops"); n != 0 {
		t.Errorf("a dry run created %d tasks", n)
	}
	after, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != len(before) {
		t.Errorf("a dry run wrote %d audit entries", len(after)-len(before))
	}
}

func TestARowInErrorRollsBackTheWholeFile(t *testing.T) {
	f := newImportFixture(t)
	file := "Title,Due,Parent\nfirst,2026-01-02,\n,2026-01-03,\nthird,not a date,\nfourth,,NOPE-9\n"
	res := f.importTasks(t, file, core.TaskImportInput{})
	if res.Committed || res.Failed != 3 {
		t.Fatalf("import = committed %v failed %d errors %+v", res.Committed, res.Failed, res.Errors)
	}
	lines := map[int]bool{}
	for _, e := range res.Errors {
		lines[e.Line] = true
	}
	if !lines[3] || !lines[4] || !lines[5] {
		t.Errorf("errors name lines %v, want 3, 4 and 5", lines)
	}
	if n := f.count(t, "ops"); n != 0 {
		t.Errorf("a refused import left %d tasks", n)
	}
}

func TestImportRecordsOneAuditEntryWithoutTheFile(t *testing.T) {
	f := newImportFixture(t)
	f.importTasks(t, "Title,Body\nsecret title,secret body\n", core.TaskImportInput{})
	entries, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{Actions: []string{"task.import"}, Page: core.Page{Limit: 10}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d task.import entries, want 1", len(entries))
	}
	after := string(entries[0].After)
	if !strings.Contains(after, `"created":1`) || !strings.Contains(after, "mapping") {
		t.Errorf("the entry lacks the counts or the mapping: %s", after)
	}
	if strings.Contains(after, "secret") {
		t.Errorf("the entry carries the file's contents: %s", after)
	}
	creates, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{Actions: []string{"task.create"}, Page: core.Page{Limit: core.MaxPageLimit}})
	if err != nil {
		t.Fatal(err)
	}
	if len(creates) != 3 {
		t.Errorf("got %d task.create entries, want the two seeded and the imported one", len(creates))
	}
}

func TestImportReadsAListOfQuickAddLines(t *testing.T) {
	f := newImportFixture(t)
	res := f.importTasks(t, "- Fix login #bug !high\n\n# a note\nWrite notes ^ops-1\n", core.TaskImportInput{Format: core.TaskFormatLines})
	if !res.Committed || res.Created != 2 {
		t.Fatalf("import = %+v", res)
	}
	if res.Preview[0].Title != "Fix login" || res.Preview[0].Priority != core.PriorityHigh || res.Preview[0].Tags[0] != "bug" {
		t.Errorf("first line = %+v", res.Preview[0])
	}
	res = f.importTasks(t, "Ask @nobody\n", core.TaskImportInput{Format: core.TaskFormatLines})
	if res.Committed || res.Failed != 1 {
		t.Errorf("an unknown person on a line was accepted: %+v", res)
	}
}

func TestImportNeedsTheImportScopeAndStaysInAPinnedProject(t *testing.T) {
	f := newImportFixture(t)
	actor, _ := core.ActorFrom(f.ctx)
	ops, err := f.l.GetProject(f.ctx, "ops")
	if err != nil {
		t.Fatal(err)
	}
	member := *actor
	member.Role, member.Scopes = core.RoleMember, nil
	_, err = f.l.ImportTasks(core.WithActor(context.Background(), &member), strings.NewReader("Title\nx\n"),
		core.TaskImportInput{ProjectRef: "ops"})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a member without the import scope imported: %v", err)
	}
	pinned := *actor
	pinned.Role, pinned.Scopes, pinned.ProjectID = "", []core.Scope{core.ScopeImport}, ops.ID
	pctx := core.WithActor(context.Background(), &pinned)
	if _, err := f.l.ImportTasks(pctx, strings.NewReader("Title\nx\n"), core.TaskImportInput{ProjectRef: "ops"}); err != nil {
		t.Errorf("a token pinned to ops could not import into ops: %v", err)
	}
	_, err = f.l.ImportTasks(pctx, strings.NewReader("Title\nx\n"), core.TaskImportInput{ProjectRef: "infra"})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a token pinned to ops imported into infra: %v", err)
	}
}

func TestImportRefusesWhatItCannotRead(t *testing.T) {
	f := newImportFixture(t)
	for _, tc := range []struct {
		name    string
		content string
		in      core.TaskImportInput
	}{
		{"no project", "Title\nx\n", core.TaskImportInput{ProjectRef: " "}},
		{"bad mapping", "Title\nx\n", core.TaskImportInput{ProjectRef: "ops", Mapping: "columns: {Title: nonsense}"}},
		{"bad mode", "Title\nx\n", core.TaskImportInput{ProjectRef: "ops", Mode: "merge"}},
		{"empty list", "\n\n", core.TaskImportInput{ProjectRef: "ops", Format: core.TaskFormatLines}},
		{"too big", strings.Repeat("x", 70<<20), core.TaskImportInput{ProjectRef: "ops"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := f.l.ImportTasks(f.ctx, bytes.NewReader([]byte(tc.content)), tc.in)
			if !core.IsKind(err, core.KindInvalid) {
				t.Errorf("err = %v, want invalid", err)
			}
		})
	}
}
