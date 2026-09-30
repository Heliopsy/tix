// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// The two-administrator interleave cannot be produced on this engine: a write
// transaction takes the single writer connection and issues BEGIN IMMEDIATE, so
// a second writer cannot commit between the first one's read and its write. The
// predicate that decides the race is asserted here instead, and the engines stay
// identical from the service's side. internal/store/postgres holds the raced
// version.

// A reclaim is keyed on the holder the caller read. Naming a holder the row does
// not carry writes nothing, which is what the loser of a race meets and what an
// administrator acting on a stale read meets outside one.
func TestForceReclaimRefusesAHolderTheRowDoesNotCarry(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "held", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Hour), LeaseToken: "live"})
		return err
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ForceReclaim(ctx, store.ForceReclaimRow{
			TaskID: task.ID, HolderID: "01J0000000000000000000000Y", At: clk.Now()})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a reclaim naming a holder the row does not carry must write nothing")
		}
		if _, err := tx.ForceReclaim(ctx, store.ForceReclaimRow{
			TaskID: "01J0000000000000000000000X", At: clk.Now()}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("reclaiming a task that is not there = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("reclaiming against the wrong holder: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != f.actor.ID {
			t.Errorf("the refused reclaim moved the lease: holder is %q", got.ClaimedByActorID)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the task back: %v", err)
	}
}

// A lapsed lease is not the administrator's to force: the task is already free,
// and the sweeper's evidence columns are what describe it.
func TestForceReclaimRefusesALapsedLease(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "lapsed", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "gone"})
		return err
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	clk.Advance(2 * time.Minute)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ForceReclaim(ctx, store.ForceReclaimRow{
			TaskID: task.ID, HolderID: f.actor.ID, At: clk.Now()})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a lapsed lease must not be reclaimed")
		}
		return nil
	}); err != nil {
		t.Fatalf("reclaiming a lapsed lease: %v", err)
	}
}

// The reclaim moves the lease columns and leaves every other column alone,
// including the status and the expiry evidence a sweep would have written.
func TestForceReclaimClearsOnlyTheLease(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "held", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Hour), LeaseToken: "live"})
		return err
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}
	before, err := readTask(ctx, s, f, task.ID)
	if err != nil {
		t.Fatalf("reading the claimed task: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ForceReclaim(ctx, store.ForceReclaimRow{
			TaskID: task.ID, HolderID: f.actor.ID, At: clk.Now()})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("the reclaim matched no row")
		}
		return nil
	}); err != nil {
		t.Fatalf("reclaiming: %v", err)
	}

	after, err := readTask(ctx, s, f, task.ID)
	if err != nil {
		t.Fatalf("reading the reclaimed task: %v", err)
	}
	if after.ClaimedByActorID != "" || after.LeaseExpiresAt != nil {
		t.Errorf("the lease survived: holder %q, expiry %v", after.ClaimedByActorID, after.LeaseExpiresAt)
	}
	if after.LeaseExpiredAt != nil || after.LeaseExpiredByActorID != "" {
		t.Errorf("the reclaim wrote expiry evidence: at %v by %q",
			after.LeaseExpiredAt, after.LeaseExpiredByActorID)
	}
	if after.Status != before.Status {
		t.Errorf("the reclaim moved the status from %q to %q", before.Status, after.Status)
	}
	if after.ClaimCount != before.ClaimCount {
		t.Errorf("the reclaim changed the claim count from %d to %d", before.ClaimCount, after.ClaimCount)
	}
}

// readTask loads one task outside a write transaction.
func readTask(ctx context.Context, s *Store, f fixture, id string) (*core.Task, error) {
	var out *core.Task
	err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: id})
		out = got
		return err
	})
	return out, err
}
