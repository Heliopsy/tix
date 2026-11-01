// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// wakeCounter is a service that reports each wake pass on a channel.
type wakeCounter struct {
	core.Service
	woke chan string
}

func (w wakeCounter) WakeSnoozed(ctx context.Context, _ int) (int, error) {
	actor, _ := core.ActorFrom(ctx)
	if actor == nil {
		w.woke <- ""
		return 0, nil
	}
	w.woke <- actor.ID
	return 0, nil
}

// The wake pass rides the sweep interval and the sweep switch: a server that
// sweeps leases also announces woken snoozes, as the system actor, and one
// told not to sweep does neither.
func TestTheSweepLoopRunsAWakePass(t *testing.T) {
	clk := clock.NewFakeAt()
	svc := wakeCounter{woke: make(chan string, 4)}
	opts := Options{Service: svc, Clock: clk, SweepInterval: time.Minute, Logger: slog.Default(),
		TenantID: "t1", DisablePrune: true, DisableDispatch: true}

	var waker Worker
	for _, w := range workersFor(opts) {
		if w.Name() == "snooze-waker" {
			waker = w
		}
	}
	if waker == nil {
		t.Fatal("no snooze-waker worker beside the lease sweeper")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- waker.Run(ctx) }()
	for clk.Tickers() == 0 {
		select {
		case err := <-done:
			t.Fatalf("the waker stopped before ticking: %v", err)
		default:
		}
	}
	clk.Advance(time.Minute)
	if who, want := <-svc.woke, core.SystemActor("t1").ID; who != want {
		t.Fatalf("the wake pass ran as %q, want the system actor %q", who, want)
	}
	cancel()
	<-done

	opts.DisableSweep = true
	for _, w := range workersFor(opts) {
		if w.Name() == "snooze-waker" {
			t.Fatal("a server told not to sweep still wakes snoozes")
		}
	}
}
