// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func mustSaveTemplate(t *testing.T, l *Local, ctx context.Context, in core.SaveTemplateInput) *core.TaskTemplate {
	t.Helper()
	tpl, err := l.SaveTemplate(ctx, in)
	if err != nil {
		t.Fatalf("saving template %q: %v", in.Name, err)
	}
	return tpl
}

func dur(d time.Duration) *core.Duration {
	v := core.Duration(d)
	return &v
}

func TestSaveTemplateSnapshotsTheTaskAndItsSubtree(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: infra.ID, Key: "env", Label: "Env", Type: core.FieldString})
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		Title: "release {N}", Body: "- [ ] notes\n- [ ] tag", Priority: core.PriorityHigh,
		Tags: []string{"ops"}, Recurrence: "every week", CustomFields: map[string]any{"env": "prod"},
	})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "notes", ParentRef: root.Ref})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "proofread", ParentRef: child.Ref})
	gone := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "dropped", ParentRef: root.Ref})
	if err := l.DeleteTask(ctx, core.TaskRef{ID: gone.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatal(err)
	}
	beforeEvents, beforeAudits := countRows(t, l, scope)

	tpl := mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: " Release ", TaskRef: root.Ref, WithSubtasks: true})

	if tpl.Name != "Release" || tpl.ProjectID != infra.ID || tpl.ProjectKey != "infra" || tpl.Uses != 0 {
		t.Fatalf("template = %+v", tpl)
	}
	task := tpl.Task
	if task.Title != root.Title || task.Body != root.Body || task.Priority != core.PriorityHigh ||
		!slices.Equal(task.Tags, []string{"ops"}) || task.Recurrence != root.Recurrence || task.CustomFields["env"] != "prod" {
		t.Errorf("snapshot = %+v, want the task's shape", task)
	}
	if len(task.Subtasks) != 1 || task.Subtasks[0].Title != "notes" ||
		len(task.Subtasks[0].Subtasks) != 1 || task.Subtasks[0].Subtasks[0].Title != "proofread" {
		t.Errorf("subtasks = %+v, want notes > proofread and no deleted one", task.Subtasks)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("events %d->%d audits %d->%d, want one of each", beforeEvents, afterEvents, beforeAudits, afterAudits)
	}

	flat := mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "flat", TaskRef: root.Ref, TenantWide: true})
	if len(flat.Task.Subtasks) != 0 || !flat.TenantWide() {
		t.Errorf("without subtasks, tenant-wide: %+v", flat)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Tasks) != 3 {
		t.Errorf("listing holds %d tasks after saving templates, want the 3 live ones", len(page.Tasks))
	}
}

func TestSaveTemplateRefusals(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "x"})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "Release", TaskRef: src.Ref})

	cases := []struct {
		name  string
		in    core.SaveTemplateInput
		kind  core.Kind
		field string
	}{
		{"a name in another case", core.SaveTemplateInput{Name: "release", TaskRef: src.Ref}, core.KindConflict, "name"},
		{"no name", core.SaveTemplateInput{Name: "  ", TaskRef: src.Ref}, core.KindInvalid, "name"},
		{"no task", core.SaveTemplateInput{Name: "a"}, core.KindInvalid, "task_ref"},
		{"a missing task", core.SaveTemplateInput{Name: "a", TaskRef: "infra-99"}, core.KindNotFound, "task_ref"},
		{"a negative offset", core.SaveTemplateInput{Name: "a", TaskRef: src.Ref, DueIn: -1}, core.KindInvalid, "due_in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.SaveTemplate(ctx, tc.in)
			if !core.IsKind(err, tc.kind) || core.FieldOf(err) != tc.field {
				t.Fatalf("err = %v (field %q), want %s on %q", err, core.FieldOf(err), tc.kind, tc.field)
			}
		})
	}
}

