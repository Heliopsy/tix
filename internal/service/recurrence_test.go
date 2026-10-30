// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	extsync "github.com/heliopsy/tix/internal/sync"
)

type recurFixture struct {
	l     *Local
	clk   *clock.Fake
	scope core.TenantScope
	actor *core.Actor
	ctx   context.Context
}

// newRecurFixture is a tenant on the shipped workflow, whose done and
// cancelled states carry the two categories the spawn rule tells apart.
func newRecurFixture(t *testing.T) *recurFixture {
	t.Helper()
	l, clk, scope, actor := newLocal(t)
	clk.Set(time.Date(2026, 11, 2, 10, 0, 0, 0, time.UTC))
	ctx := taskContext(actor)
	if _, err := l.PutWorkflow(ctx, core.WorkflowInput{Key: "flow", Name: "Flow", Definition: BuiltinWorkflow()}); err != nil {
		t.Fatalf("workflow: %v", err)
	}
	if _, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "home", Name: "Home", WorkflowKey: "flow"}); err != nil {
		t.Fatalf("project: %v", err)
	}
	return &recurFixture{l: l, clk: clk, scope: scope, actor: actor, ctx: ctx}
}

func (f *recurFixture) create(t *testing.T, in core.CreateTaskInput) *core.Task {
	t.Helper()
	in.ProjectRef = "home"
	return mustCreateTask(t, f.l, f.ctx, in)
}

func (f *recurFixture) move(t *testing.T, task *core.Task, states ...string) *core.Task {
	t.Helper()
	var out *core.Task
	for _, to := range states {
		var err error
		out, err = f.l.TransitionTask(f.ctx, core.TaskRef{ID: task.ID}, core.TransitionInput{To: to})
		if err != nil {
			t.Fatalf("moving %s to %s: %v", task.Ref, to, err)
		}
	}
	return out
}

func (f *recurFixture) tasks(t *testing.T) []core.Task {
	t.Helper()
	page, err := f.l.ListTasks(f.ctx, core.TaskFilter{Page: core.Page{Sort: core.SortSeq, Limit: 100}})
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	return page.Tasks
}

func (f *recurFixture) get(t *testing.T, id string) *core.Task {
	t.Helper()
	task, err := f.l.GetTask(f.ctx, core.TaskRef{ID: id})
	if err != nil {
		t.Fatalf("getting %s: %v", id, err)
	}
	return task
}

func (f *recurFixture) events(t *testing.T) []core.Event {
	t.Helper()
	var out []core.Event
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		var err error
		out, err = tx.ReadEvents(context.Background(), 0, 1000)
		return err
	}); err != nil {
		t.Fatalf("reading events: %v", err)
	}
	return out
}

func utc(y int, m time.Month, d, h int) time.Time { return time.Date(y, m, d, h, 0, 0, 0, time.UTC) }

