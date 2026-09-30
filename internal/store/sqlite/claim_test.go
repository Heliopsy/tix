// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
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
		_, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "token"})
		return err
	}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	clk.Advance(2 * time.Minute)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		cleared, err := tx.ClearClaim(ctx, store.ExpireClaimRow{TaskID: task.ID, HolderID: f.actor.ID, At: clk.Now()})
		if err != nil {
			return err
		}
		if !cleared {
			t.Fatal("a lapsed lease should be cleared")
		}
		return nil
	}); err != nil {
		t.Fatalf("clearing a claim: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != "" {
			t.Fatalf("claim was not cleared: %+v", got)
		}
		if got.LeaseExpiredAt == nil || !got.LeaseExpiredAt.Equal(clk.Now()) {
			t.Fatalf("expiry evidence = %v, want %v", got.LeaseExpiredAt, clk.Now())
		}
		if got.LeaseExpiredByActorID != f.actor.ID {
			t.Fatalf("expiry evidence holder = %q, want %q", got.LeaseExpiredByActorID, f.actor.ID)
		}
		if _, err := tx.ClaimTask(ctx, store.ClaimRow{TaskID: task.ID, ActorID: f.actor.ID,
			Now: clk.Now(), Until: clk.Now().Add(time.Minute), LeaseToken: "again"}); err != nil {
			return err
		}
		reclaimed, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if reclaimed.LeaseExpiredAt != nil || reclaimed.LeaseExpiredByActorID != "" {
			t.Fatalf("a fresh claim kept the old evidence: %+v", reclaimed)
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
			LeaseToken: token, Terminal: terminalOf(f.workflow.ID, "done"), Statuses: []string{"todo"},
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
	task := f.newTask(t, "tagged", core.PriorityNormal)

	tag := core.Tag{Name: "queue"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutTag(ctx, &tag); err != nil {
			return err
		}
		return tx.AttachTag(ctx, task.ID, tag.ID)
	}); err != nil {
		t.Fatalf("labelling: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: "l1", Tags: []string{"absent"}, Terminal: terminalOf(f.workflow.ID, "done"),
		})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a tag filter that matches nothing must claim nothing")
		}
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
			LeaseToken: "l2", Tags: []string{"queue"}, Terminal: terminalOf(f.workflow.ID, "done"),
		})
		if err != nil {
			return err
		}
		if !ok || id != task.ID {
			t.Fatalf("tag filter claimed %q ok=%v", id, ok)
		}
		return nil
	}); err != nil {
		t.Fatalf("tag filtered claim: %v", err)
	}
}

func TestClaimNextSkipsTasksInATerminalState(t *testing.T) {
	cases := []struct {
		name     string
		terminal []string
		status   string
	}{
		{"done", []string{"done"}, "done"},
		{"cancelled", []string{"done", "cancelled"}, "cancelled"},
		{"custom terminal state", []string{"shipped"}, "shipped"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			s, clk := newStore(t)
			f := seed(t, s, clk, "acme")
			finished := f.newTask(t, "finished", core.PriorityHighest)
			setStatus(t, s, f, finished.ID, tc.status)

			row := func(token string) store.ClaimNextRow {
				return store.ClaimNextRow{
					ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Minute),
					LeaseToken: token, Terminal: terminalOf(f.workflow.ID, tc.terminal...),
					ProjectIDs: []string{f.project.ID},
				}
			}

			if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				id, ok, err := tx.ClaimNextTask(ctx, row("t1"))
				if err != nil {
					return err
				}
				if ok {
					t.Fatalf("a task in terminal status %q was handed out as %q", tc.status, id)
				}
				return nil
			}); err != nil {
				t.Fatalf("claiming from an exhausted queue: %v", err)
			}

			open := f.newTask(t, "open", core.PriorityLowest)
			if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				id, ok, err := tx.ClaimNextTask(ctx, row("t2"))
				if err != nil {
					return err
				}
				if !ok || id != open.ID {
					t.Fatalf("claimed %q ok=%v, want the unfinished task %q", id, ok, open.ID)
				}
				return nil
			}); err != nil {
				t.Fatalf("claiming past a terminal task: %v", err)
			}

			if err := s.View(ctx, f.scope, func(tx store.Tx) error {
				got, err := tx.GetTask(ctx, core.TaskRef{ID: finished.ID})
				if err != nil {
					return err
				}
				if got.ClaimedByActorID != "" || got.ClaimCount != 0 {
					t.Fatalf("finished task was touched: %+v", got)
				}
				return nil
			}); err != nil {
				t.Fatalf("reading the finished task: %v", err)
			}
		})
	}
}

func setStatus(t *testing.T, s *Store, f fixture, taskID, status string) {
	t.Helper()
	ctx := context.Background()
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		task, err := tx.GetTask(ctx, core.TaskRef{ID: taskID})
		if err != nil {
			return err
		}
		task.Status = status
		return tx.UpdateTask(ctx, task)
	}); err != nil {
		t.Fatalf("setting status %q: %v", status, err)
	}
}

