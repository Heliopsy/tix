// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

var _ store.CommitWaker = (*Store)(nil)

func TestWakeOnCommitSignalsTheTenantsReader(t *testing.T) {
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
	case <-time.After(notifyTimeout):
		t.Fatal("the commit did not wake the reader")
	}
}

func TestWakeOnCommitDoesNotCrossTenants(t *testing.T) {
	s, clk := newStore(t)
	acme := seed(t, s, clk, "acme")
	other := seed(t, s, clk, "other")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake, err := s.WakeOnCommit(ctx, other.tenant.ID)
	if err != nil {
		t.Fatalf("WakeOnCommit: %v", err)
	}
	own, err := s.WakeOnCommit(ctx, acme.tenant.ID)
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
	case <-own:
	case <-time.After(notifyTimeout):
		t.Fatal("the commit did not wake its own tenant's reader")
	}
	select {
	case <-wake:
		t.Fatal("one tenant's commit woke another tenant's reader")
	default:
	}
}