func TestFinishingARecurringTaskSpawnsTheNextOccurrence(t *testing.T) {
	f := newRecurFixture(t)
	if _, err := f.l.PutFieldDef(f.ctx, "home", core.FieldDefInput{Key: "points", Label: "Points", Type: core.FieldInt}); err != nil {
		t.Fatalf("field: %v", err)
	}
	due := utc(2026, 11, 2, 0)
	blocker := f.create(t, core.CreateTaskInput{Title: "buy soil"})
	task := f.create(t, core.CreateTaskInput{
		Title: "water plants", Body: "all of them", Priority: core.PriorityHigh, DueAt: &due,
		Tags: []string{"garden"}, Recurrence: "weekly", AssigneeActorID: f.actor.ID,
		CustomFields: map[string]any{"points": 2}, DependsOn: []string{blocker.Ref},
	})
	if task.Recurrence != "every week" {
		t.Fatalf("stored rule = %q, want the canonical every week", task.Recurrence)
	}
	if _, err := f.l.AddComment(f.ctx, core.TaskRef{ID: task.ID}, "watered"); err != nil {
		t.Fatalf("comment: %v", err)
	}
	f.move(t, blocker, "doing", "done")
	events, audits := countRows(t, f.l, f.scope)

	done := f.move(t, task, "doing", "done")

	if done.RecurrenceNextID == "" {
		t.Fatal("the finished task names no successor")
	}
	next := f.get(t, done.RecurrenceNextID)
	checks := []struct {
		name string
		ok   bool
	}{
		{"title", next.Title == "water plants"},
		{"body", next.Body == "all of them"},
		{"priority", next.Priority == core.PriorityHigh},
		{"assignee", next.AssigneeActorID == f.actor.ID},
		{"creator is the finisher", next.CreatorActorID == f.actor.ID},
		{"project", next.ProjectID == task.ProjectID},
		{"initial status", next.Status == "todo"},
		{"due advanced a week", next.DueAt != nil && next.DueAt.Equal(utc(2026, 11, 9, 0))},
		{"rule carried", next.Recurrence == "every week"},
		{"no successor of its own", next.RecurrenceNextID == ""},
		{"tags copied", len(next.Tags) == 1 && next.Tags[0] == "garden"},
		{"custom field copied", next.CustomFields["points"] != nil},
		{"no dependency", len(next.DependsOn) == 0},
		{"not started", next.StartedAt == nil && next.CompletedAt == nil},
		{"a fresh reference", next.Ref != task.Ref},
	}
	for _, c := range checks {
		if !c.ok {
			t.Errorf("successor %s wrong: %+v", c.name, next)
		}
	}
	comments, err := f.l.ListComments(f.ctx, core.TaskRef{ID: next.ID})
	if err != nil {
		t.Fatalf("comments: %v", err)
	}
	if len(comments) != 0 {
		t.Errorf("successor carries %d comments, want none", len(comments))
	}

	afterEvents, afterAudits := countRows(t, f.l, f.scope)
	if afterEvents-events != 3 || afterAudits-audits != 3 {
		t.Errorf("doing, done and the spawn wrote %d events and %d audits, want 3 and 3",
			afterEvents-events, afterAudits-audits)
	}
	all := f.events(t)
	finish, created := all[len(all)-2], all[len(all)-1]
	if finish.Type != core.EventTaskTransitioned || finish.Payload["next"] != next.Ref {
		t.Errorf("finish event = %s %v, want a transition naming next %s", finish.Type, finish.Payload, next.Ref)
	}
	if created.Type != core.EventTaskCreated || created.SubjectID != next.ID || created.Payload["recurs_from"] != task.Ref {
		t.Errorf("spawn event = %s %s %v, want task.created for %s recurring from %s",
			created.Type, created.SubjectID, created.Payload, next.Ref, task.Ref)
	}
}

func TestCancellingARecurringTaskEndsTheSeries(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "standup", Recurrence: "daily"})
	cancelled := f.move(t, task, "cancelled")
	if cancelled.RecurrenceNextID != "" {
		t.Fatalf("cancelling spawned %s", cancelled.RecurrenceNextID)
	}
	if n := len(f.tasks(t)); n != 1 {
		t.Fatalf("%d tasks after cancelling, want 1", n)
	}
}

func TestATaskSpawnsAtMostOnce(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "invoice", Recurrence: "monthly"})
	first := f.move(t, task, "doing", "done")
	if first.RecurrenceNextID == "" {
		t.Fatal("finishing did not spawn")
	}

	again := f.move(t, task, "todo", "doing", "done")
	if again.RecurrenceNextID != first.RecurrenceNextID {
		t.Fatalf("re-finishing relinked to %s, want %s", again.RecurrenceNextID, first.RecurrenceNextID)
	}
	if n := len(f.tasks(t)); n != 2 {
		t.Fatalf("%d tasks after reopening and finishing again, want 2", n)
	}

	rule := "every week"
	if _, err := f.l.UpdateTask(f.ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Recurrence: &rule}); err != nil {
		t.Fatalf("editing the rule: %v", err)
	}
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: first.RecurrenceNextID}, core.DeleteTaskInput{Hard: true}); err != nil {
		t.Fatalf("purging the successor: %v", err)
	}
	f.move(t, task, "todo", "doing", "done")
	if n := len(f.tasks(t)); n != 1 {
		t.Fatalf("%d tasks after purging the successor and finishing again, want 1", n)
	}
}