func TestUseTemplateCreatesTheTreeThroughTheCopier(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: infra.ID, Key: "env", Label: "Env", Type: core.FieldString})
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		ProjectRef: "infra", Title: "standup {N} on {date} for {Project} {other}", Status: "doing",
		Tags: []string{"daily"}, CustomFields: map[string]any{"env": "prod"},
	})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "agenda {n}", ParentRef: root.Ref})
	tpl := mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "standup", TaskRef: root.Ref, WithSubtasks: true,
		DueIn: core.Duration(72 * time.Hour)})
	now := l.clock.Now()
	beforeEvents, beforeAudits := countRows(t, l, scope)

	got, err := l.UseTemplate(ctx, "STANDUP", core.UseTemplateInput{})
	if err != nil {
		t.Fatal(err)
	}
	wantTitle := "standup 1 on " + now.Format(time.DateOnly) + " for infra {other}"
	if got.Title != wantTitle || got.ProjectID != infra.ID || got.Status != "todo" || got.ID == root.ID {
		t.Fatalf("task = %q in %s status %s, want %q in infra, initial state", got.Title, got.ProjectID, got.Status, wantTitle)
	}
	if got.DueAt == nil || !got.DueAt.Equal(now.Add(72*time.Hour)) {
		t.Errorf("due = %v, want three days from now", got.DueAt)
	}
	if !slices.Equal(got.Tags, []string{"daily"}) || got.CustomFields["env"] != "prod" || got.CopiedFromID != "" {
		t.Errorf("tags %v fields %v link %q", got.Tags, got.CustomFields, got.CopiedFromID)
	}
	kids, err := l.TaskTree(ctx, core.TaskRef{ID: got.ID}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(kids) != 2 || kids[1].Title != "agenda 1" || kids[1].ParentID != got.ID || kids[1].DueAt != nil {
		t.Fatalf("tree = %+v, want the subtask under the new task", kids)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+2 || afterAudits != beforeAudits+2 {
		t.Errorf("events %d->%d audits %d->%d, want one per task", beforeEvents, afterEvents, beforeAudits, afterAudits)
	}

	second, err := l.UseTemplate(ctx, tpl.ID, core.UseTemplateInput{ProjectRef: "ops", Title: "run {N}", DueIn: dur(0)})
	if err != nil {
		t.Fatal(err)
	}
	if second.Title != "run 2" || !strings.HasPrefix(second.Ref, "ops-") || second.DueAt != nil || second.CustomFields["env"] != nil {
		t.Errorf("second use = %q %s due %v fields %v", second.Title, second.Ref, second.DueAt, second.CustomFields)
	}
	after, err := l.GetTemplate(ctx, "standup")
	if err != nil || after.Uses != 2 {
		t.Fatalf("uses = %v (%v), want 2", after, err)
	}
}

func TestUseTemplateRefusalsWriteNothing(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "shared", TaskRef: src.Ref, TenantWide: true})
	beforeEvents, _ := countRows(t, l, scope)

	cases := []struct {
		name, ref string
		in        core.UseTemplateInput
		kind      core.Kind
		field     string
	}{
		{"tenant-wide names no project", "shared", core.UseTemplateInput{}, core.KindInvalid, "project_ref"},
		{"a missing project", "shared", core.UseTemplateInput{ProjectRef: "nope"}, core.KindNotFound, "project_ref"},
		{"a missing template", "nope", core.UseTemplateInput{ProjectRef: "ops"}, core.KindNotFound, ""},
		{"an offset out of range", "shared", core.UseTemplateInput{ProjectRef: "ops", DueIn: dur(-time.Hour)}, core.KindInvalid, "due_in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := l.UseTemplate(ctx, tc.ref, tc.in)
			if !core.IsKind(err, tc.kind) || core.FieldOf(err) != tc.field {
				t.Fatalf("err = %v (field %q), want %s on %q", err, core.FieldOf(err), tc.kind, tc.field)
			}
		})
	}
	if events, _ := countRows(t, l, scope); events != beforeEvents {
		t.Errorf("a refused use wrote %d events", events-beforeEvents)
	}
}

