// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Two administrators forcing the same task at once. The reclaim is keyed on the
// holder both of them read, so the second finds the row no longer carrying it
// and reports having written nothing. Keyed on the row identifier alone -- the
// sweeper's own old bug -- both would report success and the second would be
// ending a lease it never saw.
//
// Two real transactions in two goroutines against a real database. The fake
// clock supplies instants and decides nothing about the schedule: the
// interleaving is what is under test, and a clock cannot order that.
func TestForceReclaimHasOneWinnerWhenTwoAdministratorsRace(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "contended", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Hour), LeaseToken: "worker-a"})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("the holder's claim should succeed")
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding the lease: %v", err)
	}

	const racers = 2
	var (
		wg      sync.WaitGroup
		start   = make(chan struct{})
		mu      sync.Mutex
		wins    int
		results = make(chan error, racers)
	)
	for range racers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			results <- s.Update(ctx, f.scope, func(tx store.Tx) error {
				ok, err := tx.ForceReclaim(ctx, store.ForceReclaimRow{
					TaskID: task.ID, HolderID: f.actor.ID, At: clk.Now()})
				if err != nil {
					return err
				}
				mu.Lock()
				defer mu.Unlock()
				if ok {
					wins++
				}
				return nil
			})
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		if err != nil {
			t.Fatalf("forcing the reclaim: %v", err)
		}
	}

	if wins != 1 {
		t.Fatalf("%d of %d concurrent reclaims reported taking the lease, want exactly 1", wins, racers)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" || got.ClaimedAtTime(clk.Now()) {
			t.Errorf("the lease survived the reclaim: %+v", got)
		}
		if got.LeaseExpiredAt != nil || got.LeaseExpiredByActorID != "" {
			t.Errorf("the reclaim recorded an expiry: at %v by %q",
				got.LeaseExpiredAt, got.LeaseExpiredByActorID)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the contended task: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.RenewLease(ctx, task.ID, "worker-a", clk.Now().Add(time.Hour))
		if err != nil {
			return err
		}
		if ok {
			t.Error("the reclaimed holder's token can still renew")
		}
		return nil
	}); err != nil {
		t.Fatalf("renewing with the reclaimed token: %v", err)
	}
}

// A reclaim aimed at one holder must not land on another's lease, which is what
// the second racer above meets and what an administrator acting on a stale read
// meets outside any race.
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
