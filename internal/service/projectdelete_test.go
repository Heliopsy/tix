// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// reviewWorkflow has a state the built-in workflow lacks, in a category the
// built-in workflow does have.
func reviewWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "open",
		States: []core.State{
			{Key: "open", Label: "Open", Category: core.CategoryTodo},
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "review", Label: "Review", Category: core.CategoryInProgress},
			{Key: "parked", Label: "Parked", Category: core.CategoryWaiting},
			{Key: "shipped", Label: "Shipped", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{{From: "open", To: "review"}, {From: "review", To: "shipped"}},
	}
}

// deletionFixture is a tenant with a source project and a target project.
type deletionFixture struct {
	l      *Local
	scope  core.TenantScope
	ctx    context.Context
	source *core.Project
	target *core.Project
}

func newDeletionFixture(t *testing.T) deletionFixture {
	t.Helper()
	l, scope, ctx := withDefaults(t)
	seedWorkflow(t, l, scope, "review", reviewWorkflow(), false)
	source, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "old", Name: "Old", WorkflowKey: "review"})
	if err != nil {
		t.Fatalf("CreateProject old: %v", err)
	}
	target, err := l.CreateProject(ctx, core.CreateProjectInput{Key: "ops", Name: "Ops"})
	if err != nil {
		t.Fatalf("CreateProject ops: %v", err)
	}
	return deletionFixture{l: l, scope: scope, ctx: ctx, source: source, target: target}
}

func (f deletionFixture) task(t *testing.T, project, title string) *core.Task {
	t.Helper()
	task, err := f.l.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: project, Title: title})
	if err != nil {
		t.Fatalf("CreateTask %q: %v", title, err)
	}
	return task
}

func (f deletionFixture) events(t *testing.T) []core.Event {
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

func TestAProjectHoldingTasksIsNotDeletedWithoutAChoice(t *testing.T) {
	f := newDeletionFixture(t)
	f.task(t, "old", "one")
	f.task(t, "old", "two")
	gone := f.task(t, "old", "three")
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: gone.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	events, audits := countRows(t, f.l, f.scope)

	_, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{})
	if !core.IsKind(err, core.KindPrecondition) {
		t.Fatalf("deleting a project holding tasks without a choice = %v, want a precondition failure", err)
	}
	var domain *core.Error
	if !errors.As(err, &domain) || domain.Details["tasks"] != 3 {
		t.Errorf("the refusal names %v tasks, want 3 counting the soft deleted one", domain.Details["tasks"])
	}
	if _, err := f.l.GetProject(f.ctx, "old"); err != nil {
		t.Errorf("the refused deletion removed the project: %v", err)
	}
	if e, a := countRows(t, f.l, f.scope); e != events || a != audits {
		t.Errorf("the refused deletion wrote %d events and %d audit entries", e-events, a-audits)
	}
}

func TestContradictoryOrImpossibleChoicesAreRefused(t *testing.T) {
	f := newDeletionFixture(t)
	f.task(t, "old", "one")
	archived, err := f.l.CreateProject(f.ctx, core.CreateProjectInput{Key: "attic", Name: "Attic"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := f.l.ArchiveProject(f.ctx, archived.Key); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	cases := []struct {
		name string
		in   core.DeleteProjectInput
		want core.Kind
	}{
		{"both choices", core.DeleteProjectInput{MoveTasksTo: "ops", DeleteTasks: true}, core.KindInvalid},
		{"itself as target", core.DeleteProjectInput{MoveTasksTo: "OLD"}, core.KindInvalid},
		{"archived target", core.DeleteProjectInput{MoveTasksTo: "attic"}, core.KindPrecondition},
		{"unknown target", core.DeleteProjectInput{MoveTasksTo: "nowhere"}, core.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.l.DeleteProject(f.ctx, "old", tc.in); !core.IsKind(err, tc.want) {
				t.Errorf("DeleteProject = %v, want %s", err, tc.want)
			}
			if _, err := f.l.PlanProjectDeletion(f.ctx, "old", tc.in); !core.IsKind(err, tc.want) {
				t.Errorf("PlanProjectDeletion = %v, want %s", err, tc.want)
			}
		})
	}
	if _, err := f.l.GetProject(f.ctx, "old"); err != nil {
		t.Errorf("a refused deletion removed the project: %v", err)
	}
}

func TestDeletingTheTasksWithTheProjectRecordsEachOne(t *testing.T) {
	f := newDeletionFixture(t)
	one := f.task(t, "old", "one")
	two := f.task(t, "old", "two")
	binned := f.task(t, "old", "binned")
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: binned.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	before := len(f.events(t))
	_, audits := countRows(t, f.l, f.scope)

	done, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{DeleteTasks: true})
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if done.Tasks != 2 || done.Deleted != 1 || !done.DeleteTasks {
		t.Errorf("the result reports %+v", done)
	}
	for _, id := range []string{one.ID, two.ID, binned.ID} {
		if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
			_, err := tx.GetTask(context.Background(), core.TaskRef{ID: id})
			return err
		}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("task %s survived its project: %v", id, err)
		}
	}
	written := f.events(t)[before:]
	var deleted []string
	projectEvents := 0
	for _, e := range written {
		switch e.Type {
		case core.EventTaskDeleted:
			if e.Payload["hard"] != true {
				t.Errorf("task deletion event %v is not marked hard", e.Payload)
			}
			deleted = append(deleted, e.SubjectID)
		case eventProjectDeleted:
			projectEvents++
		}
	}
	slices.Sort(deleted)
	want := []string{one.ID, two.ID, binned.ID}
	slices.Sort(want)
	if !slices.Equal(deleted, want) || projectEvents != 1 {
		t.Errorf("events: tasks deleted %v, want %v; project events %d, want 1", deleted, want, projectEvents)
	}
	if _, a := countRows(t, f.l, f.scope); a != audits+4 {
		t.Errorf("wrote %d audit entries, want 4", a-audits)
	}
}

