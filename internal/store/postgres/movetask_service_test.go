// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
)

// TestTheServiceMovesATaskAndItsSubtreeOnPostgres drives MoveTask through
// the service on this engine: the detach, the renumbering, the scoped tag and
// the version check all write in the one transaction PostgreSQL runs.
func TestTheServiceMovesATaskAndItsSubtreeOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	lean := core.Workflow{Key: "lean", Name: "Lean", Definition: core.WorkflowDefinition{
		Initial:     "open",
		States:      []core.State{{Key: "open", Label: "Open"}, {Key: "closed", Label: "Closed", Terminal: true}},
		Transitions: []core.Transition{{From: "open", To: "closed"}},
	}}
	beta := core.Project{Key: "beta", Name: "Beta"}
	parent := core.Task{ProjectID: f.project.ID, Title: "parent", Status: "todo", CreatorActorID: f.actor.ID}
	task := core.Task{ProjectID: f.project.ID, Title: "task", Status: "doing", CreatorActorID: f.actor.ID}
	kid := core.Task{ProjectID: f.project.ID, Title: "kid", Status: "todo", CreatorActorID: f.actor.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &lean); err != nil {
			return err
		}
		beta.WorkflowID = lean.ID
		if err := tx.CreateProject(ctx, &beta); err != nil {
			return err
		}
		if err := tx.CreateTask(ctx, &parent); err != nil {
			return err
		}
		task.ParentID = parent.ID
		if err := tx.CreateTask(ctx, &task); err != nil {
			return err
		}
		kid.ParentID = task.ID
		if err := tx.CreateTask(ctx, &kid); err != nil {
			return err
		}
		tag := &core.Tag{ProjectID: f.project.ID, Name: "area", Color: "red"}
		if err := tx.PutTag(ctx, tag); err != nil {
			return err
		}
		return tx.AttachTag(ctx, task.ID, tag.ID)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	l := service.New(s, service.WithClock(clk))
	actor := f.actor
	actor.TenantID, actor.Kind, actor.Scopes = f.tenant.ID, core.ActorUser, []core.Scope{core.ScopeAll}
	as := core.WithActor(ctx, &actor)

	current, err := l.GetTask(as, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if _, err := l.MoveTask(as, core.TaskRef{ID: task.ID}, core.MoveTaskInput{ProjectRef: "beta", Version: current.Version + 1}); !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a stale move = %v, want a conflict", err)
	}
	moved, err := l.MoveTask(as, core.TaskRef{ID: task.ID}, core.MoveTaskInput{ProjectRef: "beta", Version: current.Version})
	if err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	if moved.ID != task.ID || moved.Ref != "beta-1" || moved.ParentID != "" || !lean.Definition.HasState(moved.Status) {
		t.Errorf("moved = %s %s under %q in %q, want beta-1, a root task, in a lean state", moved.ID, moved.Ref, moved.ParentID, moved.Status)
	}
	if !slices.Contains(moved.Tags, "area") {
		t.Errorf("tags after the move = %v", moved.Tags)
	}
	for _, tc := range []struct {
		id, ref, under string
	}{{kid.ID, "beta-2", task.ID}, {parent.ID, "alpha-1", ""}} {
		got, err := l.GetTask(as, core.TaskRef{ID: tc.id})
		if err != nil || got.Ref != tc.ref || got.ParentID != tc.under {
			t.Errorf("task %s = %+v, %v; want %s under %q", tc.id, got, err, tc.ref, tc.under)
		}
	}
}
