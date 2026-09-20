package retention

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
)

// fakeRunner records every prune and signals when one has happened.
type fakeRunner struct {
	calls  atomic.Int64
	limits chan int
	fail   atomic.Bool
}

func newFakeRunner() *fakeRunner { return &fakeRunner{limits: make(chan int, 8)} }

var errPrune = errors.New("prune failed")

func (f *fakeRunner) Prune(_ context.Context, in core.PruneInput) (*core.PruneResult, error) {
	f.calls.Add(1)
	f.limits <- in.Limit
	if f.fail.Load() {
		return nil, errPrune
	}
	return &core.PruneResult{Events: 1}, nil
}

func TestPrunerRunsOnItsInterval(t *testing.T) {
	r := newFakeRunner()
	clk := clock.NewFakeAt()
	results := make(chan *core.PruneResult, 4)
	p := NewPruner(r, clk, core.Duration(time.Hour),
		WithLimit(25), WithResultHandler(func(res *core.PruneResult) { results <- res }))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	clk.Advance(time.Hour)
	if got := <-r.limits; got != 25 {
		t.Errorf("run limit = %d, want 25", got)
	}
	if res := <-results; res.Events != 1 {
		t.Errorf("reported result = %+v", res)
	}

	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run returned %v, want context cancellation", err)
	}
}

// A failed run is reported and the worker keeps going, so pruning never takes
// the server down with it.
func TestPrunerSurvivesAFailedRun(t *testing.T) {
	r := newFakeRunner()
	r.fail.Store(true)
	clk := clock.NewFakeAt()
	failures := make(chan error, 4)
	p := NewPruner(r, clk, core.Duration(time.Minute),
		WithErrorHandler(func(err error) { failures <- err }))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() { _ = p.Run(ctx) }()

	clk.Advance(time.Minute)
	<-r.limits
	if err := <-failures; !errors.Is(err, errPrune) {
		t.Fatalf("reported %v, want the prune failure", err)
	}

	r.fail.Store(false)
	clk.Advance(time.Minute)
	<-r.limits
	if got := r.calls.Load(); got < 2 {
		t.Errorf("worker stopped after a failure; ran %d times", got)
	}
}

func TestPrunerDisabledByANonPositiveInterval(t *testing.T) {
	r := newFakeRunner()
	clk := clock.NewFakeAt()
	p := NewPruner(r, clk, 0)

	if p.Enabled() {
		t.Error("a zero interval left the pruner enabled")
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()

	clk.Advance(24 * time.Hour)
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Errorf("Run returned %v, want context cancellation", err)
	}
	if got := r.calls.Load(); got != 0 {
		t.Errorf("a disabled pruner ran %d times", got)
	}

	if _, err := p.RunOnce(context.Background()); err != nil {
		t.Errorf("on-demand pruning still has to work: %v", err)
	}
	p.Stop()
}

func TestPrunerRunOnce(t *testing.T) {
	r := newFakeRunner()
	p := NewPruner(r, clock.NewFakeAt(), core.Duration(time.Hour), WithLimit(7))
	defer p.Stop()

	res, err := p.RunOnce(context.Background())
	if err != nil {
		t.Fatalf("RunOnce: %v", err)
	}
	if res.Events != 1 {
		t.Errorf("result = %+v", res)
	}
	if got := <-r.limits; got != 7 {
		t.Errorf("limit = %d, want 7", got)
	}
}
