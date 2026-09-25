// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// taskWorkflow is the state machine the task tests exercise. "review" to "done"
// requires a comment and "cancelled" requires a scope, so the transition
// requirements have an edge each.
func taskWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "Todo", Category: core.CategoryTodo},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress},
			{Key: "review", Label: "Review", Category: core.CategoryInProgress},
			{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
			{Key: "cancelled", Label: "Cancelled", Terminal: true, Category: core.CategoryDone},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "doing", To: "todo"},
			{From: "doing", To: "review"},
			{From: "review", To: "done", RequiresComment: true},
			{From: "doing", To: "done"},
			{From: "todo", To: "cancelled", RequiresScope: core.ScopeTaskDelete},
		},
	}
}

func seedTaskProject(t *testing.T, l *Local, scope core.TenantScope, key string) *core.Project {
	t.Helper()
	ctx := context.Background()
	wf := core.Workflow{Key: "wf-" + key, Name: "Workflow " + key, Definition: taskWorkflow()}
	project := core.Project{Key: key, Name: strings.ToUpper(key)}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding project %q: %v", key, err)
	}
	return &project
}

func seedFieldDef(t *testing.T, l *Local, scope core.TenantScope, d core.FieldDef) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.PutFieldDef(ctx, &d)
	}); err != nil {
		t.Fatalf("seeding field %q: %v", d.Key, err)
	}
}

func seedTaskActor(t *testing.T, l *Local, scope core.TenantScope, handle string, role core.Role, scopes ...core.Scope) *core.Actor {
	t.Helper()
	ctx := context.Background()
	actor := core.Actor{Kind: core.ActorUser, Handle: handle, Role: role, Scopes: scopes}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("seeding actor %q: %v", handle, err)
	}
	actor.TenantID = scope.TenantID
	return &actor
}

func taskContext(actor *core.Actor) context.Context {
	return core.WithSource(core.WithActor(context.Background(), actor), core.SourceCLI)
}

func newTaskFixture(t *testing.T) (*Local, context.Context, core.TenantScope, *core.Actor, *core.Project) {
	t.Helper()
	l, _, scope, actor := newLocal(t)
	project := seedTaskProject(t, l, scope, "infra")
	return l, taskContext(actor), scope, actor, project
}

func mustCreateTask(t *testing.T, l *Local, ctx context.Context, in core.CreateTaskInput) *core.Task {
	t.Helper()
	task, err := l.CreateTask(ctx, in)
	if err != nil {
		t.Fatalf("creating task %q: %v", in.Title, err)
	}
	return task
}

func TestCreateTaskWithOnlyATitle(t *testing.T) {
	l, ctx, scope, actor, project := newTaskFixture(t)
	beforeEvents, beforeAudits := countRows(t, l, scope)

	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "  ship it  "})

	if task.Title != "ship it" {
		t.Errorf("title = %q, want the trimmed title", task.Title)
	}
	if task.ProjectID != project.ID {
		t.Errorf("project = %q, want the only project %q", task.ProjectID, project.ID)
	}
	if task.Status != "todo" {
		t.Errorf("status = %q, want the workflow's initial state", task.Status)
	}
	if task.Priority != core.PriorityNormal {
		t.Errorf("priority = %d, want normal", task.Priority)
	}
	if task.Ref != "infra-1" {
		t.Errorf("ref = %q, want infra-1", task.Ref)
	}
	if task.CreatorActorID != actor.ID {
		t.Errorf("creator = %q, want %q", task.CreatorActorID, actor.ID)
	}
	if task.AssigneeActorID != "" || task.DueAt != nil || len(task.Tags) != 0 {
		t.Errorf("defaults leaked: %+v", task)
	}
	if task.Blocked {
		t.Error("a task with no dependencies must report unblocked")
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("create wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
}

func TestCreateTaskAssignsSequentialHumanRefs(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)

	for i := 1; i <= 3; i++ {
		task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: fmt.Sprintf("task %d", i)})
		want := fmt.Sprintf("infra-%d", i)
		if task.Ref != want {
			t.Fatalf("ref = %q, want %q", task.Ref, want)
		}
		if task.Seq != int64(i) {
			t.Fatalf("seq = %d, want %d", task.Seq, i)
		}
	}
}

func TestCreateTaskRejectsBadInput(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)

	cases := []struct {
		name string
		in   core.CreateTaskInput
		kind core.Kind
	}{
		{"empty title", core.CreateTaskInput{Title: "   "}, core.KindInvalid},
		{"long title", core.CreateTaskInput{Title: strings.Repeat("x", core.MaxTitleLength+1)}, core.KindInvalid},
		{"long body", core.CreateTaskInput{Title: "t", Body: strings.Repeat("x", core.MaxBodyLength+1)}, core.KindInvalid},
		{"bad priority", core.CreateTaskInput{Title: "t", Priority: 9}, core.KindInvalid},
		{"unknown project", core.CreateTaskInput{Title: "t", ProjectRef: "nope"}, core.KindNotFound},
		{"unknown status", core.CreateTaskInput{Title: "t", Status: "nowhere"}, core.KindInvalid},
		{"unknown parent", core.CreateTaskInput{Title: "t", ParentRef: "infra-99"}, core.KindNotFound},
		{"unknown dependency", core.CreateTaskInput{Title: "t", DependsOn: []string{"infra-99"}}, core.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateTask(ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("CreateTask = %v, want %s", err, tc.kind)
			}
		})
	}
}

func TestCreateTaskStoresEveryAttribute(t *testing.T) {
	l, ctx, _, actor, project := newTaskFixture(t)
	due := time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	seedFieldDef(t, l, core.TenantScope{TenantID: actor.TenantID}, core.FieldDef{
		ProjectID: project.ID, Key: "severity", Label: "Severity",
		Type: core.FieldEnum, EnumOptions: []string{"low", "high"},
	})

	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		ProjectRef:      "infra",
		Title:           "full",
		Body:            "body",
		Status:          "doing",
		Priority:        core.PriorityHigh,
		Tags:            []string{"urgent", "infra"},
		AssigneeActorID: actor.ID,
		ParentRef:       parent.Ref,
		DueAt:           &due,
		CustomFields:    map[string]any{"severity": "high"},
		DependsOn:       []string{parent.Ref},
	})

	if task.Body != "body" || task.Status != "doing" || task.Priority != core.PriorityHigh {
		t.Errorf("attributes not stored: %+v", task)
	}
	if task.ParentID != parent.ID || task.AssigneeActorID != actor.ID {
		t.Errorf("relations not stored: %+v", task)
	}
	if task.DueAt == nil || !task.DueAt.Equal(due) {
		t.Errorf("due date = %v, want %v", task.DueAt, due)
	}
	if len(task.Tags) != 2 {
		t.Errorf("tags = %v, want two", task.Tags)
	}
	if task.CustomFields["severity"] != "high" {
		t.Errorf("custom fields = %v", task.CustomFields)
	}
	if !task.Blocked {
		t.Error("a task depending on an open task must report blocked")
	}
}