func TestMovingTheTasksRenumbersThemAndKeepsWhatHangsOffThem(t *testing.T) {
	f := newDeletionFixture(t)
	for range 4 {
		f.task(t, "ops", "already there")
	}
	third, err := f.l.CreateProject(f.ctx, core.CreateProjectInput{Key: "lab", Name: "Lab"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	upstream := f.task(t, third.Key, "upstream")
	first := f.task(t, "old", "first")
	second := f.task(t, "old", "second")
	ref := core.TaskRef{ID: first.ID}
	if _, err := f.l.AddComment(f.ctx, ref, "keep me"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if err := f.l.AddTag(f.ctx, ref, "urgent"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	if err := f.l.StarTask(f.ctx, ref); err != nil {
		t.Fatalf("StarTask: %v", err)
	}
	if err := f.l.AddDependency(f.ctx, ref, core.TaskRef{ID: upstream.ID}); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	before := len(f.events(t))

	done, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{MoveTasksTo: "OPS"})
	if err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if done.MoveTo != "ops" || done.Tasks != 2 {
		t.Errorf("the result reports %+v", done)
	}
	if _, err := f.l.GetProject(f.ctx, "old"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the project survived: %v", err)
	}
	a, err := f.l.GetTask(f.ctx, core.TaskRef{ID: first.ID})
	if err != nil {
		t.Fatalf("GetTask first: %v", err)
	}
	b, err := f.l.GetTask(f.ctx, core.TaskRef{ID: second.ID})
	if err != nil {
		t.Fatalf("GetTask second: %v", err)
	}
	if a.Ref != "ops-5" || b.Ref != "ops-6" {
		t.Errorf("refs = %s, %s, want ops-5, ops-6", a.Ref, b.Ref)
	}
	if a.Status != "todo" {
		t.Errorf("status %q of the same name in the target became %q", first.Status, a.Status)
	}
	comments, err := f.l.ListComments(f.ctx, core.TaskRef{ID: first.ID})
	if err != nil || len(comments) != 1 {
		t.Errorf("comments after the move = %v, %v", comments, err)
	}
	if !slices.Contains(a.Tags, "urgent") {
		t.Errorf("tags after the move = %v", a.Tags)
	}
	if !a.Starred {
		t.Error("the star did not survive the move")
	}
	deps, err := f.l.ListDependencies(f.ctx, core.TaskRef{ID: first.ID})
	if err != nil || len(deps) != 1 || deps[0].DependsOn != upstream.ID {
		t.Errorf("dependencies after the move = %+v, %v", deps, err)
	}
	moves := 0
	for _, e := range f.events(t)[before:] {
		if e.Type == core.EventTaskUpdated && e.Payload["from_project"] == "old" && e.Payload["project"] == "ops" {
			moves++
		}
	}
	if moves != 2 {
		t.Errorf("recorded %d move events, want 2", moves)
	}
}

func TestAStatusTheTargetLacksMapsByCategoryThenToTheInitialState(t *testing.T) {
	from, to := reviewWorkflow(), BuiltinWorkflow()
	cases := []struct{ status, want string }{
		{"todo", "todo"},
		{"open", "todo"},
		{"review", "doing"},
		{"shipped", "done"},
		{"parked", "todo"},
	}
	for _, tc := range cases {
		if got := mapStatus(from, to, tc.status); got != tc.want {
			t.Errorf("mapStatus(%q) = %q, want %q", tc.status, got, tc.want)
		}
	}
}

func TestThePlanStatesTheMappingAndWritesNothing(t *testing.T) {
	f := newDeletionFixture(t)
	f.task(t, "old", "a")
	f.task(t, "old", "b")
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		task, err := tx.GetTask(context.Background(), core.TaskRef{ProjectKey: "old", Seq: 2})
		if err != nil {
			return err
		}
		task.Status = "review"
		return tx.UpdateTask(context.Background(), task)
	}); err != nil {
		t.Fatalf("putting a task in review: %v", err)
	}
	events, audits := countRows(t, f.l, f.scope)

	plan, err := f.l.PlanProjectDeletion(f.ctx, "old", core.DeleteProjectInput{MoveTasksTo: "ops"})
	if err != nil {
		t.Fatalf("PlanProjectDeletion: %v", err)
	}
	want := []core.StatusMove{{From: "open", To: "todo", Tasks: 1}, {From: "review", To: "doing", Tasks: 1}}
	if !slices.Equal(plan.Statuses, want) || plan.Tasks != 2 || plan.MoveTo != "ops" {
		t.Errorf("plan = %+v, want statuses %+v", plan, want)
	}
	if e, a := countRows(t, f.l, f.scope); e != events || a != audits {
		t.Error("planning wrote history")
	}
	bare, err := f.l.PlanProjectDeletion(f.ctx, "old", core.DeleteProjectInput{})
	if err != nil || bare.Tasks != 2 || len(bare.Statuses) != 0 {
		t.Errorf("a plan without a choice = %+v, %v; want the count and no mapping", bare, err)
	}

	if _, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{MoveTasksTo: "ops"}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	moved, err := f.l.GetTask(f.ctx, core.TaskRef{ProjectKey: "ops", Seq: 2})
	if err != nil || moved.Status != "doing" {
		t.Errorf("the task in review landed as %+v, %v; want doing", moved, err)
	}
}

func TestASoftDeletedTaskStaysRestorableAfterTheMove(t *testing.T) {
	f := newDeletionFixture(t)
	binned := f.task(t, "old", "binned")
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: binned.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	if _, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{MoveTasksTo: "ops"}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	restored, err := f.l.RestoreTask(f.ctx, core.TaskRef{ProjectKey: "ops", Seq: 1})
	if err != nil {
		t.Fatalf("RestoreTask under the new reference: %v", err)
	}
	if restored.ID != binned.ID || restored.DeletedAt != nil {
		t.Errorf("restored %+v", restored)
	}
}

func TestATagScopedToTheDeletedProjectFollowsTheTask(t *testing.T) {
	f := newDeletionFixture(t)
	task := f.task(t, "old", "tagged")
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		tag := &core.Tag{ProjectID: f.source.ID, Name: "imported", Color: "red"}
		if err := tx.PutTag(context.Background(), tag); err != nil {
			return err
		}
		return tx.AttachTag(context.Background(), task.ID, tag.ID)
	}); err != nil {
		t.Fatalf("attaching a project-scoped tag: %v", err)
	}
	if _, err := f.l.DeleteProject(f.ctx, "old", core.DeleteProjectInput{MoveTasksTo: "ops"}); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	moved, err := f.l.GetTask(f.ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if !slices.Contains(moved.Tags, "imported") {
		t.Errorf("the project-scoped tag went with the project: tags = %v", moved.Tags)
	}
}

