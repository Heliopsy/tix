package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func TestClaimIsConditionalOnTheLease(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "claimable", core.PriorityNormal)

	lease := clk.Now().Add(15 * time.Minute)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: lease, LeaseToken: "token-1"})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("first claim should succeed")
		}
		ok, err = tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: lease, LeaseToken: "token-2"})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a live lease must not be claimable")
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != f.actor.ID || got.ClaimCount != 1 {
			t.Fatalf("claimed task = %+v", got)
		}
		if !got.ClaimedAtTime(clk.Now()) {
			t.Fatal("task should read as claimed while the lease is live")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the claim: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		ok, err := tx.RenewLease(ctx, task.ID, "wrong", clk.Now().Add(time.Hour))
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a stale token must not renew a lease")
		}
		ok, err = tx.ReleaseLease(ctx, task.ID, "wrong")
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a stale token must not release a lease")
		}
		if _, err := tx.RenewLease(ctx, task.ID, "", clk.Now()); !core.IsKind(err, core.KindInvalid) {
			t.Fatal("an empty token must be rejected")
		}
		if _, err := tx.ReleaseLease(ctx, task.ID, ""); !core.IsKind(err, core.KindInvalid) {
			t.Fatal("an empty token must be rejected")
		}
		ok, err = tx.RenewLease(ctx, task.ID, "token-1", clk.Now().Add(time.Hour))
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("the current token should renew the lease")
		}
		return nil
	}); err != nil {
		t.Fatalf("lease tokens: %v", err)
	}

	clk.Advance(2 * time.Hour)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		expired, err := tx.ExpiredLeases(ctx, clk.Now(), 10)
		if err != nil {
			return err
		}
		if len(expired) != 1 || expired[0].ID != task.ID {
			t.Fatalf("expired leases = %+v", expired)
		}
		if ok, err := tx.RenewLease(ctx, task.ID, "token-1", clk.Now().Add(time.Hour)); err != nil || ok {
			t.Fatalf("an expired lease must not be renewable: ok=%v err=%v", ok, err)
		}
		ok, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "token-3"})
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("an expired lease must be claimable")
		}
		ok, err = tx.ReleaseLease(ctx, task.ID, "token-3")
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("releasing with the current token should succeed")
		}
		return nil
	}); err != nil {
		t.Fatalf("expired lease: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" || got.LeaseExpiresAt != nil || got.ClaimCount != 2 {
			t.Fatalf("released task = %+v", got)
		}
		unclaimed, err := tx.ListTasks(ctx, core.TaskFilter{Claimed: core.No})
		if err != nil {
			return err
		}
		if len(unclaimed) != 1 {
			t.Fatalf("unclaimed tasks = %d, want 1", len(unclaimed))
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the released task: %v", err)
	}
}

func TestClaimNextTaskRespectsOrderFiltersAndDependencies(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	low := f.newTask(t, "low", core.PriorityLow)
	clk.Advance(time.Second)
	high := f.newTask(t, "high", core.PriorityHighest)
	clk.Advance(time.Second)
	blocked := f.newTask(t, "blocked", core.PriorityHighest)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		tag := core.Tag{Name: "queue"}
		if err := tx.PutTag(ctx, &tag); err != nil {
			return err
		}
		if err := tx.AttachTag(ctx, low.ID, tag.ID); err != nil {
			return err
		}
		return tx.AddDependency(ctx, &core.Dependency{TaskID: blocked.ID, DependsOn: low.ID})
	}); err != nil {
		t.Fatalf("preparing the queue: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: "t1", TerminalStates: []string{"done"},
			ProjectIDs: []string{f.project.ID}, Statuses: []string{"todo"},
		})
		if err != nil {
			return err
		}
		if !ok || id != high.ID {
			t.Fatalf("claimed %q ok=%v, want the highest priority unblocked task %q", id, ok, high.ID)
		}
		id, ok, err = tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: "t2", TerminalStates: []string{"done"}, Tags: []string{"queue"},
		})
		if err != nil {
			return err
		}
		if !ok || id != low.ID {
			t.Fatalf("tag-filtered claim = %q ok=%v, want %q", id, ok, low.ID)
		}
		_, ok, err = tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: "t3", TerminalStates: []string{"done"},
		})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a blocked task must not be claimable")
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming in order: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.ClearClaim(ctx, high.ID); err != nil {
			return err
		}
		got, err := tx.GetTask(ctx, core.TaskRef{ID: high.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" {
			t.Fatalf("claim was not cleared: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("clearing a claim: %v", err)
	}
}