func TestARuleRunsOut(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "take pills", Recurrence: "every day for 2 times"})
	next := f.get(t, f.move(t, task, "doing", "done").RecurrenceNextID)
	if next.Recurrence != "every day for 1 time" {
		t.Fatalf("successor rule = %q, want one time left", next.Recurrence)
	}
	if last := f.move(t, next, "doing", "done"); last.RecurrenceNextID != "" {
		t.Fatal("the last occurrence spawned another")
	}
	if n := len(f.tasks(t)); n != 2 {
		t.Fatalf("%d tasks, want 2", n)
	}

	due := utc(2027, 6, 30, 0)
	until := f.create(t, core.CreateTaskInput{Title: "summer watering", DueAt: &due, Recurrence: "every week until 2027-07-05"})
	if done := f.move(t, until, "doing", "done"); done.RecurrenceNextID != "" {
		t.Fatal("a rule past its until date spawned")
	}
}

func TestTheNextDueDateFollowsTheAnchor(t *testing.T) {
	cases := []struct {
		name string
		rule string
		due  *time.Time
		want time.Time
	}{
		{"schedule skips missed weeks", "every week", ptr(utc(2026, 10, 1, 0)), utc(2026, 11, 5, 0)},
		{"after done counts from completion", "every 3 days after done", ptr(utc(2026, 10, 1, 9)), utc(2026, 11, 5, 9)},
		{"undated counts from completion", "every 2 weeks", nil, utc(2026, 11, 16, 0)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f := newRecurFixture(t)
			task := f.create(t, core.CreateTaskInput{Title: c.name, DueAt: c.due, Recurrence: c.rule})
			next := f.get(t, f.move(t, task, "doing", "done").RecurrenceNextID)
			if next.DueAt == nil || !next.DueAt.Equal(c.want) {
				t.Fatalf("next due = %v, want %s", next.DueAt, c.want)
			}
		})
	}
}

func ptr(t time.Time) *time.Time { return &t }

func TestCreatingADoneTaskDoesNotSpawn(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "already", Status: "done", Recurrence: "daily"})
	if task.RecurrenceNextID != "" || len(f.tasks(t)) != 1 {
		t.Fatal("creating a finished recurring task spawned another")
	}
}

func TestTheRuleIsValidatedSetAndCleared(t *testing.T) {
	f := newRecurFixture(t)
	if _, err := f.l.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: "home", Title: "x", Recurrence: "every fortnight"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an unparsable rule on create = %v, want invalid", err)
	}
	task := f.create(t, core.CreateTaskInput{Title: "plain"})
	ref := core.TaskRef{ID: task.ID}

	bad := "every blue moon"
	if _, err := f.l.UpdateTask(f.ctx, ref, core.UpdateTaskInput{Recurrence: &bad}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an unparsable rule on update = %v, want invalid", err)
	}
	set := "every 2 weeks on thu,mon"
	got, err := f.l.UpdateTask(f.ctx, ref, core.UpdateTaskInput{Recurrence: &set})
	if err != nil || got.Recurrence != "every 2 weeks on mon,thu" {
		t.Fatalf("setting the rule = %q, %v", got.Recurrence, err)
	}
	title := "renamed"
	got, err = f.l.UpdateTask(f.ctx, ref, core.UpdateTaskInput{Title: &title})
	if err != nil || got.Recurrence != "every 2 weeks on mon,thu" {
		t.Fatalf("an unrelated update left the rule %q, %v", got.Recurrence, err)
	}
	clear := ""
	got, err = f.l.UpdateTask(f.ctx, ref, core.UpdateTaskInput{Recurrence: &clear})
	if err != nil || got.Recurrence != "" {
		t.Fatalf("clearing the rule left %q, %v", got.Recurrence, err)
	}
	last := f.events(t)
	if fields, _ := last[len(last)-1].Payload["fields"].([]any); len(fields) != 1 || fields[0] != "recurrence" {
		t.Errorf("update event names fields %v, want recurrence", last[len(last)-1].Payload["fields"])
	}
}