// terminalOf names one workflow's terminal states the way the claim queries
// take them: terminal is a property of a workflow, never of a state name.
func terminalOf(workflowID string, states ...string) []store.WorkflowTerminal {
	return []store.WorkflowTerminal{{WorkflowID: workflowID, States: states}}
}

// The interleave that guard covers cannot be produced on this engine: a write
// transaction takes the single writer connection and issues BEGIN IMMEDIATE, so
// a second writer cannot commit between the sweeper's listing and its clear.
// The predicate itself is still asserted here, and the engines stay identical
// from the service's side.

// TestClearClaimRefusesALiveLease is the predicate on its own, without the
// interleave: a lease that has not lapsed is not the sweeper's to clear, and
// saying so is distinct from saying the task is not there.
func TestClearClaimRefusesALiveLease(t *testing.T) {
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
		cleared, err := tx.ClearClaim(ctx, store.ExpireClaimRow{
			TaskID: task.ID, HolderID: f.actor.ID, At: clk.Now()})
		if err != nil {
			return err
		}
		if cleared {
			t.Fatal("a live lease must not be cleared")
		}
		if _, err := tx.ClearClaim(ctx, store.ExpireClaimRow{
			TaskID: "01J0000000000000000000000X", At: clk.Now()}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("clearing a claim on a task that is not there = %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatalf("clearing a live claim: %v", err)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if got.ClaimedByActorID != f.actor.ID || got.LeaseExpiredAt != nil {
			t.Fatalf("a refused clear changed the task: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the held task: %v", err)
	}
}

// TestClaimNextJudgesTerminalPerWorkflow keeps the engines honest about the
// shape of the predicate: one query carries two workflows that disagree about
// "done", and each row is judged by the workflow its own project names.
func TestClaimNextJudgesTerminalPerWorkflow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	ops := f.secondProject(t, "ops", "shipped")

	agentsDone := f.newTask(t, "finished under agents", core.PriorityHighest)
	setStatus(t, s, f, agentsDone.ID, "done")
	opsDone := f.newTaskIn(t, ops, "still going under ops", core.PriorityHigh)
	setStatus(t, s, f, opsDone.ID, "done")

	terminal := []store.WorkflowTerminal{
		{WorkflowID: f.workflow.ID, States: []string{"done"}},
		{WorkflowID: ops.WorkflowID, States: []string{"shipped"}},
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: "t1", Terminal: terminal,
		})
		if err != nil {
			return err
		}
		if !ok || id != opsDone.ID {
			t.Fatalf("claimed %q ok=%v, want %q, whose own workflow does not call \"done\" finished",
				id, ok, opsDone.ID)
		}
		_, ok, err = tx.ClaimNextTask(ctx, store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: "t2", Terminal: terminal,
		})
		if err != nil {
			return err
		}
		if ok {
			t.Fatal("a task terminal under its own workflow was handed out")
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming across two workflows: %v", err)
	}
}

// TestClaimNextBlocksOnADependencyUnfinishedInItsOwnWorkflow does the same for
// the dependency gate, where the row judged is in a different project from the
// task waiting on it.
func TestClaimNextBlocksOnADependencyUnfinishedInItsOwnWorkflow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	ops := f.secondProject(t, "ops", "shipped")

	blocker := f.newTaskIn(t, ops, "upstream", core.PriorityNormal)
	setStatus(t, s, f, blocker.ID, "done")
	blocked := f.newTask(t, "downstream", core.PriorityHighest)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDependency(ctx, &core.Dependency{TaskID: blocked.ID, DependsOn: blocker.ID})
	}); err != nil {
		t.Fatalf("seeding the dependency: %v", err)
	}

	terminal := []store.WorkflowTerminal{
		{WorkflowID: f.workflow.ID, States: []string{"done"}},
		{WorkflowID: ops.WorkflowID, States: []string{"shipped"}},
	}
	row := func(token string) store.ClaimNextRow {
		return store.ClaimNextRow{
			ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
			LeaseToken: token, Terminal: terminal, ProjectIDs: []string{f.project.ID},
		}
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, row("t1"))
		if err != nil {
			return err
		}
		if ok {
			t.Fatalf("claimed %q, whose dependency is unfinished under the dependency's own workflow", id)
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming over an unfinished dependency: %v", err)
	}

	setStatus(t, s, f, blocker.ID, "shipped")
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		id, ok, err := tx.ClaimNextTask(ctx, row("t2"))
		if err != nil {
			return err
		}
		if !ok || id != blocked.ID {
			t.Fatalf("claimed %q ok=%v, want %q once its dependency reached its own workflow's terminal state",
				id, ok, blocked.ID)
		}
		return nil
	}); err != nil {
		t.Fatalf("claiming once the dependency finished: %v", err)
	}
}
