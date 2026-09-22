package postgres

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func hardDelete(t *testing.T, f fixture, taskID string) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.DeleteTask(ctx, taskID, true)
	}); err != nil {
		t.Fatalf("hard deleting task %q: %v", taskID, err)
	}
}

func nextSeq(t *testing.T, f fixture, projectID string) int64 {
	t.Helper()
	ctx := context.Background()
	var seq int64
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		var err error
		seq, err = tx.NextSeq(ctx, projectID)
		return err
	}); err != nil {
		t.Fatalf("reserving a task number: %v", err)
	}
	return seq
}

func TestTaskNumberIsNotReusedAfterHardDeletingTheHighest(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	first := f.newTask(t, "one", core.PriorityNormal)
	second := f.newTask(t, "two", core.PriorityNormal)
	if first.Seq != 1 || second.Seq != 2 {
		t.Fatalf("seqs = %d, %d, want 1, 2", first.Seq, second.Seq)
	}

	hardDelete(t, f, second.ID)

	third := f.newTask(t, "three", core.PriorityNormal)
	if third.Seq == second.Seq {
		t.Fatalf("task number %d was reused after a hard delete", third.Seq)
	}
	if third.Seq != 3 {
		t.Fatalf("seq = %d, want 3", third.Seq)
	}
	if third.Ref != "alpha-3" {
		t.Fatalf("ref = %q, want %q", third.Ref, "alpha-3")
	}
}

func TestTaskNumberingContinuesAfterHardDeletingEveryTask(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	for i := 0; i < 3; i++ {
		task := f.newTask(t, fmt.Sprintf("task-%d", i), core.PriorityNormal)
		hardDelete(t, f, task.ID)
	}

	next := f.newTask(t, "after", core.PriorityNormal)
	if next.Seq != 4 {
		t.Fatalf("seq = %d, want 4: numbering restarted after the project was emptied", next.Seq)
	}
}

func TestConcurrentAllocationsAreDistinct(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	const writers = 32
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		seqs  = map[int64]int{}
		fails []error
		start = make(chan struct{})
	)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			<-start
			var seq int64
			err := s.Update(ctx, f.scope, func(tx store.Tx) error {
				task := core.Task{ProjectID: f.project.ID, Title: fmt.Sprintf("task-%d", n),
					Status: "todo", CreatorActorID: f.actor.ID}
				if err := tx.CreateTask(ctx, &task); err != nil {
					return err
				}
				seq = task.Seq
				return nil
			})
			mu.Lock()
			defer mu.Unlock()
			if err != nil {
				fails = append(fails, err)
				return
			}
			seqs[seq]++
		}(i)
	}
	close(start)
	wg.Wait()

	if len(fails) > 0 {
		t.Fatalf("%d of %d allocations failed, first: %v", len(fails), writers, fails[0])
	}
	if len(seqs) != writers {
		t.Fatalf("%d distinct task numbers for %d writers: %v", len(seqs), writers, seqs)
	}
}

// TestMigrationResumesAboveExistingTasks rewinds a migrated database to the
// state it had before the counter existed, then migrates it again, which is
// what an upgrade of a database already holding tasks does.
func TestMigrationResumesAboveExistingTasks(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	var highest int64
	for i := 0; i < 5; i++ {
		highest = f.newTask(t, fmt.Sprintf("task-%d", i), core.PriorityNormal).Seq
	}
	// Rewinding means undoing every migration above the first, or the runner
	// would skip the one under test: it replays from the highest version
	// recorded, not from each missing one.
	for _, stmt := range []string{
		"ALTER TABLE projects DROP COLUMN seq_counter",
		"ALTER TABLE projects DROP COLUMN color",
		"ALTER TABLE projects DROP COLUMN icon",
		"DROP INDEX idx_tasks_urgency",
		"DROP TABLE ssh_keys",
		"DELETE FROM schema_migrations WHERE version > 1",
	} {
		if _, err := s.db.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("rewinding the schema (%s): %v", stmt, err)
		}
	}

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating a database that already holds tasks: %v", err)
	}
	if got := nextSeq(t, f, f.project.ID); got != highest+1 {
		t.Fatalf("next number after migration = %d, want %d", got, highest+1)
	}
}

func TestProjectsNumberIndependently(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	other := core.Project{Key: "beta", Name: "Beta", WorkflowID: f.workflow.ID}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateProject(ctx, &other)
	}); err != nil {
		t.Fatalf("creating the second project: %v", err)
	}

	f.newTask(t, "alpha one", core.PriorityNormal)
	f.newTask(t, "alpha two", core.PriorityNormal)

	if got := nextSeq(t, f, other.ID); got != 1 {
		t.Fatalf("first number of the second project = %d, want 1", got)
	}
	if got := nextSeq(t, f, f.project.ID); got != 3 {
		t.Fatalf("next number of the first project = %d, want 3", got)
	}
}

func TestNextSeqRejectsAnUnknownProject(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.NextSeq(ctx, "missing")
		return err
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("NextSeq on a missing project = %v, want not found", err)
	}
}
