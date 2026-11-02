// SPDX-License-Identifier: AGPL-3.0-or-later

package retention

import (
	"context"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// Runner prunes a tenant's expired records once.
type Runner interface {
	Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error)
}

// Housekeeper applies a tenant's housekeeping policy once.
type Housekeeper interface {
	RunHousekeeping(ctx context.Context, in core.HousekeepingRunInput) (*core.HousekeepingResult, error)
}

// Pruner prunes on a ticker until its context is cancelled.
type Pruner struct {
	runner  Runner
	keeper  Housekeeper
	ticker  clock.Ticker
	enabled bool
	limit   int
	onError func(error)
	onRun   func(*core.PruneResult)
}

// PrunerOption configures a Pruner.
type PrunerOption func(*Pruner)

// WithLimit bounds how many rows per class one run removes.
func WithLimit(n int) PrunerOption { return func(p *Pruner) { p.limit = n } }

// WithHousekeeper runs the housekeeping policy after every prune on the same
// tick, so tidying tasks needs no loop of its own.
func WithHousekeeper(h Housekeeper) PrunerOption { return func(p *Pruner) { p.keeper = h } }

// WithErrorHandler reports a failed run. A failed run never stops the loop.
func WithErrorHandler(fn func(error)) PrunerOption { return func(p *Pruner) { p.onError = fn } }

// WithResultHandler reports the outcome of every successful run.
func WithResultHandler(fn func(*core.PruneResult)) PrunerOption {
	return func(p *Pruner) { p.onRun = fn }
}

// NewPruner builds a background pruner. A non-positive interval disables it,
// leaving on-demand pruning the only path. The ticker is armed here so a
// caller can schedule work before Run starts.
func NewPruner(r Runner, clk clock.Clock, interval core.Duration, opts ...PrunerOption) *Pruner {
	p := &Pruner{runner: r, enabled: interval.D() > 0}
	for _, opt := range opts {
		opt(p)
	}
	if p.enabled {
		p.ticker = clk.NewTicker(interval.D())
	}
	return p
}

// Enabled reports whether the pruner will do anything when run.
func (p *Pruner) Enabled() bool { return p.enabled }

// Stop releases the ticker.
func (p *Pruner) Stop() {
	if p.ticker != nil {
		p.ticker.Stop()
	}
}

// Run prunes on every tick until ctx is cancelled. A failed run is reported
// and the loop continues, so pruning never takes the server down with it.
func (p *Pruner) Run(ctx context.Context) error {
	if !p.enabled {
		<-ctx.Done()
		return ctx.Err()
	}
	defer p.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-p.ticker.C():
			p.tick(ctx)
		}
	}
}

// tick prunes, then housekeeps. A failure of either is reported and never
// stops the other.
func (p *Pruner) tick(ctx context.Context) {
	res, err := p.RunOnce(ctx)
	switch {
	case err != nil:
		p.report(err)
	case p.onRun != nil:
		p.onRun(res)
	}
	if p.keeper == nil {
		return
	}
	if _, err := p.keeper.RunHousekeeping(ctx, core.HousekeepingRunInput{Limit: p.limit}); err != nil {
		p.report(err)
	}
}

// report hands a failed run to the error handler.
func (p *Pruner) report(err error) {
	if p.onError != nil {
		p.onError(err)
	}
}

// RunOnce prunes immediately, regardless of the interval.
func (p *Pruner) RunOnce(ctx context.Context) (*core.PruneResult, error) {
	return p.runner.Prune(ctx, core.PruneInput{Limit: p.limit})
}