func TestGetTaskAcceptsBothReferenceForms(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	created := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "addressable"})

	byID, err := l.GetTask(ctx, core.TaskRef{ID: created.ID})
	if err != nil {
		t.Fatalf("GetTask by id: %v", err)
	}
	byRef, err := l.GetTask(ctx, core.MustParseTaskRef(created.Ref))
	if err != nil {
		t.Fatalf("GetTask by ref: %v", err)
	}
	if byID.ID != byRef.ID {
		t.Errorf("the two reference forms addressed different tasks: %q and %q", byID.ID, byRef.ID)
	}
	if _, err := l.GetTask(ctx, core.MustParseTaskRef("infra-404")); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("unknown reference = %v, want not found", err)
	}
}

func TestUpdateTaskChangesOnlyWhatIsSupplied(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		Title: "original", Body: "body", Priority: core.PriorityLow, Tags: []string{"keep"},
	})

	assignee := actor.ID
	updated, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{AssigneeActorID: &assignee})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.AssigneeActorID != actor.ID {
		t.Errorf("assignee = %q, want %q", updated.AssigneeActorID, actor.ID)
	}
	if updated.Title != "original" || updated.Body != "body" || updated.Priority != core.PriorityLow {
		t.Errorf("an unrelated attribute changed: %+v", updated)
	}
	if len(updated.Tags) != 1 || updated.Tags[0] != "keep" {
		t.Errorf("tags = %v, want the original tag", updated.Tags)
	}
	if updated.Version <= task.Version {
		t.Errorf("version did not move: %d then %d", task.Version, updated.Version)
	}
}

func TestUpdateTaskClearsAndReplaces(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	due := time.Date(2031, 5, 6, 7, 8, 9, 0, time.UTC)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t", DueAt: &due, Tags: []string{"a", "b"}})

	var cleared *time.Time
	title := "renamed"
	tags := []string{"b", "c"}
	updated, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
		Title: &title, DueAt: &cleared, Tags: &tags,
	})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if updated.DueAt != nil {
		t.Errorf("due date = %v, want cleared", updated.DueAt)
	}
	if updated.Title != "renamed" {
		t.Errorf("title = %q", updated.Title)
	}
	if strings.Join(updated.Tags, ",") != "b,c" {
		t.Errorf("tags = %v, want exactly b and c", updated.Tags)
	}
}

func TestUpdateTaskRejectsStaleVersion(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "contended"})
	first := "first"
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
		Title: &first, Version: task.Version,
	}); err != nil {
		t.Fatalf("first update: %v", err)
	}

	second := "second"
	_, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
		Title: &second, Version: task.Version,
	})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("stale update = %v, want conflict", err)
	}
	got, err := l.GetTask(ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != "first" {
		t.Errorf("title = %q, want the first update to have survived", got.Title)
	}

	third := "third"
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
		Title: &third, Version: got.Version,
	}); err != nil {
		t.Errorf("update at the current version = %v, want success", err)
	}
}

func TestUpdateTaskRejectsInvalidValues(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "guarded"})
	ref := core.TaskRef{ID: task.ID}

	empty := "  "
	long := strings.Repeat("x", core.MaxTitleLength+1)
	body := strings.Repeat("x", core.MaxBodyLength+1)
	bad := core.Priority(42)
	self := task.Ref
	cases := []struct {
		name string
		in   core.UpdateTaskInput
		kind core.Kind
	}{
		{"empty title", core.UpdateTaskInput{Title: &empty}, core.KindInvalid},
		{"long title", core.UpdateTaskInput{Title: &long}, core.KindInvalid},
		{"long body", core.UpdateTaskInput{Body: &body}, core.KindInvalid},
		{"bad priority", core.UpdateTaskInput{Priority: &bad}, core.KindInvalid},
		{"self parent", core.UpdateTaskInput{ParentRef: &self}, core.KindInvalid},
		{"unknown field", core.UpdateTaskInput{CustomFields: map[string]any{"nope": 1}}, core.KindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.UpdateTask(ctx, ref, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("UpdateTask = %v, want %s", err, tc.kind)
			}
		})
	}
	got, err := l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Title != "guarded" {
		t.Errorf("a rejected update changed the task: %+v", got)
	}
}

func TestUpdateTaskRejectsParentCycle(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "root"})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: root.Ref})

	ref := child.Ref
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: root.ID}, core.UpdateTaskInput{ParentRef: &ref}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("parent cycle = %v, want invalid", err)
	}

	none := ""
	detached, err := l.UpdateTask(ctx, core.TaskRef{ID: child.ID}, core.UpdateTaskInput{ParentRef: &none})
	if err != nil {
		t.Fatalf("detaching: %v", err)
	}
	if detached.ParentID != "" {
		t.Errorf("parent = %q, want detached", detached.ParentID)
	}
}

func TestTransitionRejectsIllegalTarget(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}

	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "done"}); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("todo to done = %v, want precondition failed", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "nowhere"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("unknown state = %v, want invalid", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("missing target = %v, want invalid", err)
	}
	got, err := l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.Status != "todo" {
		t.Errorf("status = %q, want unchanged", got.Status)
	}
}

func TestTransitionRequiringAComment(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "reviewed"})
	ref := core.TaskRef{ID: task.ID}

	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "doing"}); err != nil {
		t.Fatalf("todo to doing: %v", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "review"}); err != nil {
		t.Fatalf("doing to review: %v", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "done"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("review to done without a comment = %v, want invalid", err)
	}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	done, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "done", Comment: "looks good"})
	if err != nil {
		t.Fatalf("review to done with a comment: %v", err)
	}
	if done.Status != "done" {
		t.Errorf("status = %q, want done", done.Status)
	}
	if done.CompletedAt == nil {
		t.Error("a terminal state must record a completion time")
	}
	comments, err := l.ListComments(ctx, ref)
	if err != nil {
		t.Fatalf("ListComments: %v", err)
	}
	if len(comments) != 1 || comments[0].Body != "looks good" {
		t.Errorf("comments = %+v, want the transition's comment", comments)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("transition wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
}

func TestTransitionRequiringAScope(t *testing.T) {
	l, admin, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, admin, core.CreateTaskInput{Title: "cancellable"})

	member := taskContext(seedTaskActor(t, l, scope, "member", "",
		core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
		core.ScopeProjectRead, core.ScopeWorkflowRead))
	if _, err := l.TransitionTask(member, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "cancelled"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("transition without the required scope = %v, want forbidden", err)
	}
	if _, err := l.TransitionTask(admin, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "cancelled"}); err != nil {
		t.Errorf("transition with the required scope = %v, want success", err)
	}
}

func TestTransitionUnderALeaseNeedsTheToken(t *testing.T) {
	l, ctx, scope, actor, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "claimed"})

	const token = "lease-token"
	if err := l.store.Update(context.Background(), scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(context.Background(), store.ClaimRow{
			TaskID: task.ID, ActorID: actor.ID,
			Now: l.clock.Now(), Until: l.clock.Now().Add(time.Hour), LeaseToken: token,
		})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("seeding a claim did not take the lease")
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	ref := core.TaskRef{ID: task.ID}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "doing"}); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("transition with no lease token = %v, want lease expired", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "doing", LeaseToken: "wrong"}); !core.IsKind(err, core.KindLeaseExpired) {
		t.Errorf("transition with the wrong lease token = %v, want lease expired", err)
	}
	got, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "doing", LeaseToken: token})
	if err != nil {
		t.Fatalf("transition with the matching lease token: %v", err)
	}
	if got.Status != "doing" {
		t.Errorf("status = %q, want doing", got.Status)
	}
	if got.StartedAt == nil {
		t.Error("leaving the initial state must record a start time")
	}
}

