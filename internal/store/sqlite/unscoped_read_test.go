// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// writerWait is how long the test that expects to be blocked is willing to
// wait. It is short because a pass here is a timeout, not a success.
const writerWait = 300 * time.Millisecond

// holdWrite opens a write transaction and keeps it open for the rest of the
// test, which is the state a cross-tenant read has to survive.
func holdWrite(t *testing.T, f fixture) {
	t.Helper()
	ctx := context.Background()
	held, err := f.store.Begin(ctx, f.scope)
	if err != nil {
		t.Fatalf("opening the write transaction to hold: %v", err)
	}
	t.Cleanup(func() { _ = held.Rollback() })
	task := core.Task{
		ProjectID:      f.project.ID,
		Title:          "held open",
		Status:         "todo",
		Priority:       core.PriorityNormal,
		CreatorActorID: f.actor.ID,
	}
	if err := held.CreateTask(ctx, &task); err != nil {
		t.Fatalf("writing inside the held transaction: %v", err)
	}
}

// seedDomain claims a hostname for the fixture's tenant.
func seedDomain(t *testing.T, f fixture, host string) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &core.Domain{Hostname: host})
	}); err != nil {
		t.Fatalf("adding domain %q: %v", host, err)
	}
}

// TestViewUnscopedReadsWhileAWriteIsOpen is the throughput bug, kept caught.
// Host resolution runs on every HTTP request and went through Unscoped, which
// takes the single writer connection and issues BEGIN IMMEDIATE; one open write
// therefore stalled every request until its own deadline. The reader pool has no
// such contention, so the read must land with a write still in flight.
func TestViewUnscopedReadsWhileAWriteIsOpen(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	seedDomain(t, f, "acme.example")
	holdWrite(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), acquireTimeout)
	defer cancel()
	var got *core.Tenant
	if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		v, err := u.ResolveDomain(ctx, "acme.example")
		got = v
		return err
	}); err != nil {
		t.Fatalf("resolving a host while a write is open: %v", err)
	}
	if got == nil || got.ID != f.tenant.ID {
		t.Fatalf("resolved acme.example to %+v, want tenant %q", got, f.tenant.ID)
	}
}

// TestUnscopedStillWaitsForTheWriter names the contention the read-only door
// avoids, so the previous test is read as a difference rather than as a fact
// about SQLite. Unscoped is a write transaction and must keep queueing behind
// the writer; if this ever passes, the two doors have become the same one and
// the read-only guard above proves nothing.
func TestUnscopedStillWaitsForTheWriter(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	seedDomain(t, f, "acme.example")
	holdWrite(t, f)

	ctx, cancel := context.WithTimeout(context.Background(), writerWait)
	defer cancel()
	err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		_, err := u.ResolveDomain(ctx, "acme.example")
		return err
	})
	if err == nil {
		t.Fatal("Unscoped resolved a host while a write was open; it no longer takes the writer connection")
	}
}
