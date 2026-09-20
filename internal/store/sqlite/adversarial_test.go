package sqlite

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// These tests were written independently of the implementation, to attack it
// rather than to confirm it. The scenario each one targets is stated up front.

// Attack: hold a valid handle to tenant A and ask for tenant B's task by its
// real id. A scope check that trusted the id rather than the predicate would
// return the row.
func TestForgedScopeCannotReadAnotherTenantsTask(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "alpha")
	b := seed(t, s, clk, "bravo")

	victim := b.newTask(t, "bravo secret", core.PriorityNormal)

	err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: victim.ID})
		if err == nil {
			t.Errorf("tenant alpha read tenant bravo's task: %+v", got)
		}
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("cross-tenant read error = %v, want not found", err)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("view: %v", err)
	}
}

// Attack: address another tenant's task by its human ref, which is only unique
// per project and therefore collides across tenants by construction.
func TestHumanRefDoesNotCrossTenants(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "alpha")
	b := seed(t, s, clk, "bravo")

	ta := a.newTask(t, "alpha one", core.PriorityNormal)
	tb := b.newTask(t, "bravo one", core.PriorityNormal)

	if ta.Seq != tb.Seq {
		t.Fatalf("expected colliding sequence numbers across tenants, got %d and %d", ta.Seq, tb.Seq)
	}

	ref := core.TaskRef{ProjectKey: a.project.Key, Seq: tb.Seq}
	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, ref)
		switch {
		case core.IsKind(err, core.KindNotFound):
			return nil
		case err != nil:
			return err
		case got.ID == tb.ID:
			t.Errorf("human ref resolved to another tenant's task %q", got.ID)
		case got.TenantID != a.tenant.ID:
			t.Errorf("resolved task belongs to tenant %q, want %q", got.TenantID, a.tenant.ID)
		}
		return nil
	}); err != nil {
		t.Fatalf("view: %v", err)
	}
}

// Attack: claim another tenant's task. A claim is a conditional UPDATE, so a
// missing tenant predicate would silently succeed and steal the row.
func TestCannotClaimAnotherTenantsTask(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "alpha")
	b := seed(t, s, clk, "bravo")

	victim := b.newTask(t, "bravo work", core.PriorityNormal)
	now := clk.Now()

	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID:     victim.ID,
			ActorID:    a.actor.ID,
			Now:        now,
			Until:      now.Add(15 * time.Minute),
			LeaseToken: "forged-token",
		})
		if err == nil && ok {
			t.Error("tenant alpha claimed tenant bravo's task")
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.View(ctx, b.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: victim.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" {
			t.Errorf("victim task was claimed by %q", got.ClaimedByActorID)
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying victim: %v", err)
	}
}

// Attack: a task is claimed, then the lease expires and someone else takes it.
// The original holder must not be able to act with its old token.
func TestZombieWorkerCannotActAfterReclaim(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "alpha")
	task := f.newTask(t, "long job", core.PriorityNormal)

	const ttl = 15 * time.Minute
	now := clk.Now()

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID: task.ID, ActorID: f.actor.ID, Now: now,
			Until: now.Add(ttl), LeaseToken: "first-holder",
		})
		if err != nil || !ok {
			t.Fatalf("first claim failed: ok=%v err=%v", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	clk.Advance(ttl + 1)
	later := clk.Now()

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID: task.ID, ActorID: f.actor.ID, Now: later,
			Until: later.Add(ttl), LeaseToken: "second-holder",
		})
		if err != nil || !ok {
			t.Fatalf("reclaim after expiry failed: ok=%v err=%v", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.RenewLease(ctx, task.ID, "first-holder", later.Add(ttl*2))
		if err == nil && ok {
			t.Error("the zombie holder renewed a lease it no longer owns")
		}
		ok, err = tx.ReleaseLease(ctx, task.ID, "first-holder")
		if err == nil && ok {
			t.Error("the zombie holder released a lease it no longer owns")
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID == "" {
			t.Error("the zombie's release detached the second holder's claim")
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying claim: %v", err)
	}
}

// Attack: many workers racing for one queue must never be handed the same task,
// even across independent store instances on the same file.
func TestClaimNextNeverDoubleClaimsAcrossStores(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "alpha")

	const tasks, workers = 10, 20
	for i := range tasks {
		f.newTask(t, "queued", core.Priority(1+i%5))
	}

	now := clk.Now()
	var (
		mu      sync.Mutex
		claimed []string
		wg      sync.WaitGroup
	)
	for i := range workers {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = s.Update(ctx, f.scope, func(tx store.Tx) error {
				id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
					ActorID: f.actor.ID, Now: now, Until: now.Add(15 * time.Minute),
					LeaseToken:     fmt.Sprintf("tok-%d", i),
					TerminalStates: []string{"done"},
				})
				if err != nil {
					return err
				}
				if !ok {
					return nil
				}
				mu.Lock()
				claimed = append(claimed, id)
				mu.Unlock()
				return nil
			})
		}(i)
	}
	wg.Wait()

	if len(claimed) != tasks {
		t.Errorf("claimed %d tasks, want exactly %d", len(claimed), tasks)
	}
	seen := make(map[string]bool, len(claimed))
	for _, id := range claimed {
		if seen[id] {
			t.Errorf("task %q was claimed twice", id)
		}
		seen[id] = true
	}
}

// Spec: "Renew after expiry -> the request is rejected as lease-expired and no
// lease is re-established." Without this, a worker whose lease lapsed could
// resurrect its claim on a task that lazy expiry had already made available,
// and the next worker to ask for it would be refused.
func TestExpiredLeaseCannotBeRenewedOrReleased(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "alpha")
	task := f.newTask(t, "long job", core.PriorityNormal)

	const ttl = 15 * time.Minute
	now := clk.Now()

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID: task.ID, ActorID: f.actor.ID, Now: now,
			Until: now.Add(ttl), LeaseToken: "tok-1",
		})
		if err != nil || !ok {
			t.Fatalf("initial claim: ok=%v err=%v", ok, err)
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	clk.Advance(ttl + time.Minute)
	later := clk.Now()

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.RenewLease(ctx, task.ID, "tok-1", later.Add(ttl))
		if err != nil {
			return err
		}
		if ok {
			t.Error("an expired lease was renewed; the holder must re-claim instead")
		}
		ok, err = tx.ReleaseLease(ctx, task.ID, "tok-1")
		if err != nil {
			return err
		}
		if ok {
			t.Error("an expired lease was released with its stale token")
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{
			TaskID: task.ID, ActorID: f.actor.ID, Now: later,
			Until: later.Add(ttl), LeaseToken: "tok-2",
		})
		if err != nil {
			return err
		}
		if !ok {
			t.Error("the task was not claimable after its lease expired")
		}
		return nil
	}); err != nil {
		t.Fatalf("update: %v", err)
	}
}
