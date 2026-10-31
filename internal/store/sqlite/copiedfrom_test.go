// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestCopiedFromRoundTrip writes the link on create, reads it back, and
// checks that neither an update nor purging the source clears it.
func TestCopiedFromRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	source := f.newTask(t, "original", core.PriorityNormal)
	task := core.Task{
		ProjectID: f.project.ID, Title: "copy", Status: "todo",
		CreatorActorID: f.actor.ID, CopiedFromID: source.ID,
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
	if got.CopiedFromID != source.ID {
		t.Fatalf("copied_from_id = %q, want %q", got.CopiedFromID, source.ID)
	}
	if plain := read(source.ID); plain.CopiedFromID != "" {
		t.Fatalf("a task created without a link reads back %q", plain.CopiedFromID)
	}
	got.Title = "renamed"
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.UpdateTask(ctx, got); err != nil {
			return err
		}
		return tx.DeleteTask(ctx, source.ID, true)
	}); err != nil {
		t.Fatalf("updating and purging: %v", err)
	}
	if kept := read(task.ID); kept.CopiedFromID != source.ID {
		t.Fatalf("an update or purging the source cleared the link: %q", kept.CopiedFromID)
	}
}
