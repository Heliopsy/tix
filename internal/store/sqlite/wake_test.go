// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// wakeTimeout only keeps the test from hanging. An in-process wake costs a
// channel send, so anything near this bound is a failure rather than a slow
// machine.
const wakeTimeout = 10 * time.Second

var _ store.CommitWaker = (*Store)(nil)

func TestCommitWakesAnInProcessReader(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake, err := s.WakeOnCommit(ctx, f.tenant.ID)
	if err != nil {
		t.Fatalf("WakeOnCommit: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	select {
	case <-wake:
	case <-time.After(wakeTimeout):
		t.Fatal("the commit did not wake the reader")
	}
}

func TestATransactionWithoutEventsDoesNotWake(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake, err := s.WakeOnCommit(ctx, f.tenant.ID)
	if err != nil {
		t.Fatalf("WakeOnCommit: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		p := core.Project{Key: "quiet", Name: "Quiet", WorkflowID: f.workflow.ID}
		return tx.CreateProject(ctx, &p)
	}); err != nil {
		t.Fatalf("creating a project: %v", err)
	}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	select {
	case <-wake:
	case <-time.After(wakeTimeout):
		t.Fatal("the commit did not wake the reader")
	}
	select {
	case <-wake:
		t.Fatal("a second wake arrived, so a transaction carrying no events announced itself")
	case <-time.After(100 * time.Millisecond):
	}
}

func TestAWakeReachesOnlyItsOwnTenant(t *testing.T) {
	s, clk := newStore(t)
	acme := seed(t, s, clk, "acme")
	other := seed(t, s, clk, "other")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake, err := s.WakeOnCommit(ctx, other.tenant.ID)
	if err != nil {
		t.Fatalf("WakeOnCommit: %v", err)
	}

	if err := s.Update(ctx, acme.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	select {
	case <-wake:
		t.Fatal("one tenant's commit woke another tenant's reader")
	case <-time.After(100 * time.Millisecond):
	}
}
