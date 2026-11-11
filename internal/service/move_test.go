// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func (f deletionFixture) child(t *testing.T, project, title string, parent *core.Task) *core.Task {
	t.Helper()
	task, err := f.l.CreateTask(f.ctx, core.CreateTaskInput{ProjectRef: project, Title: title, ParentRef: parent.Ref})
	if err != nil {
		t.Fatalf("CreateTask %q: %v", title, err)
	}
	return task
}

func (f deletionFixture) get(t *testing.T, id string) *core.Task {
	t.Helper()
	task, err := f.l.GetTask(f.ctx, core.TaskRef{ID: id})
	if err != nil {
		t.Fatalf("GetTask %s: %v", id, err)
	}
	return task
}

func (f deletionFixture) scopedTag(t *testing.T, project *core.Project, name, color string, task *core.Task) {
	t.Helper()
	if err := f.l.store.Update(context.Background(), f.scope, func(tx store.Tx) error {
		tag := &core.Tag{ProjectID: project.ID, Name: name, Color: color}
		if err := tx.PutTag(context.Background(), tag); err != nil {
			return err
		}
		if task == nil {
			return nil
		}
		return tx.AttachTag(context.Background(), task.ID, tag.ID)
	}); err != nil {
		t.Fatalf("scoping tag %q to %s: %v", name, project.Key, err)
	}
}

func TestMovingATaskRenumbersItMapsItsStatusAndKeepsWhatHangsOffIt(t *testing.T) {
	f := newDeletionFixture(t)
	for range 4 {
		f.task(t, "ops", "already there")
	}
	lab, err := f.l.CreateProject(f.ctx, core.CreateProjectInput{Key: "lab", Name: "Lab"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	upstream := f.task(t, lab.Key, "upstream")
	task := f.task(t, "old", "wanderer")
	ref := core.TaskRef{ID: task.ID}
	if _, err := f.l.TransitionTask(f.ctx, ref, core.TransitionInput{To: "review"}); err != nil {
		t.Fatalf("TransitionTask: %v", err)
	}
	if _, err := f.l.AddComment(f.ctx, ref, "keep me"); err != nil {
		t.Fatalf("AddComment: %v", err)
	}
	if err := f.l.AddTag(f.ctx, ref, "urgent"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	f.scopedTag(t, f.source, "imported", "red", task)
	if err := f.l.StarTask(f.ctx, ref); err != nil {
		t.Fatalf("StarTask: %v", err)
	}
	if err := f.l.AddDependency(f.ctx, ref, core.TaskRef{ID: upstream.ID}); err != nil {
		t.Fatalf("AddDependency: %v", err)
	}
	stay := f.task(t, "old", "stays")
	current := f.get(t, task.ID)
	events := len(f.events(t))

	moved, err := f.l.MoveTask(f.ctx, core.TaskRef{ProjectKey: "old", Seq: 1},
		core.MoveTaskInput{ProjectRef: "OPS", Version: current.Version})
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if moved.ID != task.ID || moved.Ref != "ops-5" || moved.ProjectID != f.target.ID {
		t.Errorf("moved = %s %s in %s, want %s ops-5 in ops", moved.ID, moved.Ref, moved.ProjectID, task.ID)
	}
	if moved.Status != "doing" {
		t.Errorf("review mapped to %q, want doing, the target's first in-progress state", moved.Status)
	}
	if !slices.Contains(moved.Tags, "urgent") || !slices.Contains(moved.Tags, "imported") {
		t.Errorf("tags after the move = %v", moved.Tags)
	}
	if !f.get(t, task.ID).Starred {
		t.Error("the star did not survive the move")
	}
	comments, err := f.l.ListComments(f.ctx, ref)
	if err != nil || len(comments) != 1 {
		t.Errorf("comments after the move = %v, %v", comments, err)
	}
	deps, err := f.l.ListDependencies(f.ctx, ref)
	if err != nil || len(deps) != 1 || deps[0].DependsOn != upstream.ID {
		t.Errorf("dependencies after the move = %+v, %v", deps, err)
	}
	if _, err := f.l.GetTask(f.ctx, core.TaskRef{ProjectKey: "old", Seq: 1}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("the old reference = %v, want not found", err)
	}
	if got := f.get(t, stay.ID); got.Ref != "old-2" {
		t.Errorf("a task the move did not name became %s", got.Ref)
	}
	var tags []core.Tag
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		tags, err = tx.ListTags(context.Background())
		return err
	}); err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if !slices.ContainsFunc(tags, func(tg core.Tag) bool {
		return tg.ProjectID == f.target.ID && tg.Name == "imported" && tg.Color == "red"
	}) {
		t.Errorf("no red imported tag scoped to the target among %+v", tags)
	}

	after := f.events(t)[events:]
	if len(after) != 1 {
		t.Fatalf("the move emitted %d events, want 1: %+v", len(after), after)
	}
	want := map[string]any{
		"ref": "ops-5", "from_ref": "old-1", "project": "ops", "from_project": "old",
		"status": "doing", "from_status": "review",
	}
	for k, v := range want {
		if after[0].Payload[k] != v {
			t.Errorf("event payload %s = %v, want %v", k, after[0].Payload[k], v)
		}
	}
	if after[0].Type != core.EventTaskUpdated {
		t.Errorf("event type %s, want %s", after[0].Type, core.EventTaskUpdated)
	}
	if _, ok := after[0].Payload["from_parent"]; ok {
		t.Error("a root task's move names a parent it never had")
	}
	audit, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID,
		Actions: []string{auditTaskMove}, Page: core.Page{Limit: 10}})
	if err != nil || len(audit) != 1 {
		t.Errorf("task.move audit entries = %+v, %v; want one", audit, err)
	}
}