// TestASpawnThatFailsStoresNeither finishes a task whose tag cannot be copied,
// which fails after the successor row is written and before the finish is,
// and checks that the transaction took both with it.
func TestASpawnThatFailsStoresNeither(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "doomed", Recurrence: "daily"})
	f.move(t, task, "doing")
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		tag := core.Tag{Name: "   "}
		if err := tx.PutTag(context.Background(), &tag); err != nil {
			return err
		}
		return tx.AttachTag(context.Background(), task.ID, tag.ID)
	}); err != nil {
		t.Fatalf("seeding an uncopyable tag: %v", err)
	}
	events, audits := countRows(t, f.l, f.scope)

	if _, err := f.l.TransitionTask(f.ctx, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "done"}); err == nil {
		t.Fatal("finishing succeeded although the successor could not be written")
	}
	if got := f.get(t, task.ID); got.Status != "doing" || got.RecurrenceNextID != "" {
		t.Fatalf("after the failed finish the task is %s linked to %q", got.Status, got.RecurrenceNextID)
	}
	if n := len(f.tasks(t)); n != 1 {
		t.Fatalf("%d tasks after the failed finish, want 1", n)
	}
	if e, a := countRows(t, f.l, f.scope); e != events || a != audits {
		t.Fatalf("the failed finish left %d events and %d audits behind", e-events, a-audits)
	}
}

func TestAReleaseThatFinishesTheTaskSpawns(t *testing.T) {
	f := newClaimFixture(t, 0)
	task := f.seedTask(t, "rotate keys", "todo", core.PriorityNormal)
	rule := "every 3 months"
	if _, err := f.local.UpdateTask(f.ctx, ref(task), core.UpdateTaskInput{Recurrence: &rule}); err != nil {
		t.Fatalf("setting the rule: %v", err)
	}
	claim := mustClaim(t, f, task)
	if _, err := f.local.TransitionTask(f.ctx, ref(task), core.TransitionInput{To: "doing", LeaseToken: claim.LeaseToken}); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if err := f.local.ReleaseLease(f.ctx, ref(task), claim.LeaseToken, core.ReleaseInput{Status: "done"}); err != nil {
		t.Fatalf("releasing as done: %v", err)
	}
	done := f.reload(t, task)
	if done.RecurrenceNextID == "" {
		t.Fatal("a release that finished the task did not spawn")
	}
	if created := f.events(t, core.EventTaskCreated); len(created) != 1 || created[0].Payload["recurs_from"] != task.Ref {
		t.Fatalf("created events = %+v, want one recurring from %s", created, task.Ref)
	}
}

func TestAnUncategorisedTerminalStateCompletesWork(t *testing.T) {
	def := claimWorkflowDefinition(0)
	if !def.CompletesWork("done") || !def.CompletesWork("audit") {
		t.Fatal("a terminal state naming no category must complete work")
	}
	if def.CompletesWork("doing") || BuiltinWorkflow().CompletesWork("cancelled") {
		t.Fatal("a non-terminal or cancelled state completed work")
	}
}

