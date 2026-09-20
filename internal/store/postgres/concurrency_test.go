package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

func TestConcurrentWritersAllSucceed(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	const writers = 24
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		fails []error
		start = make(chan struct{})
	)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				task := core.Task{ProjectID: f.project.ID, Title: fmt.Sprintf("task-%d", n),
					Status: "todo", Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
				return tx.CreateTask(ctx, &task)
			})
			if err != nil {
				mu.Lock()
				fails = append(fails, err)
				mu.Unlock()
			}
		}(i)
	}
	close(start)
	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("%d of %d concurrent writes failed, first: %v", len(fails), writers, fails[0])
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return err
		}
		if len(tasks) != writers {
			t.Fatalf("tasks = %d, want %d", len(tasks), writers)
		}
		seqs := map[int64]bool{}
		for _, task := range tasks {
			if seqs[task.Seq] {
				t.Fatalf("two tasks share sequence number %d", task.Seq)
			}
			seqs[task.Seq] = true
		}
		return nil
	}); err != nil {
		t.Fatalf("counting tasks: %v", err)
	}
}

// TestConcurrentWritersAcrossPoolsSucceed writes through two independent Stores
// on one database, which is the goroutine-level stand-in for two processes.
func TestConcurrentWritersAcrossPoolsSucceed(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	second := openStore(t, s.dsn, clk)

	const perStore = 12
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		fails []error
		start = make(chan struct{})
	)
	for _, target := range []*Store{s, second} {
		for i := 0; i < perStore; i++ {
			wg.Add(1)
			go func(target *Store, n int) {
				defer wg.Done()
				<-start
				err := target.Update(ctx, f.scope, func(tx store.Tx) error {
					task := core.Task{ProjectID: f.project.ID, Title: fmt.Sprintf("task-%d", n),
						Status: "todo", Priority: core.PriorityNormal, CreatorActorID: f.actor.ID}
					return tx.CreateTask(ctx, &task)
				})
				if err != nil {
					mu.Lock()
					fails = append(fails, err)
					mu.Unlock()
				}
			}(target, i)
		}
	}
	close(start)
	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("%d cross-pool writes failed, first: %v", len(fails), fails[0])
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		tasks, err := tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return err
		}
		if len(tasks) != 2*perStore {
			t.Fatalf("tasks = %d, want %d", len(tasks), 2*perStore)
		}
		return nil
	}); err != nil {
		t.Fatalf("counting tasks: %v", err)
	}
}

func TestConcurrentClaimNextTaskNeverDoubleClaims(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	const tasks = 8
	const workers = 12
	for i := 0; i < tasks; i++ {
		f.newTask(t, fmt.Sprintf("queued-%d", i), core.PriorityNormal)
		clk.Advance(time.Second)
	}

	type outcome struct {
		taskID string
		ok     bool
		err    error
	}
	results := make([]outcome, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				id, ok, err := tx.ClaimNextTask(ctx, store.ClaimNextRow{
					ActorID: f.actor.ID, Now: clk.Now(), Until: clk.Now().Add(time.Hour),
					LeaseToken: fmt.Sprintf("worker-%d", n), TerminalStates: []string{"done"},
				})
				if err != nil {
					return err
				}
				results[n] = outcome{taskID: id, ok: ok}
				return nil
			})
			if err != nil {
				results[n] = outcome{err: err}
			}
		}(i)
	}
	close(start)
	wg.Wait()

	claimed := map[string]int{}
	successes := 0
	for i, r := range results {
		if r.err != nil {
			t.Fatalf("worker %d failed: %v", i, r.err)
		}
		if !r.ok {
			continue
		}
		successes++
		claimed[r.taskID]++
		if claimed[r.taskID] > 1 {
			t.Fatalf("task %q was claimed twice", r.taskID)
		}
	}
	if successes != tasks {
		t.Fatalf("successful claims = %d, want %d", successes, tasks)
	}
	if len(claimed) != tasks {
		t.Fatalf("distinct claimed tasks = %d, want %d", len(claimed), tasks)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		all, err := tx.ListTasks(ctx, core.TaskFilter{Page: core.Page{Limit: core.MaxPageLimit}})
		if err != nil {
			return err
		}
		for _, task := range all {
			if task.ClaimCount != 1 {
				t.Fatalf("task %q has claim count %d, want 1", task.ID, task.ClaimCount)
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying claims: %v", err)
	}
}

func TestConcurrentClaimTaskOnOneTaskHasOneWinner(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "contended", core.PriorityNormal)

	const workers = 16
	wins := make([]bool, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				ok, err := tx.ClaimTask(ctx, store.ClaimRow{
					TaskID: task.ID, ActorID: f.actor.ID, Now: clk.Now(),
					Until: clk.Now().Add(time.Hour), LeaseToken: fmt.Sprintf("token-%d", n),
				})
				if err != nil {
					return err
				}
				wins[n] = ok
				return nil
			})
			if err != nil {
				t.Errorf("worker %d: %v", n, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	won := 0
	for _, w := range wins {
		if w {
			won++
		}
	}
	if won != 1 {
		t.Fatalf("winners = %d, want exactly 1", won)
	}
}

func TestConcurrentDeliveryClaimsDoNotOverlap(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	endpoint := core.WebhookEndpoint{URL: "https://example.test/hook", Secret: "s", Active: true}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWebhook(ctx, &endpoint); err != nil {
			return err
		}
		for i := 0; i < 6; i++ {
			d := core.WebhookDelivery{EndpointID: endpoint.ID, EventSeq: int64(i + 1)}
			if err := tx.EnqueueDelivery(ctx, &d); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("queueing deliveries: %v", err)
	}

	const workers = 4
	claimed := make([][]core.WebhookDelivery, workers)
	var wg sync.WaitGroup
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				got, err := tx.ClaimDeliveries(ctx, fmt.Sprintf("owner-%d", n),
					clk.Now(), clk.Now().Add(time.Minute), 2)
				if err != nil {
					return err
				}
				claimed[n] = got
				return nil
			})
			if err != nil {
				t.Errorf("worker %d: %v", n, err)
			}
		}(i)
	}
	close(start)
	wg.Wait()

	seen := map[string]bool{}
	for _, batch := range claimed {
		for _, d := range batch {
			if seen[d.ID] {
				t.Fatalf("delivery %q was claimed by two owners", d.ID)
			}
			seen[d.ID] = true
		}
	}
}