func TestTransitionChecksVersionAndCustomFields(t *testing.T) {
	l, ctx, scope, _, project := newTaskFixture(t)
	seedFieldDef(t, l, scope, core.FieldDef{
		ProjectID: project.ID, Key: "reason", Label: "Reason", Type: core.FieldString,
	})
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}

	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{To: "doing", Version: task.Version + 5}); !core.IsKind(err, core.KindConflict) {
		t.Errorf("transition at a stale version = %v, want conflict", err)
	}
	if _, err := l.TransitionTask(ctx, ref, core.TransitionInput{
		To: "doing", CustomFields: map[string]any{"reason": 7},
	}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("transition with a mistyped field = %v, want invalid", err)
	}
	got, err := l.TransitionTask(ctx, ref, core.TransitionInput{
		To: "doing", Version: task.Version, CustomFields: map[string]any{"reason": "starting"},
	})
	if err != nil {
		t.Fatalf("transition: %v", err)
	}
	if got.CustomFields["reason"] != "starting" {
		t.Errorf("custom fields = %v", got.CustomFields)
	}
}

func TestSoftDeleteHidesAndRestoreBringsBack(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "temporary"})
	ref := core.TaskRef{ID: task.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.DeleteTask(ctx, ref, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("delete wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	if _, err := l.GetTask(ctx, ref); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("reading a soft-deleted task = %v, want not found", err)
	}
	page, err := l.ListTasks(ctx, core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("a soft-deleted task still lists: %+v", page.Tasks)
	}

	restored, err := l.RestoreTask(ctx, ref)
	if err != nil {
		t.Fatalf("RestoreTask: %v", err)
	}
	if restored.Deleted() || restored.Title != "temporary" {
		t.Errorf("restored task = %+v", restored)
	}
	page, err = l.ListTasks(ctx, core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Errorf("restored task does not list again: %+v", page.Tasks)
	}
}

func TestHardDeleteIsPermanent(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "gone"})
	ref := core.TaskRef{ID: task.ID}

	if err := l.DeleteTask(ctx, ref, core.DeleteTaskInput{Hard: true}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := l.RestoreTask(ctx, ref); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("restoring a hard-deleted task = %v, want not found", err)
	}
}

func TestReferencesAreNotReusedAfterADelete(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "retired"})
	if err := l.DeleteTask(ctx, core.TaskRef{ID: task.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	next := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "successor"})
	if next.Ref == task.Ref {
		t.Errorf("reference %q was reused after a delete", next.Ref)
	}
}

func TestDeleteTaskWithChildrenIsRejected(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: parent.Ref})

	if err := l.DeleteTask(ctx, core.TaskRef{ID: parent.ID}, core.DeleteTaskInput{}); !core.IsKind(err, core.KindPrecondition) {
		t.Errorf("deleting a parent = %v, want precondition failed", err)
	}
}

func TestTaskTreeReportsDescendants(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "root"})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: root.Ref})
	grand := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "grandchild", ParentRef: child.Ref})
	leaf := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "leaf"})

	full, err := l.TaskTree(ctx, core.TaskRef{ID: root.ID}, 0)
	if err != nil {
		t.Fatalf("TaskTree: %v", err)
	}
	if len(full) != 3 || full[0].ID != root.ID || full[2].ID != grand.ID {
		t.Errorf("tree = %d nodes %v, want root, child, grandchild", len(full), taskTitles(full))
	}

	shallow, err := l.TaskTree(ctx, core.TaskRef{ID: root.ID}, 1)
	if err != nil {
		t.Fatalf("TaskTree with a depth limit: %v", err)
	}
	if len(shallow) != 2 {
		t.Errorf("depth-limited tree = %v, want the root and its child", taskTitles(shallow))
	}

	only, err := l.TaskTree(ctx, core.TaskRef{ID: leaf.ID}, 0)
	if err != nil {
		t.Fatalf("TaskTree of a leaf: %v", err)
	}
	if len(only) != 1 {
		t.Errorf("leaf tree = %v, want just the leaf", taskTitles(only))
	}

	if err := l.DeleteTask(ctx, core.TaskRef{ID: grand.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("deleting a descendant: %v", err)
	}
	pruned, err := l.TaskTree(ctx, core.TaskRef{ID: root.ID}, 0)
	if err != nil {
		t.Fatalf("TaskTree: %v", err)
	}
	if len(pruned) != 2 {
		t.Errorf("tree = %v, want the soft-deleted descendant omitted", taskTitles(pruned))
	}
}

func taskTitles(tasks []core.Task) []string {
	out := make([]string, len(tasks))
	for i, task := range tasks {
		out[i] = task.Title
	}
	return out
}

func TestListTasksFiltersAndSorts(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		Title: "alpha needle", Priority: core.PriorityHighest, Tags: []string{"red"},
		AssigneeActorID: actor.ID,
	})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "beta", Priority: core.PriorityLowest})

	cases := []struct {
		name  string
		f     core.TaskFilter
		count int
	}{
		{"by status", core.TaskFilter{Statuses: []string{"todo"}}, 2},
		{"by tag", core.TaskFilter{Tags: []string{"red"}}, 1},
		{"by assignee", core.TaskFilter{AssigneeIDs: []string{actor.ID}}, 1},
		{"by priority", core.TaskFilter{Priorities: []core.Priority{core.PriorityHighest}}, 1},
		{"by query", core.TaskFilter{Query: "needle"}, 1},
		{"combined", core.TaskFilter{Tags: []string{"red"}, Statuses: []string{"todo"}, Blocked: core.No}, 1},
		{"no match", core.TaskFilter{Statuses: []string{"done"}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(page.Tasks) != tc.count {
				t.Errorf("returned %d tasks, want %d", len(page.Tasks), tc.count)
			}
		})
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{Sort: core.SortPriority, Direction: core.Ascending}})
	if err != nil {
		t.Fatalf("ListTasks sorted: %v", err)
	}
	if len(page.Tasks) != 2 || page.Tasks[0].ID != a.ID {
		t.Errorf("sorted order = %v, want the highest priority first", taskTitles(page.Tasks))
	}
	if _, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{Sort: "colour"}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("unsupported sort = %v, want invalid", err)
	}
	if _, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: -1}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("negative limit = %v, want invalid", err)
	}
}

func TestListTasksPagesOverEveryRowExactlyOnce(t *testing.T) {
	l, _, scope, actor, _ := newTaskFixture(t)
	ctx := taskContext(actor)
	clk := l.clock.(interface{ Advance(time.Duration) })

	const total = 23
	want := map[string]bool{}
	for i := range total {
		task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: fmt.Sprintf("task-%02d", i)})
		want[task.ID] = true
		clk.Advance(time.Second)
	}
	_ = scope

	for _, dir := range []core.SortDirection{core.Ascending, core.Descending} {
		t.Run(string(dir), func(t *testing.T) {
			seen := map[string]int{}
			cursor := ""
			for page := 0; page <= total; page++ {
				got, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{
					Limit: 7, Cursor: cursor, Sort: core.SortCreatedAt, Direction: dir,
				}})
				if err != nil {
					t.Fatalf("listing page %d: %v", page, err)
				}
				for _, task := range got.Tasks {
					seen[task.ID]++
				}
				if got.NextCursor == "" {
					break
				}
				if len(got.Tasks) != 7 {
					t.Fatalf("page %d returned %d tasks with a next cursor", page, len(got.Tasks))
				}
				cursor = got.NextCursor
			}
			if len(seen) != total {
				t.Errorf("saw %d distinct tasks, want %d", len(seen), total)
			}
			for id, n := range seen {
				if n != 1 {
					t.Errorf("task %q appeared %d times", id, n)
				}
			}
		})
	}

	if _, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{
		Cursor: core.Cursor{SortValue: "x", ID: "y", Sort: core.SortCreatedAt, Direction: core.Ascending}.Encode(),
		Sort:   core.SortSeq,
	}}); !core.IsKind(err, core.KindInvalid) {
		t.Error("a cursor from a different ordering must be rejected")
	}
}

