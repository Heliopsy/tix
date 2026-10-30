// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestRecurrenceRoundTrip writes a rule and a successor link through create
// and update, and reads both back, so neither column is dropped on one path.
func TestRecurrenceRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	first := f.newTask(t, "pay rent", core.PriorityNormal)
	task := core.Task{
		ProjectID: f.project.ID, Title: "water plants", Status: "todo",
		CreatorActorID: f.actor.ID, Recurrence: "every week on mon,thu",
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task: %v", err)
	}

	read := func(id string) *core.Task {
		t.Helper()
		var out *core.Task
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			out, err = tx.GetTask(ctx, core.TaskRef{ID: id})
			return err
		}); err != nil {
			t.Fatalf("reading task: %v", err)
		}
		return out
	}
	got := read(task.ID)
	if got.Recurrence != "every week on mon,thu" || got.RecurrenceNextID != "" {
		t.Fatalf("created rule = %q, next = %q", got.Recurrence, got.RecurrenceNextID)
	}
	if plain := read(first.ID); plain.Recurrence != "" || plain.RecurrenceNextID != "" {
		t.Fatalf("a task created without a rule reads back %q, %q", plain.Recurrence, plain.RecurrenceNextID)
	}

	got.Recurrence = "every day for 2 times"
	got.RecurrenceNextID = first.ID
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.UpdateTask(ctx, got)
	}); err != nil {
		t.Fatalf("updating task: %v", err)
	}
	again := read(task.ID)
	if again.Recurrence != "every day for 2 times" || again.RecurrenceNextID != first.ID {
		t.Fatalf("updated rule = %q, next = %q", again.Recurrence, again.RecurrenceNextID)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.DeleteTask(ctx, first.ID, true)
	}); err != nil {
		t.Fatalf("purging the successor: %v", err)
	}
	if kept := read(task.ID); kept.RecurrenceNextID != first.ID {
		t.Fatalf("purging the successor cleared the link to %q", kept.RecurrenceNextID)
	}
}
