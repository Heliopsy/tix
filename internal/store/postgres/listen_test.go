// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// notifyTimeout only guards the test against hanging; the listener is woken by
// the commit, not by waiting.
const notifyTimeout = 30 * time.Second

func TestNotifyWakesAListenerOnCommit(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	task := f.newTask(t, "watched", core.PriorityNormal)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifications, err := s.Listen(ctx)
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: task.ID}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	select {
	case n, ok := <-notifications:
		if !ok {
			t.Fatal("the listener closed before delivering a notification")
		}
		if n.TenantID != f.tenant.ID {
			t.Fatalf("notification tenant = %q, want %q", n.TenantID, f.tenant.ID)
		}
	case <-time.After(notifyTimeout):
		t.Fatal("the commit did not wake the listener")
	}
}

func TestListenerStopsWhenTheContextEnds(t *testing.T) {
	s, _ := newStore(t)

	ctx, cancel := context.WithCancel(context.Background())
	notifications, err := s.Listen(ctx)
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	cancel()

	select {
	case _, ok := <-notifications:
		if ok {
			t.Fatal("a notification arrived after the context ended")
		}
	case <-time.After(notifyTimeout):
		t.Fatal("the listener did not stop when its context ended")
	}
}

func TestTransactionsWithoutEventsDoNotNotify(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	notifications, err := s.Listen(ctx)
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	f.newTask(t, "silent", core.PriorityNormal)
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		e := core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "x"}
		return tx.AppendEvent(ctx, &e)
	}); err != nil {
		t.Fatalf("appending an event: %v", err)
	}

	select {
	case n := <-notifications:
		if n.TenantID != f.tenant.ID {
			t.Fatalf("the first notification came from %q, so a silent transaction announced itself", n.TenantID)
		}
	case <-time.After(notifyTimeout):
		t.Fatal("the commit did not wake the listener")
	}
}