func TestCustomFieldValuesAreValidated(t *testing.T) {
	l, ctx, scope, _, project := newTaskFixture(t)
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "count", Label: "Count", Type: core.FieldInt})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "ratio", Label: "Ratio", Type: core.FieldFloat})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "urgent", Label: "Urgent", Type: core.FieldBool})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "when", Label: "When", Type: core.FieldDate})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "at", Label: "At", Type: core.FieldDateTime})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: project.ID, Key: "blob", Label: "Blob", Type: core.FieldJSON})
	seedFieldDef(t, l, scope, core.FieldDef{
		ProjectID: project.ID, Key: "severity", Label: "Severity",
		Type: core.FieldEnum, EnumOptions: []string{"low", "high"},
	})
	seedFieldDef(t, l, scope, core.FieldDef{
		ProjectID: project.ID, Key: "owner", Label: "Owner", Type: core.FieldActor,
		Required: true, Default: "nobody",
	})

	good := map[string]any{
		"count": 3, "ratio": 1.5, "urgent": true, "when": "2030-01-02",
		"at": "2030-01-02T03:04:05Z", "blob": map[string]any{"k": "v"}, "severity": "low",
	}
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "typed", CustomFields: good})
	if task.CustomFields["owner"] != "nobody" {
		t.Errorf("the declared default was not applied: %v", task.CustomFields)
	}

	bad := []struct {
		name   string
		fields map[string]any
	}{
		{"unknown key", map[string]any{"nope": 1}},
		{"int as text", map[string]any{"count": "three"}},
		{"float as text", map[string]any{"ratio": "half"}},
		{"bool as text", map[string]any{"urgent": "yes"}},
		{"date format", map[string]any{"when": "yesterday"}},
		{"datetime format", map[string]any{"at": "noon"}},
		{"enum option", map[string]any{"severity": "critical"}},
		{"actor type", map[string]any{"owner": 12}},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "t", CustomFields: tc.fields}); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("CreateTask = %v, want invalid", err)
			}
		})
	}

	t.Run("required without a default", func(t *testing.T) {
		other := seedTaskProject(t, l, scope, "app")
		seedFieldDef(t, l, scope, core.FieldDef{
			ProjectID: other.ID, Key: "team", Label: "Team", Type: core.FieldString, Required: true,
		})
		if _, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: "app", Title: "t"}); !core.IsKind(err, core.KindInvalid) {
			t.Errorf("CreateTask without a required field = %v, want invalid", err)
		}
		if _, err := l.CreateTask(ctx, core.CreateTaskInput{
			ProjectRef: "app", Title: "t", CustomFields: map[string]any{"team": "core"},
		}); err != nil {
			t.Errorf("CreateTask with the required field = %v, want success", err)
		}
	})

	t.Run("clearing a required field", func(t *testing.T) {
		if _, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
			CustomFields: map[string]any{"owner": nil},
		}); !core.IsKind(err, core.KindInvalid) {
			t.Error("clearing a required custom field must be rejected")
		}
	})
}

func TestCreateTaskNeedsAProjectWhenSeveralExist(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "app")

	if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "ambiguous"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("CreateTask with several projects = %v, want invalid", err)
	}
	if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "explicit", ProjectRef: "app"}); err != nil {
		t.Errorf("CreateTask naming a project = %v, want success", err)
	}
}

func TestTaskOperationsRequireAnActorAndScopes(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "guarded"})
	ref := core.TaskRef{ID: task.ID}
	anonymous := context.Background()

	if _, err := l.CreateTask(anonymous, core.CreateTaskInput{Title: "t"}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("CreateTask unauthenticated = %v", err)
	}
	if _, err := l.GetTask(anonymous, ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("GetTask unauthenticated = %v", err)
	}
	if _, err := l.ListTasks(anonymous, core.TaskFilter{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListTasks unauthenticated = %v", err)
	}
	if _, err := l.UpdateTask(anonymous, ref, core.UpdateTaskInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("UpdateTask unauthenticated = %v", err)
	}
	if _, err := l.TransitionTask(anonymous, ref, core.TransitionInput{To: "doing"}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("TransitionTask unauthenticated = %v", err)
	}
	if err := l.DeleteTask(anonymous, ref, core.DeleteTaskInput{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("DeleteTask unauthenticated = %v", err)
	}
	if _, err := l.RestoreTask(anonymous, ref); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("RestoreTask unauthenticated = %v", err)
	}
	if _, err := l.TaskTree(anonymous, ref, 0); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("TaskTree unauthenticated = %v", err)
	}

	viewer := taskContext(seedTaskActor(t, l, tenantScope(t, l), "viewer", core.RoleViewer))
	if err := l.DeleteTask(viewer, ref, core.DeleteTaskInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a viewer deleting a task = %v, want forbidden", err)
	}
	if _, err := l.GetTask(viewer, ref); err != nil {
		t.Errorf("a viewer reading a task = %v, want success", err)
	}
}

// A task belonging to another tenant must read as absent rather than forbidden,
// so no operation confirms that another tenant's record exists.
func TestCrossTenantAccessReportsNotFound(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "private"})

	other := core.Tenant{Key: "other", Name: "Other"}
	if err := l.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		return u.CreateTenant(context.Background(), &other)
	}); err != nil {
		t.Fatalf("creating the second tenant: %v", err)
	}
	intruder := taskContext(&core.Actor{
		ID: "intruder", TenantID: other.ID, Kind: core.ActorUser, Scopes: []core.Scope{core.ScopeAll},
	})

	ref := core.TaskRef{ID: task.ID}
	if _, err := l.GetTask(intruder, ref); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant GetTask = %v, want not found", err)
	}
	if _, err := l.UpdateTask(intruder, ref, core.UpdateTaskInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant UpdateTask = %v, want not found", err)
	}
	if _, err := l.TransitionTask(intruder, ref, core.TransitionInput{To: "doing"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant TransitionTask = %v, want not found", err)
	}
	if err := l.DeleteTask(intruder, ref, core.DeleteTaskInput{}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant DeleteTask = %v, want not found", err)
	}
	if _, err := l.AddComment(intruder, ref, "hello"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant AddComment = %v, want not found", err)
	}
	page, err := l.ListTasks(intruder, core.TaskFilter{})
	if err != nil {
		t.Fatalf("cross-tenant ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("another tenant's listing returned %d tasks", len(page.Tasks))
	}
}

// tenantScope reads back the scope of the tenant the fixture created.
func tenantScope(t *testing.T, l *Local) core.TenantScope {
	t.Helper()
	var scope core.TenantScope
	if err := l.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByKey(context.Background(), "acme")
		if err != nil {
			return err
		}
		scope = core.TenantScope{TenantID: tenant.ID}
		return nil
	}); err != nil {
		t.Fatalf("reading the tenant: %v", err)
	}
	return scope
}

