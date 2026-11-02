// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func days(n int) *int { return &n }

func TestHousekeepingPolicyRoundTrips(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	var got []core.HousekeepingPolicy
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		for _, p := range []core.HousekeepingPolicy{
			{Done: days(30), Trash: days(0)},
			{ProjectID: f.project.ID, Cancelled: days(7)},
			{Done: days(14), Trash: days(0)},
		} {
			if err := tx.PutHousekeepingPolicy(ctx, &p); err != nil {
				return err
			}
		}
		var err error
		got, err = tx.HousekeepingPolicies(ctx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ProjectID != "" || got[0].Days(core.RuleDone) != 14 ||
		got[0].Cancelled != nil || got[0].Trash == nil || *got[0].Trash != 0 {
		t.Fatalf("tenant level = %+v, want done 14, cancelled unset and trash explicitly off", got)
	}
	if got[1].ProjectID != f.project.ID || got[1].Done != nil || got[1].Days(core.RuleCancelled) != 7 {
		t.Fatalf("project level = %+v, want only cancelled 7", got[1])
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutHousekeepingPolicy(ctx, &core.HousekeepingPolicy{ProjectID: f.project.ID}); err != nil {
			return err
		}
		var err error
		got, err = tx.HousekeepingPolicies(ctx)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("an override that inherits everything left a row: %+v", got)
	}

	other := seed(t, s, clk, "other")
	if err := s.View(ctx, other.scope, func(tx store.Tx) error {
		var err error
		got, err = tx.HousekeepingPolicies(ctx)
		return err
	}); err != nil || len(got) != 0 {
		t.Errorf("another tenant reads %+v (%v), want nothing", got, err)
	}
}

// Every acting statement is conditional, so a second worker that selected the
// same task changes nothing and reports so.
func TestHousekeepingActsOnceOnARow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	completed := clk.Now()
	task := core.Task{ProjectID: f.project.ID, Title: "shipped", Status: "done",
		CompletedAt: &completed, CreatorActorID: f.actor.ID}
	open := f.newTask(t, "open", core.PriorityNormal)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateTask(ctx, &task) }); err != nil {
		t.Fatal(err)
	}
	clk.Advance(48 * time.Hour)
	cutoff := clk.Now().Add(-24 * time.Hour)
	q := store.FinishedQuery{ProjectID: f.project.ID, Statuses: []string{"done"}, Before: cutoff}

	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		found, err := tx.FinishedTasks(ctx, q, 10)
		if err != nil {
			return err
		}
		n, err := tx.CountFinishedTasks(ctx, q)
		if err != nil {
			return err
		}
		if len(found) != 1 || found[0].ID != task.ID || n != 1 {
			t.Errorf("finished = %v, count %d, want only the done task", found, n)
		}
		if ok, err := tx.TrashFinished(ctx, open.ID, q); ok || err != nil {
			t.Errorf("trashing an open task = %v, %v, want no row", ok, err)
		}
		for i, want := range []bool{true, false} {
			if ok, err := tx.TrashFinished(ctx, task.ID, q); ok != want || err != nil {
				t.Errorf("trash attempt %d = %v, %v, want %v", i+1, ok, err, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	clk.Advance(48 * time.Hour)
	cutoff = clk.Now().Add(-24 * time.Hour)
	err = s.Update(ctx, f.scope, func(tx store.Tx) error {
		expired, err := tx.ExpiredTrash(ctx, f.project.ID, cutoff, 10)
		if err != nil {
			return err
		}
		if n, _ := tx.CountExpiredTrash(ctx, f.project.ID, cutoff); len(expired) != 1 || n != 1 {
			t.Errorf("expired trash = %v, count %d, want the trashed task", expired, n)
		}
		if ok, _ := tx.PurgeTrashed(ctx, open.ID, cutoff); ok {
			t.Error("purged a live task")
		}
		for i, want := range []bool{true, false} {
			if ok, err := tx.PurgeTrashed(ctx, task.ID, cutoff); ok != want || err != nil {
				t.Errorf("purge attempt %d = %v, %v, want %v", i+1, ok, err, want)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
