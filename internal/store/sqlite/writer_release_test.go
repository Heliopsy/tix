// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// acquireTimeout bounds a write that a wedged writer pool would otherwise
// block on until the test binary's own deadline.
const acquireTimeout = 3 * time.Second

// mustWriteAfter fails when the writer pool can no longer hand out its one
// connection, which is how every wedge in this file shows itself.
func mustWriteAfter(t *testing.T, f fixture, what string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), acquireTimeout)
	defer cancel()
	task := core.Task{
		ProjectID:      f.project.ID,
		Title:          "after-" + what,
		Status:         "todo",
		Priority:       core.PriorityNormal,
		CreatorActorID: f.actor.ID,
	}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("writing after %s: the writer is wedged: %v", what, err)
	}
}

// TestPanicInUpdateReleasesTheWriterConnection pins the zombie: the HTTP
// middleware recovers a panic and returns 500, so the process survives, and
// without a deferred release the single writer connection never returns to
// the pool and every later write blocks until its deadline.
func TestPanicInUpdateReleasesTheWriterConnection(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	func() {
		defer func() {
			if recover() == nil {
				t.Errorf("the panic did not propagate out of Update")
			}
		}()
		_ = s.Update(context.Background(), f.scope, func(store.Tx) error {
			panic("boom")
		})
	}()

	mustWriteAfter(t, f, "a panicking Update")
}

// TestPanicInUnscopedReleasesTheWriterConnection covers the other write entry
// point, which takes the same single connection.
func TestPanicInUnscopedReleasesTheWriterConnection(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	func() {
		defer func() { _ = recover() }()
		_ = s.Unscoped(context.Background(), func(store.UnscopedTx) error {
			panic("boom")
		})
	}()

	mustWriteAfter(t, f, "a panicking Unscoped")
}

// TestPanicInViewReleasesTheReaderConnection drains the reader pool one panic
// at a time; without a deferred release the pool runs dry and a later read
// blocks on a connection that will never come back.
func TestPanicInViewReleasesTheReaderConnection(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	for i := 0; i < readerConns()+2; i++ {
		func() {
			defer func() { _ = recover() }()
			loop, cancel := context.WithTimeout(context.Background(), acquireTimeout)
			defer cancel()
			_ = s.View(loop, f.scope, func(store.Tx) error {
				panic("boom")
			})
		}()
	}

	ctx, cancel := context.WithTimeout(context.Background(), acquireTimeout)
	defer cancel()
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		_, err := tx.ListTasks(ctx, core.TaskFilter{})
		return err
	}); err != nil {
		t.Fatalf("reading after panicking views: the reader pool is drained: %v", err)
	}
}

// TestFailedCommitReleasesTheWriterConnection pins the sibling of the panic
// wedge: a COMMIT can fail with the transaction still open, a deferred
// constraint being the usual cause, and handing that connection back to the
// single-slot writer pool leaves every later BEGIN IMMEDIATE facing a
// transaction that is already running.
func TestFailedCommitReleasesTheWriterConnection(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	for _, stmt := range []string{
		`CREATE TABLE deferred_parent (id INTEGER PRIMARY KEY)`,
		`CREATE TABLE deferred_child (id INTEGER PRIMARY KEY,
			parent_id INTEGER REFERENCES deferred_parent(id) DEFERRABLE INITIALLY DEFERRED)`,
	} {
		if _, err := s.writer.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("preparing the deferred constraint: %v", err)
		}
	}

	wtx, err := s.beginWrite(ctx, f.scope)
	if err != nil {
		t.Fatalf("beginning the write transaction: %v", err)
	}
	if _, err := wtx.conn.ExecContext(ctx,
		`INSERT INTO deferred_child (id, parent_id) VALUES (1, 404)`); err != nil {
		t.Fatalf("inserting the row that violates the deferred constraint: %v", err)
	}
	if err := wtx.Commit(); err == nil {
		t.Fatal("Commit() succeeded although a deferred foreign key was violated")
	}

	mustWriteAfter(t, f, "a failed commit")
}