func TestTaskSortValueRendersEverySortField(t *testing.T) {
	due := time.Date(2030, 7, 8, 9, 10, 11, 0, time.UTC)
	created := time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := time.Date(2029, 2, 2, 0, 0, 0, 0, time.UTC)
	task := core.Task{
		Title: "title", Seq: 42, Priority: core.PriorityHigh,
		CreatedAt: created, UpdatedAt: updated, DueAt: &due,
	}

	cases := []struct {
		sort string
		want string
	}{
		{core.SortUrgency, "2"},
		{core.SortCreatedAt, "2029-01-01T00:00:00.000000000Z"},
		{core.SortUpdatedAt, "2029-02-02T00:00:00.000000000Z"},
		{core.SortDueAt, "2030-07-08T09:10:11.000000000Z"},
		{core.SortPriority, "2"},
		{core.SortSeq, "42"},
		{core.SortTitle, "title"},
	}
	for _, tc := range cases {
		t.Run(tc.sort, func(t *testing.T) {
			if got := taskSortValue(tc.sort, task); got != tc.want {
				t.Errorf("taskSortValue = %q, want %q", got, tc.want)
			}
		})
	}
	if got := taskSortValue(core.SortDueAt, core.Task{}); got != sqlb.NoDueSentinel {
		t.Errorf("a task with no due date rendered %q, want the no-due sentinel", got)
	}

	if got := taskSortValue2(core.SortUrgency, task); got != "2030-07-08T09:10:11.000000000Z" {
		t.Errorf("taskSortValue2(urgency) = %q, want the due date", got)
	}
	if got := taskSortValue2(core.SortUrgency, core.Task{}); got != sqlb.NoDueSentinel {
		t.Errorf("taskSortValue2(urgency) with no due date = %q, want the sentinel", got)
	}
	if got := taskSortValue2(core.SortCreatedAt, task); got != "" {
		t.Errorf("taskSortValue2 outside urgency = %q, want empty", got)
	}
}

func TestPagingWorksForEverySortField(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	due := time.Date(2032, 3, 4, 5, 6, 7, 0, time.UTC)
	for i := range 4 {
		mustCreateTask(t, l, ctx, core.CreateTaskInput{
			Title: fmt.Sprintf("row-%d", i), Priority: core.Priority(i%5 + 1), DueAt: &due,
		})
	}

	for _, sort := range core.TaskSortFields {
		t.Run(sort, func(t *testing.T) {
			seen := map[string]int{}
			cursor := ""
			for range 5 {
				page, err := l.ListTasks(ctx, core.TaskFilter{Page: core.Page{
					Limit: 2, Cursor: cursor, Sort: sort, Direction: core.Ascending,
				}})
				if err != nil {
					t.Fatalf("ListTasks sorted by %s: %v", sort, err)
				}
				for _, task := range page.Tasks {
					seen[task.ID]++
				}
				if page.NextCursor == "" {
					break
				}
				cursor = page.NextCursor
			}
			if len(seen) != 4 {
				t.Errorf("paging by %s saw %d tasks, want 4", sort, len(seen))
			}
		})
	}
}

func TestFieldCoercionAcceptsEveryNumericForm(t *testing.T) {
	intDef := core.FieldDef{Key: "n", Type: core.FieldInt}
	floatDef := core.FieldDef{Key: "f", Type: core.FieldFloat}
	dateDef := core.FieldDef{Key: "d", Type: core.FieldDate}
	when := time.Date(2030, 4, 5, 6, 7, 8, 0, time.UTC)

	cases := []struct {
		name string
		def  core.FieldDef
		in   any
		want any
	}{
		{"int from int", intDef, 7, int64(7)},
		{"int from int32", intDef, int32(7), int64(7)},
		{"int from int64", intDef, int64(7), int64(7)},
		{"int from whole float", intDef, float64(7), int64(7)},
		{"int from json number", intDef, json.Number("7"), int64(7)},
		{"float from int", floatDef, 7, float64(7)},
		{"float from int64", floatDef, int64(7), float64(7)},
		{"float from float", floatDef, 7.5, 7.5},
		{"float from json number", floatDef, json.Number("7.5"), 7.5},
		{"date from time", dateDef, when, "2030-04-05"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := coerceFieldValue(tc.def, tc.in)
			if err != nil {
				t.Fatalf("coerceFieldValue: %v", err)
			}
			if got != tc.want {
				t.Errorf("coerceFieldValue = %#v, want %#v", got, tc.want)
			}
		})
	}

	bad := []struct {
		name string
		def  core.FieldDef
		in   any
	}{
		{"int from fraction", intDef, 7.5},
		{"int from bad json number", intDef, json.Number("x")},
		{"int from bool", intDef, true},
		{"float from bool", floatDef, true},
		{"float from bad json number", floatDef, json.Number("x")},
		{"date from number", dateDef, 7},
		{"unsupported type", core.FieldDef{Key: "u", Type: "colour"}, "red"},
	}
	for _, tc := range bad {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := coerceFieldValue(tc.def, tc.in); !core.IsKind(err, core.KindInvalid) {
				t.Errorf("coerceFieldValue = %v, want invalid", err)
			}
		})
	}
}

func TestParentReferencesAreResolvedThroughAncestors(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	root := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "root"})
	middle := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "middle", ParentRef: root.Ref})
	leaf := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "leaf", ParentRef: middle.Ref})

	deep := leaf.Ref
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: root.ID}, core.UpdateTaskInput{ParentRef: &deep}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("making a task a child of its own grandchild = %v, want invalid", err)
	}

	malformed := "not a ref!"
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: leaf.ID}, core.UpdateTaskInput{ParentRef: &malformed}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a malformed parent reference = %v, want invalid", err)
	}
	if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "t", ParentRef: malformed}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("creating under a malformed parent reference = %v, want invalid", err)
	}
	if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "t", DependsOn: []string{malformed}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("depending on a malformed reference = %v, want invalid", err)
	}
}

func TestReplacingLabelsIgnoresBlankNames(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t", Tags: []string{"keep", "drop"}})

	tags := []string{"keep", "  ", ""}
	got, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Tags: &tags})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	if strings.Join(got.Tags, ",") != "keep" {
		t.Errorf("tags = %v, want only the named one", got.Tags)
	}
}

func TestCreateTaskInAProjectWithoutTasksReportsMissingProject(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := taskContext(actor)

	if _, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "orphan"}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("CreateTask with no project at all = %v, want not found", err)
	}
}

func TestDeleteRefusesSubtasksUnlessCascadeIsAsked(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: parent.ID})
	grand := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "grandchild", ParentRef: child.ID})

	err := l.DeleteTask(ctx, core.TaskRef{ID: parent.ID}, core.DeleteTaskInput{})
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("delete without cascade = %v, want precondition", err)
	}
	if _, err := l.GetTask(ctx, core.TaskRef{ID: grand.ID}); err != nil {
		t.Fatalf("refused delete disturbed the subtree: %v", err)
	}
}

