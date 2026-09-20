package sqlite

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
		claimed, err := tx.ListTasks(ctx, core.TaskFilter{Claimed: core.Yes})
		if err != nil {
			return err
		}
		if len(claimed) != 0 {
			t.Fatalf("claimed tasks = %d, want 0", len(claimed))
		}
		return nil
	}); err != nil {
		t.Fatalf("after release: %v", err)
	}
}

func TestClearClaim(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "swept", core.PriorityNormal)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if _, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "token"}); err != nil {
			return err
		}
		return tx.ClearClaim(ctx, task.ID)
	}); err != nil {
		t.Fatalf("clearing a claim: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" {
			t.Fatalf("claim was not cleared: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading after clear: %v", err)
	}
}

func TestClaimNextTaskRespectsDependenciesAndPriority(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	blocker := f.newTask(t, "blocker", core.PriorityLow)
	clk.Advance(time.Second)
	blocked := f.newTask(t, "blocked", core.PriorityHighest)
	clk.Advance(time.Second)
	free := f.newTask(t, "free", core.PriorityHigh)

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDependency(ctx, &core.Dependency{TaskID: blocked.ID, DependsOn: blocker.ID})
	}); err != nil {
		t.Fatalf("adding dependency: %v", err)
	}

	row := func(token string) store.ClaimNextRow {
		return store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: token, TerminalStates: []string{"done"}, Statuses: []string{"todo"},
			ProjectIDs: []string{f.project.ID},
		}
	}

	var first string
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, row("t1"))
		if err != nil {
			return err
		}
		if !ok {
			t.Fatal("expected a claim")
		}
		first = id
		return nil
	}); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if first != free.ID {
		t.Fatalf("claimed %q, want the unblocked high-priority task %q", first, free.ID)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, row("t2"))
		if err != nil {
			return err
		}
		if !ok || id != blocker.ID {
			t.Fatalf("claimed %q ok=%v, want the blocker %q", id, ok, blocker.ID)
		}
		_, ok, err = tx.ClaimNextTask(ctx, row("t3"))
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("the blocked task must not be claimable while its dependency is unfinished")
		}
		return nil
	}); err != nil {
		t.Fatalf("second claim: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		done, err := tx.GetTask(ctx, core.TaskRef{ID: blocker.ID})
		if err != nil {
			return err
		}
		done.Status = "done"
		if err := tx.UpdateTask(ctx, done); err != nil {
			return err
		}
		id, ok, err := tx.ClaimNextTask(ctx, row("t4"))
		if err != nil {
			return err
		}
		if !ok || id != blocked.ID {
			t.Fatalf("claimed %q ok=%v, want the unblocked task %q", id, ok, blocked.ID)
		}
		return nil
	}); err != nil {
		t.Fatalf("third claim: %v", err)
	}
}

func TestClaimNextTaskFilters(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "labelled", core.PriorityNormal)

	label := core.Label{Name: "queue"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutLabel(ctx, &label); err != nil {
			return err
		}
		return tx.AttachLabel(ctx, task.ID, label.ID)
	}); err != nil {
		t.Fatalf("labelling: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: "l1", Labels: []string{"absent"}, TerminalStates: []string{"done"},
		})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a label filter that matches nothing must claim nothing")
		}
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: "l2", Labels: []string{"queue"}, TerminalStates: []string{"done"},
		})
		if err != nil {
			return err
		}
		if !ok || id != task.ID {
			t.Fatalf("label filter claimed %q ok=%v", id, ok)
		}
		return nil
	}); err != nil {
		t.Fatalf("label filtered claim: %v", err)
	}
}
