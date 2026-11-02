// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestLeaseColumnsRoundTrip proves the project's lease bounds and the task's
// lease are written by both the insert and the update, and read back.
func TestLeaseColumnsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "long build", core.PriorityNormal)

	var gotProject *core.Project
	var gotTask, created *core.Task
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		p := f.project
		p.LeaseDefault, p.LeaseMax = core.Duration(2*time.Hour), core.Duration(8*time.Hour)
		if err := tx.UpdateProject(ctx, &p); err != nil {
			return err
		}
		other := core.Project{Key: "ops", Name: "Ops", WorkflowID: f.project.WorkflowID, LeaseMax: core.Duration(time.Hour)}
		if err := tx.CreateProject(ctx, &other); err != nil {
			return err
		}
		reread, err := tx.GetProject(ctx, other.ID)
		if err != nil {
			return err
		}
		if reread.LeaseMax.D() != time.Hour {
			t.Errorf("created project lease max = %s, want 1h", reread.LeaseMax)
		}

		task.Lease = core.Duration(4 * time.Hour)
		if err := tx.UpdateTask(ctx, &task); err != nil {
			return err
		}
		fresh := core.Task{ProjectID: other.ID, Title: "quick", Status: "todo", CreatorActorID: f.actor.ID,
			Lease: core.Duration(90 * time.Minute)}
		if err := tx.CreateTask(ctx, &fresh); err != nil {
			return err
		}
		if gotProject, err = tx.GetProject(ctx, f.project.ID); err != nil {
			return err
		}
		if gotTask, err = tx.GetTask(ctx, core.TaskRef{ID: task.ID}); err != nil {
			return err
		}
		created, err = tx.GetTask(ctx, core.TaskRef{ID: fresh.ID})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if gotProject.LeaseDefault.D() != 2*time.Hour || gotProject.LeaseMax.D() != 8*time.Hour {
		t.Fatalf("project lease = %s / %s, want 2h / 8h", gotProject.LeaseDefault, gotProject.LeaseMax)
	}
	if gotTask.Lease.D() != 4*time.Hour {
		t.Fatalf("updated task lease = %s, want 4h", gotTask.Lease)
	}
	if created.Lease.D() != 90*time.Minute {
		t.Fatalf("created task lease = %s, want 90m", created.Lease)
	}
}