func TestCascadeDeleteRemovesTheWholeSubtree(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	child := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: parent.ID})
	grand := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "grandchild", ParentRef: child.ID})
	bystander := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "bystander"})

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.DeleteTask(ctx, core.TaskRef{ID: parent.ID}, core.DeleteTaskInput{Cascade: true}); err != nil {
		t.Fatalf("cascade delete: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+3 || afterAudits != beforeAudits+3 {
		t.Errorf("cascade wrote %d events and %d audit entries, want three of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	for _, id := range []string{parent.ID, child.ID, grand.ID} {
		if _, err := l.GetTask(ctx, core.TaskRef{ID: id}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("task %s after cascade = %v, want not found", id, err)
		}
	}
	if _, err := l.GetTask(ctx, core.TaskRef{ID: bystander.ID}); err != nil {
		t.Errorf("cascade reached a task outside the subtree: %v", err)
	}
}

// seedParentCycle writes A as the parent of B and B as the parent of A,
// straight into the store, which is the state a snapshot import or a sync used
// to be able to leave behind.
func seedParentCycle(t *testing.T, l *Local, scope core.TenantScope, a, b *core.Task) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		first, err := tx.GetTask(ctx, core.TaskRef{ID: a.ID})
		if err != nil {
			return err
		}
		second, err := tx.GetTask(ctx, core.TaskRef{ID: b.ID})
		if err != nil {
			return err
		}
		first.ParentID = second.ID
		second.ParentID = first.ID
		if err := tx.UpdateTask(ctx, first); err != nil {
			return err
		}
		return tx.UpdateTask(ctx, second)
	}); err != nil {
		t.Fatalf("seeding a parent cycle: %v", err)
	}
}

// A cycle in parent links used to make TaskTree recurse without end, which is
// a stack overflow: a fatal runtime error no recover in the http layer can
// catch. The walk must terminate instead.
func TestTaskTreeTerminatesOnAParentCycle(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	project := seedTaskProject(t, l, scope, "cyc")
	ctx := taskContext(actor)

	first, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "first"})
	if err != nil {
		t.Fatalf("first task: %v", err)
	}
	second, err := l.CreateTask(ctx, core.CreateTaskInput{
		ProjectRef: project.Key, Title: "second", ParentRef: first.Ref,
	})
	if err != nil {
		t.Fatalf("second task: %v", err)
	}
	seedParentCycle(t, l, scope, first, second)

	tree, err := l.TaskTree(ctx, core.TaskRef{ID: first.ID}, 0)
	if err != nil {
		t.Fatalf("TaskTree over a cycle: %v", err)
	}
	if len(tree) != 2 {
		t.Fatalf("tree has %d tasks, want the root and its one child visited once each", len(tree))
	}
	seen := map[string]bool{}
	for _, task := range tree {
		if seen[task.ID] {
			t.Fatalf("task %q appears twice in the tree", task.Ref)
		}
		seen[task.ID] = true
	}
}

// Reparenting into a graph that already holds a cycle used to spin inside an
// open write transaction, holding the write lock against every other writer.
func TestUpdateTaskRefusesToWalkAParentCycle(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	project := seedTaskProject(t, l, scope, "spin")
	ctx := taskContext(actor)

	first, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "first"})
	if err != nil {
		t.Fatalf("first task: %v", err)
	}
	second, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "second"})
	if err != nil {
		t.Fatalf("second task: %v", err)
	}
	loose, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "loose"})
	if err != nil {
		t.Fatalf("loose task: %v", err)
	}
	seedParentCycle(t, l, scope, first, second)

	ref := first.Ref
	_, err = l.UpdateTask(ctx, core.TaskRef{ID: loose.ID}, core.UpdateTaskInput{ParentRef: &ref})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("UpdateTask = %v, want a refusal naming the cycle", err)
	}
}

// Subscribers see only an event's payload, so an update event that does not
// name the fields it changed makes every edit read the same in tix watch.
func TestUpdateTaskEventNamesTheChangedFields(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	project := seedTaskProject(t, l, scope, "fld")
	ctx := taskContext(actor)

	task, err := l.CreateTask(ctx, core.CreateTaskInput{ProjectRef: project.Key, Title: "first"})
	if err != nil {
		t.Fatalf("task: %v", err)
	}
	title := "renamed"
	priority := core.PriorityHigh
	if _, err := l.UpdateTask(ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{
		Title: &title, Priority: &priority,
	}); err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}

	var payload map[string]any
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(context.Background(), 0, 100)
		if err != nil {
			return err
		}
		for _, e := range events {
			if e.Type == core.EventTaskUpdated {
				payload = e.Payload
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("reading events: %v", err)
	}
	if payload == nil {
		t.Fatal("the update emitted no task.updated event")
	}
	raw, ok := payload["fields"].([]any)
	if !ok {
		t.Fatalf("payload fields = %#v, want the changed field names", payload["fields"])
	}
	got := make([]string, 0, len(raw))
	for _, item := range raw {
		got = append(got, fmt.Sprint(item))
	}
	want := []string{"title", "priority"}
	if len(got) != len(want) {
		t.Fatalf("fields = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("fields = %v, want %v", got, want)
		}
	}
}

// TestAssigneeAcceptsAHandleWithoutLeakingSQL pins the rough edge the skill
// refresh found: a handle reached the database as though it were an identifier
// and surfaced "FOREIGN KEY constraint failed" verbatim. An identifier-shaped
// value still passes through unresolved, because an actor from another tenant
// is deliberately assignable and never resolves locally.
func TestAssigneeAcceptsAHandleWithoutLeakingSQL(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "assign me"})
	ref := core.TaskRef{ID: task.ID}

	handle := actor.Handle
	updated, err := l.UpdateTask(ctx, ref, core.UpdateTaskInput{AssigneeActorID: &handle})
	if err != nil {
		t.Fatalf("assigning by handle %q: %v", handle, err)
	}
	if updated.AssigneeActorID != actor.ID {
		t.Errorf("assignee = %q, want the actor id %q", updated.AssigneeActorID, actor.ID)
	}

	unknown := "nosuchperson"
	_, err = l.UpdateTask(ctx, ref, core.UpdateTaskInput{AssigneeActorID: &unknown})
	if err == nil {
		t.Fatal("an unknown handle was accepted")
	}
	if core.KindOf(err) != core.KindNotFound {
		t.Errorf("unknown handle gave %v, want a not-found", core.KindOf(err))
	}
	if strings.Contains(err.Error(), "FOREIGN KEY") || strings.Contains(err.Error(), "constraint") {
		t.Errorf("the error leaks the schema: %v", err)
	}
}

// TestListTasksRejectsAnUnknownAssigneeInsteadOfAnsweringEmpty pins the defect
// the published skill demonstrated: `task ls --assignee alice` answered with an
// empty list and a zero exit status, which reads as "alice has no tasks" rather
// than "that is a handle, and the filter wanted an identifier". An empty
// success is the one outcome that cannot be told apart from a correct answer,
// so the unknown name must fail.
func TestListTasksRejectsAnUnknownAssigneeInsteadOfAnsweringEmpty(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "assigned", AssigneeActorID: actor.ID})

	page, err := l.ListTasks(ctx, core.TaskFilter{AssigneeIDs: []string{"nosuchperson"}})
	if err == nil {
		t.Fatalf("an unknown assignee returned %d tasks and no error, want a not-found",
			len(page.Tasks))
	}
	if core.KindOf(err) != core.KindNotFound {
		t.Errorf("unknown assignee gave %v, want a not-found", core.KindOf(err))
	}
	if !strings.Contains(err.Error(), "nosuchperson") {
		t.Errorf("the error does not name what was not found: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("a failed listing also returned %d tasks", len(page.Tasks))
	}
}