func TestRecurrenceSurvivesExportAndImport(t *testing.T) {
	f := newRecurFixture(t)
	due := utc(2026, 11, 2, 0)
	finished := f.create(t, core.CreateTaskInput{Title: "mow", DueAt: &due, Recurrence: "every 2 weeks on mon"})
	f.move(t, finished, "doing", "done")
	snapshot := exportBytes(t, f.l, f.ctx, core.ExportInput{})

	target, _, _ := newTenant(t, f.l, "beta")
	result, err := f.l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.Created["task"] != 2 {
		t.Fatalf("import created %v, want exactly the two snapshot tasks", result.Created)
	}
	page, err := f.l.ListTasks(target, core.TaskFilter{Page: core.Page{Sort: core.SortSeq, Limit: 10}})
	if err != nil || len(page.Tasks) != 2 {
		t.Fatalf("imported %d tasks, %v", len(page.Tasks), err)
	}
	old, next := page.Tasks[0], page.Tasks[1]
	if old.Recurrence != "every 2 weeks on mon" || next.Recurrence != "every 2 weeks on mon" {
		t.Fatalf("imported rules = %q and %q", old.Recurrence, next.Recurrence)
	}
	if old.RecurrenceNextID != next.ID {
		t.Fatalf("imported link = %q, want the imported successor %q", old.RecurrenceNextID, next.ID)
	}

	again := exportBytes(t, f.l, target, core.ExportInput{})
	if !bytes.Contains(again, []byte(`"recurrence":"every 2 weeks on mon"`)) || !bytes.Contains(again, []byte(`"recurrence_next_id":"`+next.ID+`"`)) {
		t.Fatalf("re-export lost the rule or the link:\n%s", again)
	}

	reopened, err := f.l.TransitionTask(target, core.TaskRef{ID: old.ID}, core.TransitionInput{To: "todo"})
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	for _, to := range []string{"doing", "done"} {
		if reopened, err = f.l.TransitionTask(target, core.TaskRef{ID: old.ID}, core.TransitionInput{To: to}); err != nil {
			t.Fatalf("moving to %s: %v", to, err)
		}
	}
	if page, _ := f.l.ListTasks(target, core.TaskFilter{}); len(page.Tasks) != 2 {
		t.Fatalf("an imported finished task spawned again: %d tasks", len(page.Tasks))
	}
	_ = reopened
}

func TestImportRefusesAnUnparsableRule(t *testing.T) {
	f := newRecurFixture(t)
	task := f.create(t, core.CreateTaskInput{Title: "odd"})
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		task.Recurrence = "every fortnight"
		return tx.UpdateTask(context.Background(), task)
	}); err != nil {
		t.Fatalf("corrupting the rule: %v", err)
	}
	snapshot := exportBytes(t, f.l, f.ctx, core.ExportInput{})
	target, _, _ := newTenant(t, f.l, "beta")
	if _, err := f.l.ImportFrom(target, bytes.NewReader(snapshot), core.ImportInput{Mode: core.ImportMerge}); err == nil ||
		!bytes.Contains([]byte(err.Error()), []byte("fortnight")) {
		t.Fatalf("import of an unparsable rule = %v, want a rejection naming it", err)
	}
}

func TestASyncThatFinishesARecurringTaskNeitherSpawnsNorTouchesTheRule(t *testing.T) {
	f := newSyncFixture(t, "ops")
	useRelationMapping(t, f)
	first := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{relationRecord("U-1", "weekly report", "To Do", "2026-01-01T00:00:00Z", nil)},
		Cursor:  "2026-01-01T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return first, nil
	})
	f.run(t, core.RunSyncInput{})
	tasks := f.tasks(t)
	if len(tasks) != 1 {
		t.Fatalf("first sync made %d tasks", len(tasks))
	}
	rule := "every week"
	if _, err := f.local.UpdateTask(f.ctx, core.TaskRef{ID: tasks[0].ID}, core.UpdateTaskInput{Recurrence: &rule}); err != nil {
		t.Fatalf("setting the rule: %v", err)
	}

	second := &fakeImporter{failAt: -1, pages: []extsync.Batch{{
		Records: []extsync.Record{relationRecord("U-1", "weekly report", "Done", "2026-02-01T00:00:00Z", nil)},
		Cursor:  "2026-02-01T00:00:00Z",
	}}}
	useImporter(t, func(string, extsync.SourceConfig, *extsync.Mapping) (extsync.Importer, error) {
		return second, nil
	})
	f.run(t, core.RunSyncInput{})
	after := f.tasks(t)
	if len(after) != 1 {
		t.Fatalf("a sync that finished a recurring task left %d tasks, want 1", len(after))
	}
	if after[0].Status != "done" || after[0].Recurrence != "every week" || after[0].RecurrenceNextID != "" {
		t.Fatalf("after the sync the task is %s carrying %q linked to %q", after[0].Status, after[0].Recurrence, after[0].RecurrenceNextID)
	}
}