func TestUpdateAndDeleteTemplate(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x", Tags: []string{"a"}})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "other", TaskRef: src.Ref})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "release", TaskRef: src.Ref})

	name, title, empty, ops := "ship", "Ship {N}", "", "ops"
	prio := core.PriorityLow
	tags := []string{" b ", "B", "c", ""}
	got, err := l.UpdateTemplate(ctx, "release", core.UpdateTemplateInput{
		Name: &name, Title: &title, Priority: &prio, Tags: &tags, ProjectRef: &ops, DueIn: dur(24 * time.Hour),
		CustomFields: map[string]any{"env": "prod"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "ship" || got.Task.Title != "Ship {N}" || got.Task.Priority != core.PriorityLow ||
		!slices.Equal(got.Task.Tags, []string{"b", "c"}) || got.ProjectKey != "ops" || got.DueIn != core.Duration(24*time.Hour) ||
		got.Task.CustomFields["env"] != "prod" {
		t.Fatalf("updated = %+v", got)
	}
	if _, err := l.GetTemplate(ctx, "release"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the old name still reads: %v", err)
	}
	got, err = l.UpdateTemplate(ctx, "ship", core.UpdateTemplateInput{ProjectRef: &empty, CustomFields: map[string]any{"env": nil}})
	if err != nil || !got.TenantWide() || got.Task.CustomFields != nil {
		t.Fatalf("to tenant-wide, field removed: %+v, %v", got, err)
	}
	clash := "OTHER"
	if _, err := l.UpdateTemplate(ctx, "ship", core.UpdateTemplateInput{Name: &clash}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("renaming onto another = %v, want conflict", err)
	}
	beforeEvents, _ := countRows(t, l, scope)
	if _, err := l.UpdateTemplate(ctx, "ship", core.UpdateTemplateInput{Name: &name}); err != nil {
		t.Fatal(err)
	}
	if events, _ := countRows(t, l, scope); events != beforeEvents {
		t.Errorf("an update changing nothing wrote %d events", events-beforeEvents)
	}

	if err := l.DeleteTemplate(ctx, "ship"); err != nil {
		t.Fatal(err)
	}
	if events, _ := countRows(t, l, scope); events != beforeEvents+1 {
		t.Errorf("delete wrote %d events, want 1", events-beforeEvents)
	}
	if err := l.DeleteTemplate(ctx, "ship"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleting twice = %v, want not found", err)
	}
	if _, err := l.GetTask(ctx, core.TaskRef{ID: src.ID}); err != nil {
		t.Errorf("deleting a template touched its source: %v", err)
	}
}

func TestListTemplatesPagesAndHonoursScope(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	opsTask := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "ops", Title: "y"})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "b-infra", TaskRef: src.Ref})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "A-shared", TaskRef: src.Ref, TenantWide: true})
	mustSaveTemplate(t, l, ctx, core.SaveTemplateInput{Name: "c-ops", TaskRef: opsTask.Ref})

	names := func(items []core.TaskTemplate) []string {
		var out []string
		for _, i := range items {
			out = append(out, i.Name)
		}
		return out
	}
	all, next, err := l.ListTemplates(ctx, core.TemplateFilter{})
	if err != nil || next != "" || !slices.Equal(names(all), []string{"A-shared", "b-infra", "c-ops"}) {
		t.Fatalf("all = %v next %q err %v", names(all), next, err)
	}
	first, next, err := l.ListTemplates(ctx, core.TemplateFilter{Page: core.Page{Limit: 2}})
	if err != nil || next == "" || !slices.Equal(names(first), []string{"A-shared", "b-infra"}) {
		t.Fatalf("first page = %v next %q err %v", names(first), next, err)
	}
	rest, next, err := l.ListTemplates(ctx, core.TemplateFilter{Page: core.Page{Limit: 2, Cursor: next}})
	if err != nil || next != "" || !slices.Equal(names(rest), []string{"c-ops"}) {
		t.Fatalf("second page = %v next %q err %v", names(rest), next, err)
	}
	ops, _, err := l.ListTemplates(ctx, core.TemplateFilter{ProjectRef: "ops"})
	if err != nil || !slices.Equal(names(ops), []string{"A-shared", "c-ops"}) {
		t.Fatalf("ops = %v err %v", names(ops), err)
	}

	pinned := seedTaskActor(t, l, scope, "pinned", core.RoleMember, core.ScopeAll)
	pinned.ProjectID = infra.ID
	pctx := taskContext(pinned)
	seen, _, err := l.ListTemplates(pctx, core.TemplateFilter{})
	if err != nil || !slices.Equal(names(seen), []string{"A-shared", "b-infra"}) {
		t.Fatalf("pinned sees %v err %v", names(seen), err)
	}
	if _, err := l.GetTemplate(pctx, "c-ops"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("pinned reading another project's template = %v, want not found", err)
	}
	if err := l.DeleteTemplate(pctx, "A-shared"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("pinned deleting a tenant-wide template = %v, want forbidden", err)
	}
	if _, err := l.SaveTemplate(pctx, core.SaveTemplateInput{Name: "z", TaskRef: src.Ref, TenantWide: true}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("pinned saving a tenant-wide template = %v, want forbidden", err)
	}
	if _, err := l.UseTemplate(pctx, "A-shared", core.UseTemplateInput{ProjectRef: "infra"}); err != nil {
		t.Errorf("pinned using a tenant-wide template in its project: %v", err)
	}
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	if err := l.DeleteTemplate(viewer, "b-infra"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a viewer deleting = %v, want forbidden", err)
	}
	if _, err := l.GetTemplate(viewer, "b-infra"); err != nil {
		t.Errorf("a viewer reading: %v", err)
	}
}

func TestFillPlaceholdersInTheService(t *testing.T) {
	sources := templateSources(core.TemplateTask{Title: "{DATE}", Subtasks: []core.TemplateTask{{Title: "{n}", Priority: 9}}},
		core.TemplateValues{Date: time.Date(2026, 11, 1, 23, 0, 0, 0, time.UTC), N: 5})
	if len(sources) != 2 || sources[0].Title != "2026-11-01" || sources[1].Title != "5" ||
		sources[1].ParentID != sources[0].ID || sources[1].Priority != core.PriorityNormal {
		t.Fatalf("sources = %+v", sources)
	}
}