// TestListTasksAcceptsAnAssigneeHandle covers the working half of the same
// rule, on both the inclusion and the exclusion side, because the filter
// language offers "-assignee:" too and a term resolved on one side only would
// answer a different question than the one asked.
func TestListTasksAcceptsAnAssigneeHandle(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "mine", AssigneeActorID: actor.ID})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "nobody's"})

	cases := []struct {
		name string
		f    core.TaskFilter
		want []string
	}{
		{"handle", core.TaskFilter{AssigneeIDs: []string{actor.Handle}}, []string{"mine"}},
		{"identifier", core.TaskFilter{AssigneeIDs: []string{actor.ID}}, []string{"mine"}},
		{"handle in upper case", core.TaskFilter{AssigneeIDs: []string{strings.ToUpper(actor.Handle)}},
			[]string{"mine"}},
		{"excluded by handle", core.TaskFilter{Exclude: core.TaskExclude{AssigneeIDs: []string{actor.Handle}}},
			[]string{"nobody's"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			got := taskTitles(page.Tasks)
			if len(got) != len(tc.want) || (len(got) == 1 && got[0] != tc.want[0]) {
				t.Errorf("tasks = %v, want %v", got, tc.want)
			}
		})
	}

	if _, err := l.ListTasks(ctx, core.TaskFilter{
		Exclude: core.TaskExclude{AssigneeIDs: []string{"nosuchperson"}},
	}); core.KindOf(err) != core.KindNotFound {
		t.Errorf("an unknown excluded assignee gave %v, want a not-found", core.KindOf(err))
	}
}

// TestCreateTaskResolvesTheAssigneeHandle pins the creation half. An
// unresolved handle reached the foreign key and surfaced the constraint error
// verbatim, which is a schema leak rather than an answer; and had the column
// carried no constraint it would have stored a task assigned to nobody.
func TestCreateTaskResolvesTheAssigneeHandle(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)

	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		Title: "by handle", AssigneeActorID: actor.Handle})
	if task.AssigneeActorID != actor.ID {
		t.Errorf("assignee = %q, want the actor id %q", task.AssigneeActorID, actor.ID)
	}

	_, err := l.CreateTask(ctx, core.CreateTaskInput{Title: "nope", AssigneeActorID: "nosuchperson"})
	if err == nil {
		t.Fatal("an unknown handle was accepted")
	}
	if core.KindOf(err) != core.KindNotFound {
		t.Errorf("unknown handle gave %v, want a not-found", core.KindOf(err))
	}
	if strings.Contains(err.Error(), "FOREIGN KEY") || strings.Contains(err.Error(), "constraint") {
		t.Errorf("the error leaks the schema: %v", err)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	for _, got := range page.Tasks {
		if got.Title == "nope" {
			t.Errorf("the refused task was written anyway: %+v", got)
		}
	}
}

// TestAssigneeIdentifierShapeDecidesResolution documents the disambiguation
// rule, because it is the part a reader would otherwise have to infer from two
// call sites. Shape alone decides, and an identifier-shaped value is never
// looked up, which is what keeps an actor of another tenant assignable and
// what keeps a filter from costing one query per value.
func TestAssigneeIdentifierShapeDecidesResolution(t *testing.T) {
	cases := []struct {
		ref  string
		want bool
	}{
		{"01ARZ3NDEKTSV4RRFFQ69G5FAV", true},
		{"alice", false},
		{"", false},
		{"alice@example.com", false},
		{"01ARZ3NDEKTSV4RRFFQ69G5FA", false},
		{"01ARZ3NDEKTSV4RRFFQ69G5FAVX", false},
		{"01arz3ndektsv4rrffq69g5fav", true},
		{"01ARZ3NDEKTSV4RRFFQ69G5FAU", false},
	}
	for _, tc := range cases {
		if got := assigneeIsIdentifier(tc.ref); got != tc.want {
			t.Errorf("assigneeIsIdentifier(%q) = %v, want %v", tc.ref, got, tc.want)
		}
	}
}

// seedTaskProjectWith seeds a project whose workflow is not the shared one, so
// a test can ask what a status defined by one workflow and not another does.
func seedTaskProjectWith(t *testing.T, l *Local, scope core.TenantScope, key string, def core.WorkflowDefinition) *core.Project {
	t.Helper()
	ctx := context.Background()
	wf := core.Workflow{Key: "wf-" + key, Name: "Workflow " + key, Definition: def}
	project := core.Project{Key: key, Name: strings.ToUpper(key)}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		project.WorkflowID = wf.ID
		return tx.CreateProject(ctx, &project)
	}); err != nil {
		t.Fatalf("seeding project %q: %v", key, err)
	}
	return &project
}

// shippingWorkflow defines states the shared task workflow does not, so a
// status can be real for the tenant and undefined for one project at once.
func shippingWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "backlog",
		States: []core.State{
			{Key: "backlog", Label: "Backlog", Category: core.CategoryTodo},
			{Key: "shipped", Label: "Shipped", Terminal: true, Category: core.CategoryDone},
		},
		Transitions: []core.Transition{{From: "backlog", To: "shipped"}},
	}
}

// TestListTasksRejectsAnUnknownProjectInsteadOfAnsweringEmpty pins the same
// defect the assignee filter had, in the term a reader is most likely to get
// wrong: `task ls -p nosuchproject` printed an empty document and exited zero,
// which reads as "that project has no work" rather than "there is no such
// project". A project key names a row that exists or does not, so there is no
// legitimate query the refusal takes away.
func TestListTasksRejectsAnUnknownProjectInsteadOfAnsweringEmpty(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "real work"})

	cases := []struct {
		name string
		f    core.TaskFilter
	}{
		{"key", core.TaskFilter{ProjectKeys: []string{"nosuchproject"}}},
		{"excluded key", core.TaskFilter{Exclude: core.TaskExclude{ProjectKeys: []string{"nosuchproject"}}}},
		{"identifier", core.TaskFilter{ProjectIDs: []string{"01ARZ3NDEKTSV4RRFFQ69G5FAV"}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err == nil {
				t.Fatalf("an unknown project returned %d tasks and no error, want a not-found", len(page.Tasks))
			}
			if core.KindOf(err) != core.KindNotFound {
				t.Errorf("unknown project gave %v, want a not-found", core.KindOf(err))
			}
			if len(page.Tasks) != 0 {
				t.Errorf("a failed listing also returned %d tasks", len(page.Tasks))
			}
		})
	}
}

// TestListTasksAcceptsAProjectKeyInAnyCase covers the other half of the same
// term. The store matches a key exactly, so an upper-case key selected nothing
// while naming a project that plainly exists.
func TestListTasksAcceptsAProjectKeyInAnyCase(t *testing.T) {
	l, ctx, _, _, project := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "mine"})

	page, err := l.ListTasks(ctx, core.TaskFilter{ProjectKeys: []string{strings.ToUpper(project.Key)}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("an upper-case project key returned %d tasks, want 1", len(page.Tasks))
	}
}

