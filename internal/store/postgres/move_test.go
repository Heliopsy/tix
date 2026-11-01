// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestMoveTaskRehomesTheRowAndKeepsItsIdentity(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	other := core.Project{Key: "beta", Name: "Beta", WorkflowID: f.workflow.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateProject(ctx, &other)
	}); err != nil {
		t.Fatalf("creating the second project: %v", err)
	}
	task := f.newTask(t, "travels", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.MoveTask(ctx, task.ID, other.ID, 7, "doing")
	}); err != nil {
		t.Fatalf("MoveTask: %v", err)
	}
	var got *core.Task
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		var err error
		got, err = tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	}); err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if got.ProjectID != other.ID || got.Seq != 7 || got.Ref != "beta-7" || got.Status != "doing" {
		t.Errorf("moved task = project %q seq %d ref %q status %q, want %q 7 beta-7 doing",
			got.ProjectID, got.Seq, got.Ref, got.Status, other.ID)
	}
	if got.Version != task.Version+1 {
		t.Errorf("version = %d, want %d", got.Version, task.Version+1)
	}

	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.MoveTask(ctx, "missing", other.ID, 8, "todo")
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Errorf("MoveTask on a missing task = %v, want not found", err)
	}
}

func TestMoveTaskStaysInsideTheTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	mine := seed(t, s, clk, "acme")
	theirs := seed(t, s, clk, "globex")
	task := theirs.newTask(t, "not yours", core.PriorityNormal)

	err := s.Update(ctx, mine.scope, func(tx store.Tx) error {
		return tx.MoveTask(ctx, task.ID, mine.project.ID, 9, "todo")
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("moving another tenant's task = %v, want not found", err)
	}
}
