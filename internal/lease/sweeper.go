package lease

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

// DefaultInterval is how often a running sweeper materializes expiry.
const DefaultInterval = time.Minute

// DefaultBatch is how many expired leases one sweep pass materializes.
const DefaultBatch = 100

// DefaultBound is how long an opportunistic sweep may take before it gives up.
const DefaultBound = 250 * time.Millisecond

// SweepFunc materializes expired leases, reporting how many it swept.
type SweepFunc func(ctx context.Context, limit int) (int, error)

// Sweeper materializes lease expiry on a ticker. Expiry is authoritative
// without it; the sweeper only writes down what a lazy read already concludes.
type Sweeper struct {
	sweep    SweepFunc
	clock    clock.Clock
	interval time.Duration
	batch    int
	onError  func(error)
}

// Option configures a Sweeper.
type Option func(*Sweeper)

// WithClock sets the time source.
func WithClock(c clock.Clock) Option { return func(s *Sweeper) { s.clock = c } }

// WithInterval sets how often Run sweeps.
func WithInterval(d time.Duration) Option {
	return func(s *Sweeper) {
		if d > 0 {
			s.interval = d
		}
	}
}

// WithBatch sets how many leases one pass materializes.
func WithBatch(n int) Option {
	return func(s *Sweeper) {
		if n > 0 {
			s.batch = n
		}
	}
}

// WithErrorHandler sets what a running sweeper does with a failed pass.
func WithErrorHandler(fn func(error)) Option { return func(s *Sweeper) { s.onError = fn } }

// NewSweeper builds a sweeper over a sweep function.
func NewSweeper(sweep SweepFunc, opts ...Option) *Sweeper {
	s := &Sweeper{
		sweep:    sweep,
		clock:    clock.New(),
		interval: DefaultInterval,
		batch:    DefaultBatch,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Interval reports how often Run sweeps.
func (s *Sweeper) Interval() time.Duration { return s.interval }

// Once runs a single sweep pass.
func (s *Sweeper) Once(ctx context.Context) (int, error) {
	if s.sweep == nil {
		return 0, nil
	}
	return s.sweep(ctx, s.batch)
}

// Bounded runs one sweep pass that gives up after d, for callers that must not
// be delayed by it.
func (s *Sweeper) Bounded(ctx context.Context, d time.Duration) (int, error) {
	if d <= 0 {
		d = DefaultBound
	}
	bounded, cancel := context.WithTimeout(ctx, d)
	defer cancel()
	return s.Once(bounded)
}

// Run sweeps on the ticker until ctx is done.
func (s *Sweeper) Run(ctx context.Context) error {
	ticker := s.clock.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C():
			if _, err := s.Once(ctx); err != nil && s.onError != nil {
				s.onError(err)
			}
		}
	}
}