// TestListTasksRejectsAStatusNoWorkflowDefines pins the status half of the
// defect. The vocabulary is every state of every workflow the tenant has, so
// what is refused here is a word that cannot describe any task at all.
func TestListTasksRejectsAStatusNoWorkflowDefines(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "real work"})

	cases := []struct {
		name string
		f    core.TaskFilter
	}{
		{"included", core.TaskFilter{Statuses: []string{"nosuchstatus"}}},
		{"excluded", core.TaskFilter{Exclude: core.TaskExclude{Statuses: []string{"nosuchstatus"}}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err == nil {
				t.Fatalf("an undefined status returned %d tasks and no error, want a not-found", len(page.Tasks))
			}
			if core.KindOf(err) != core.KindNotFound {
				t.Errorf("undefined status gave %v, want a not-found", core.KindOf(err))
			}
			if !strings.Contains(err.Error(), "nosuchstatus") {
				t.Errorf("the error does not name what was not found: %v", err)
			}
			if !strings.Contains(err.Error(), "todo") {
				t.Errorf("the error does not offer the vocabulary it checked against: %v", err)
			}
		})
	}
}

// TestListTasksAllowsAStatusOnlyOneWorkflowDefines is the regression the
// status rule exists to avoid creating. A tenant's workflows need not agree,
// so a listing spanning projects, or scoped to a project whose workflow lacks
// the state, may legitimately name a status defined elsewhere. That query must
// answer, with an empty page where there is nothing, rather than be refused:
// here the status is a real thing that this scope does not reach, which is not
// the same as a word that names nothing.
func TestListTasksAllowsAStatusOnlyOneWorkflowDefines(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	ship := seedTaskProjectWith(t, l, scope, "ship", shippingWorkflow())
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: infra.Key, Title: "in infra"})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: ship.Key, Title: "in ship"})

	cases := []struct {
		name string
		f    core.TaskFilter
		want int
	}{
		{"tenant wide", core.TaskFilter{Statuses: []string{"backlog"}}, 1},
		{"scoped to the project that lacks the state",
			core.TaskFilter{ProjectKeys: []string{infra.Key}, Statuses: []string{"backlog"}}, 0},
		{"spanning both projects",
			core.TaskFilter{ProjectKeys: []string{infra.Key, ship.Key}, Statuses: []string{"todo", "backlog"}}, 2},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err != nil {
				t.Fatalf("a status defined by one workflow must not fail the listing: %v", err)
			}
			if len(page.Tasks) != tc.want {
				t.Errorf("got %d tasks, want %d", len(page.Tasks), tc.want)
			}
		})
	}
}

// TestListTasksAcceptsAStatusInAnyCase mirrors the project key: the store
// matches the state exactly, so an upper-case status selected nothing.
func TestListTasksAcceptsAStatusInAnyCase(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "mine"})

	page, err := l.ListTasks(ctx, core.TaskFilter{Statuses: []string{"TODO"}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Fatalf("an upper-case status returned %d tasks, want 1", len(page.Tasks))
	}
}

// TestListTasksAnswersAnUnknownTagWithAnEmptyPage records the one term of the
// family that is deliberately not an error. A tag is free-form and exists only
// by being applied, so "unknown tag" and "tag nothing carries" are the same
// state: refusing the first refuses the second, removing the last task from a
// tag would turn a working filter into a failure, and excluding a tag is a
// question whose whole point is that nothing carries it.
func TestListTasksAnswersAnUnknownTagWithAnEmptyPage(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "untagged"})

	page, err := l.ListTasks(ctx, core.TaskFilter{Tags: []string{"nosuchtag"}})
	if err != nil {
		t.Fatalf("an unapplied tag must not fail a listing: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("got %d tasks, want none", len(page.Tasks))
	}

	page, err = l.ListTasks(ctx, core.TaskFilter{Exclude: core.TaskExclude{Tags: []string{"nosuchtag"}}})
	if err != nil {
		t.Fatalf("excluding an unapplied tag must not fail a listing: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Errorf("excluding a tag nothing carries removed %d tasks", 1-len(page.Tasks))
	}
}

// TestListTasksResolvesTheParentTerm pins the term where silence was most
// convincing: the parent column holds an identifier, so a parent written in
// the reference form every surface displays selected nothing while looking
// exactly right.
func TestListTasksResolvesTheParentTerm(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	parent := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "parent"})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "child", ParentRef: parent.Ref})

	page, err := l.ListTasks(ctx, core.TaskFilter{ParentID: parent.Ref})
	if err != nil {
		t.Fatalf("a parent named by reference must resolve: %v", err)
	}
	if len(page.Tasks) != 1 || page.Tasks[0].Title != "child" {
		t.Fatalf("filtering by parent reference returned %d tasks, want the one child", len(page.Tasks))
	}

	if _, err := l.ListTasks(ctx, core.TaskFilter{ParentID: "infra-999"}); err == nil {
		t.Fatal("an unknown parent answered successfully, want a not-found")
	} else if core.KindOf(err) != core.KindNotFound {
		t.Errorf("unknown parent gave %v, want a not-found", core.KindOf(err))
	}
}

// TestListTasksResolvesEveryActorTerm extends the assignee rule to the other
// two actor-shaped terms. All three reach storage as identifiers, so a handle
// in any of them selected nothing and said so with a zero exit status.
func TestListTasksResolvesEveryActorTerm(t *testing.T) {
	l, ctx, _, actor, _ := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "mine"})

	cases := []struct {
		name string
		f    core.TaskFilter
		want int
	}{
		{"creator by handle", core.TaskFilter{CreatorIDs: []string{actor.Handle}}, 1},
		{"creator excluded by handle",
			core.TaskFilter{Exclude: core.TaskExclude{CreatorIDs: []string{actor.Handle}}}, 0},
		{"claimant by handle", core.TaskFilter{ClaimedBy: []string{actor.Handle}}, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			page, err := l.ListTasks(ctx, tc.f)
			if err != nil {
				t.Fatalf("ListTasks: %v", err)
			}
			if len(page.Tasks) != tc.want {
				t.Errorf("got %d tasks, want %d", len(page.Tasks), tc.want)
			}
		})
	}

	unknown := []core.TaskFilter{
		{CreatorIDs: []string{"nosuchperson"}},
		{ClaimedBy: []string{"nosuchperson"}},
		{Exclude: core.TaskExclude{ClaimedBy: []string{"nosuchperson"}}},
	}
	for _, f := range unknown {
		if _, err := l.ListTasks(ctx, f); err == nil || core.KindOf(err) != core.KindNotFound {
			t.Errorf("an unknown actor term gave %v, want a not-found", err)
		}
	}
}

// TestListTasksResolvesARepeatedReferenceOnce exercises the cache that keeps a
// listing's cost proportional to the distinct references its filter names: the
// same project and the same handle appear on both sides of the filter, and an
// identifier passes through with no lookup at all.
func TestListTasksResolvesARepeatedReferenceOnce(t *testing.T) {
	l, ctx, _, actor, project := newTaskFixture(t)
	mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "mine", AssigneeActorID: actor.ID})

	page, err := l.ListTasks(ctx, core.TaskFilter{
		ProjectKeys: []string{project.Key, project.Key},
		AssigneeIDs: []string{actor.Handle, actor.ID},
		Statuses:    []string{"todo", "TODO"},
		Exclude:     core.TaskExclude{ProjectKeys: []string{project.Key}, AssigneeIDs: []string{actor.Handle}},
	})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("an exclusion of the included project returned %d tasks", len(page.Tasks))
	}
}
