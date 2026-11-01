// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestSnoozeColumns writes a snooze, filters on it against the store clock,
// and wakes it only while it still carries the wake time that was read.
func TestSnoozeColumns(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	asleep := f.newTask(t, "asleep", core.PriorityNormal)
	f.newTask(t, "awake", core.PriorityNormal)
	until := clk.Now().Add(time.Hour).Truncate(time.Second)

	update := func(fn func(tx store.Tx) error) {
		t.Helper()
		if err := s.Update(ctx, f.scope, fn); err != nil {
			t.Fatal(err)
		}
	}
	titles := func(filter core.TaskFilter) []string {
		t.Helper()
		var out []string
		filter.Page.Sort = core.SortSeq
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			tasks, err := tx.ListTasks(ctx, filter)
			for _, task := range tasks {
				out = append(out, task.Title)
			}
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}
	read := func() *core.Task {
		t.Helper()
		var out *core.Task
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			out, err = tx.GetTask(ctx, core.TaskRef{ID: asleep.ID})
			return err
		}); err != nil {
			t.Fatal(err)
		}
		return out
	}

	before := read().Version
	update(func(tx store.Tx) error { return tx.SetSnooze(ctx, asleep.ID, &until, f.actor.ID) })
	got := read()
	if got.SnoozedUntil == nil || !got.SnoozedUntil.Equal(until) || got.SnoozedByActorID != f.actor.ID {
		t.Fatalf("read back until %v by %q, want %v by %q", got.SnoozedUntil, got.SnoozedByActorID, until, f.actor.ID)
	}
	if got.Version != before {
		t.Fatalf("a snooze moved the version %d -> %d", before, got.Version)
	}
	got.Title = "edited"
	update(func(tx store.Tx) error { return tx.UpdateTask(ctx, got) })
	if read().SnoozedUntil == nil {
		t.Fatal("UpdateTask cleared the snooze")
	}

	if got := titles(core.TaskFilter{}); !slices.Equal(got, []string{"edited", "awake"}) {
		t.Fatalf("the store's own default = %v, want every task: the service owns the hiding default", got)
	}
	if got := titles(core.TaskFilter{Snoozed: core.Yes}); !slices.Equal(got, []string{"edited"}) {
		t.Fatalf("snoozed = %v", got)
	}
	if got := titles(core.TaskFilter{Snoozed: core.No}); !slices.Equal(got, []string{"awake"}) {
		t.Fatalf("awake = %v", got)
	}

	var expired []core.Task
	update(func(tx store.Tx) (err error) { expired, err = tx.ExpiredSnoozes(ctx, clk.Now(), 10); return err })
	if len(expired) != 0 {
		t.Fatalf("expired before the wake time = %d", len(expired))
	}
	clk.Advance(2 * time.Hour)
	if got := titles(core.TaskFilter{Snoozed: core.No}); !slices.Equal(got, []string{"edited", "awake"}) {
		t.Fatalf("awake after the wake time = %v, want both", got)
	}
	update(func(tx store.Tx) (err error) { expired, err = tx.ExpiredSnoozes(ctx, clk.Now(), 10); return err })
	if len(expired) != 1 || expired[0].ID != asleep.ID {
		t.Fatalf("expired = %+v, want the one task", expired)
	}

	stale := until.Add(-time.Minute)
	update(func(tx store.Tx) error {
		ok, err := tx.WakeSnooze(ctx, asleep.ID, stale)
		if ok {
			t.Error("woke a task whose wake time changed since the read")
		}
		return err
	})
	update(func(tx store.Tx) error {
		ok, err := tx.WakeSnooze(ctx, asleep.ID, *expired[0].SnoozedUntil)
		if !ok {
			t.Error("did not wake the task with the wake time it carries")
		}
		return err
	})
	if got := read(); got.SnoozedUntil != nil || got.SnoozedByActorID != "" {
		t.Fatalf("after waking: until %v by %q", got.SnoozedUntil, got.SnoozedByActorID)
	}

	update(func(tx store.Tx) error { return tx.SetSnooze(ctx, asleep.ID, &until, f.actor.ID) })
	update(func(tx store.Tx) error { return tx.SetSnooze(ctx, asleep.ID, nil, f.actor.ID) })
	if got := read(); got.SnoozedUntil != nil || got.SnoozedByActorID != "" {
		t.Fatalf("clearing left until %v by %q", got.SnoozedUntil, got.SnoozedByActorID)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.SetSnooze(ctx, "01NOSUCHTASK0000000000000", &until, f.actor.ID)
	}); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("snoozing a missing task = %v, want not found", err)
	}
}