func TestMovingATaskTakesItsSubtreeAndLeavesItsParent(t *testing.T) {
	f := newDeletionFixture(t)
	lab, err := f.l.CreateProject(f.ctx, core.CreateProjectInput{Key: "lab", Name: "Lab"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	parent := f.task(t, "old", "parent")
	task := f.child(t, "old", "task", parent)
	kid := f.child(t, "old", "kid", task)
	grandkid := f.child(t, "old", "grandkid", kid)
	binned := f.child(t, "old", "binned", task)
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: binned.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	abroad := f.child(t, lab.Key, "abroad", task)
	events := len(f.events(t))

	moved, err := f.l.MoveTask(f.ctx, core.TaskRef{ID: task.ID}, core.MoveTaskInput{ProjectRef: "ops"})
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if moved.Ref != "ops-1" || moved.ParentID != "" {
		t.Errorf("moved = %s under %q, want ops-1 as a root task", moved.Ref, moved.ParentID)
	}
	cases := []struct {
		task       *core.Task
		ref, under string
	}{
		{kid, "ops-2", task.ID},
		{grandkid, "ops-3", kid.ID},
		{parent, "old-1", ""},
		{abroad, "lab-1", task.ID},
	}
	for _, tc := range cases {
		got := f.get(t, tc.task.ID)
		if got.Ref != tc.ref || got.ParentID != tc.under {
			t.Errorf("%s is %s under %q, want %s under %q", tc.task.Title, got.Ref, got.ParentID, tc.ref, tc.under)
		}
	}
	var trashed *core.Task
	err = f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		trashed, err = tx.GetTask(context.Background(), core.TaskRef{ID: binned.ID})
		return err
	})
	if err != nil || trashed.ProjectID != f.source.ID || trashed.ParentID != task.ID {
		t.Errorf("the soft deleted subtask = %+v, %v; want it left in old under the moved task", trashed, err)
	}
	children, err := f.l.TaskTree(f.ctx, core.TaskRef{ID: parent.ID}, 0)
	if err != nil || len(children) != 1 {
		t.Errorf("the former parent's tree = %d tasks, %v; want it alone", len(children), err)
	}
	after := f.events(t)[events:]
	if len(after) != 3 {
		t.Fatalf("the move emitted %d events, want one per moved task", len(after))
	}
	if after[0].Payload["from_parent"] != parent.Ref {
		t.Errorf("the root's event names from_parent %v, want %s", after[0].Payload["from_parent"], parent.Ref)
	}
	if _, ok := after[1].Payload["from_parent"]; ok {
		t.Error("a subtask that kept its parent names from_parent")
	}
}

