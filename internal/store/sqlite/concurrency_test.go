package sqlite

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
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

// TestConcurrentWritersAcrossPoolsSerialize writes through two independent
// Stores on one file, which is the goroutine-level stand-in for two processes.
func TestConcurrentWritersAcrossPoolsSerialize(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFakeAt()
	path := filepath.Join(t.TempDir(), "tix.db")

	first, err := Open(path, clk)
	if err != nil {
		t.Fatalf("opening the first store: %v", err)
	}
	defer func() { _ = first.Close() }()
	if err := first.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	f := seed(t, first, clk, "acme")

	second, err := Open(path, clk)
	if err != nil {
		t.Fatalf("opening the second store: %v", err)
	}
	defer func() { _ = second.Close() }()

	const perStore = 12
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		fails []error
		start = make(chan struct{})
	)
	for _, s := range []*Store{first, second} {
		for i := 0; i < perStore; i++ {
			wg.Add(1)
			go func(s *Store, n int) {
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
			}(s, i)
		}
	}
	close(start)
	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("%d cross-pool writes failed, first: %v", len(fails), fails[0])
	}
	if err := first.View(ctx, f.scope, func(tx store.Tx) error {
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