func TestMovingNeedsTaskWriteOnTheTarget(t *testing.T) {
	f := newDeletionFixture(t)
	f.task(t, "old", "one")
	actor, _ := core.ActorFrom(f.ctx)
	cases := []struct {
		name  string
		actor *core.Actor
	}{
		{"no task scope", &core.Actor{ID: actor.ID, TenantID: actor.TenantID, Kind: core.ActorAgent,
			Scopes: []core.Scope{core.ScopeProjectRead, core.ScopeProjectWrite, core.ScopeTaskRead}}},
		{"confined to the source", &core.Actor{ID: actor.ID, TenantID: actor.TenantID, Kind: core.ActorAgent,
			Scopes: []core.Scope{core.ScopeAll}, ProjectID: f.source.ID}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := core.WithActor(context.Background(), tc.actor)
			in := core.DeleteProjectInput{MoveTasksTo: "ops"}
			if _, err := f.l.DeleteProject(ctx, "old", in); !core.IsKind(err, core.KindForbidden) {
				t.Errorf("DeleteProject = %v, want forbidden", err)
			}
			if _, err := f.l.PlanProjectDeletion(ctx, "old", in); !core.IsKind(err, core.KindForbidden) {
				t.Errorf("PlanProjectDeletion = %v, want forbidden", err)
			}
		})
	}
	if _, err := f.l.GetTask(f.ctx, core.TaskRef{ProjectKey: "old", Seq: 1}); err != nil {
		t.Errorf("a refused move touched the task: %v", err)
	}
}