func TestATargetTagOfTheSameNameKeepsItsColour(t *testing.T) {
	f := newDeletionFixture(t)
	task := f.task(t, "old", "tagged")
	f.scopedTag(t, f.source, "area", "red", task)
	f.scopedTag(t, f.target, "area", "blue", nil)

	if _, err := f.l.MoveTask(f.ctx, core.TaskRef{ID: task.ID}, core.MoveTaskInput{ProjectRef: "ops"}); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if got := f.get(t, task.ID); !slices.Contains(got.Tags, "area") {
		t.Errorf("tags after the move = %v", got.Tags)
	}
	if err := f.l.store.View(context.Background(), f.scope, func(tx store.Tx) error {
		tags, err := projectTags(context.Background(), tx, f.target.ID)
		if err != nil {
			return err
		}
		if tags["area"].Color != "blue" {
			t.Errorf("the target's own tag became %q", tags["area"].Color)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading tags: %v", err)
	}
}

func TestMovingATaskIsRefusedWhereItCannotLand(t *testing.T) {
	f := newDeletionFixture(t)
	task := f.task(t, "old", "one")
	binned := f.task(t, "old", "binned")
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: binned.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatalf("DeleteTask: %v", err)
	}
	attic, err := f.l.CreateProject(f.ctx, core.CreateProjectInput{Key: "attic", Name: "Attic"})
	if err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if err := f.l.ArchiveProject(f.ctx, attic.Key); err != nil {
		t.Fatalf("ArchiveProject: %v", err)
	}
	title := "one, renamed"
	current, err := f.l.UpdateTask(f.ctx, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Title: &title})
	if err != nil {
		t.Fatalf("UpdateTask: %v", err)
	}
	events, audits := countRows(t, f.l, f.scope)
	cases := []struct {
		name string
		ref  string
		in   core.MoveTaskInput
		want core.Kind
	}{
		{"no target", task.ID, core.MoveTaskInput{ProjectRef: "  "}, core.KindInvalid},
		{"its own project", task.ID, core.MoveTaskInput{ProjectRef: "OLD"}, core.KindInvalid},
		{"archived target", task.ID, core.MoveTaskInput{ProjectRef: "attic"}, core.KindPrecondition},
		{"unknown target", task.ID, core.MoveTaskInput{ProjectRef: "nowhere"}, core.KindNotFound},
		{"stale version", task.ID, core.MoveTaskInput{ProjectRef: "ops", Version: current.Version - 1}, core.KindConflict},
		{"version ahead", task.ID, core.MoveTaskInput{ProjectRef: "ops", Version: current.Version + 1}, core.KindConflict},
		{"soft deleted task", binned.ID, core.MoveTaskInput{ProjectRef: "ops"}, core.KindNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := f.l.MoveTask(f.ctx, core.TaskRef{ID: tc.ref}, tc.in); !core.IsKind(err, tc.want) {
				t.Errorf("MoveTask = %v, want %s", err, tc.want)
			}
		})
	}
	if got := f.get(t, task.ID); got.Ref != "old-1" || got.Version != current.Version {
		t.Errorf("a refused move changed the task to %s v%d", got.Ref, got.Version)
	}
	if e, a := countRows(t, f.l, f.scope); e != events || a != audits {
		t.Errorf("refused moves wrote %d events and %d audit entries", e-events, a-audits)
	}
}

func TestMovingATaskNeedsTaskUpdateOnBothProjects(t *testing.T) {
	f := newDeletionFixture(t)
	task := f.task(t, "old", "one")
	actor, _ := core.ActorFrom(f.ctx)
	agent := func(scopes []core.Scope, project string) *core.Actor {
		return &core.Actor{ID: actor.ID, TenantID: actor.TenantID, Kind: core.ActorAgent, Scopes: scopes, ProjectID: project}
	}
	cases := []struct {
		name  string
		actor *core.Actor
	}{
		{"read only", agent([]core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}, "")},
		{"confined to the source", agent([]core.Scope{core.ScopeAll}, f.source.ID)},
		{"confined to the target", agent([]core.Scope{core.ScopeAll}, f.target.ID)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := core.WithActor(context.Background(), tc.actor)
			_, err := f.l.MoveTask(ctx, core.TaskRef{ID: task.ID}, core.MoveTaskInput{ProjectRef: "ops"})
			if !core.IsKind(err, core.KindForbidden) {
				t.Errorf("MoveTask = %v, want forbidden", err)
			}
		})
	}
	if got := f.get(t, task.ID); got.Ref != "old-1" {
		t.Errorf("a refused move moved the task to %s", got.Ref)
	}
}
