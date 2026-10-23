// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"testing"
	"time"
)

func TestWakeReachesOnlyTheTenantsReceivers(t *testing.T) {
	w := NewWakers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	acme := w.Add(ctx, "acme")
	other := w.Add(ctx, "other")
	w.Wake("acme")

	select {
	case <-acme:
	default:
		t.Fatal("the tenant's own receiver was not signalled")
	}
	select {
	case <-other:
		t.Fatal("another tenant's receiver was signalled")
	default:
	}
}

// Signals coalesce deliberately: a receiver that has not yet read the first one
// learns nothing from a second, and a full buffer must never block the
// committing transaction.
func TestRepeatedWakesCoalesceAndNeverBlock(t *testing.T) {
	w := NewWakers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := w.Add(ctx, "acme")
	for range 100 {
		w.Wake("acme")
	}

	select {
	case <-ch:
	default:
		t.Fatal("no signal arrived")
	}
	select {
	case <-ch:
		t.Fatal("a coalesced signal was buffered twice")
	default:
	}
}

func TestAReceiverIsForgottenWhenItsContextEnds(t *testing.T) {
	w := NewWakers()
	ctx, cancel := context.WithCancel(context.Background())
	w.Add(ctx, "acme")
	cancel()

	deadline := time.Now().Add(10 * time.Second)
	for {
		w.mu.Lock()
		n := len(w.byTenant)
		w.mu.Unlock()
		if n == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the receiver outlived its context, so a store accumulates them")
		}
		time.Sleep(time.Millisecond)
	}
}
