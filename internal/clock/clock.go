// Package clock provides an injectable time source.
//
// Leases, retention windows and webhook backoff are all time-driven, and tests
// for them must not depend on wall-clock timing. Sleeping to let a lease expire
// makes a suite slow and flaky; advancing a fake clock makes it instant and
// deterministic. Every package that reads the time takes a Clock.
package clock

import (
	"sync"
	"time"
)

// Clock is a source of time.
type Clock interface {
	// Now returns the current time in UTC.
	Now() time.Time
	// Since returns the time elapsed since t.
	Since(t time.Time) time.Duration
	// NewTicker returns a ticker that fires on this clock.
	NewTicker(d time.Duration) Ticker
	// After returns a channel that receives once d has elapsed.
	After(d time.Duration) <-chan time.Time
	// Sleep blocks for d.
	Sleep(d time.Duration)
}

// Ticker fires repeatedly. It mirrors time.Ticker so real and fake tickers are
// interchangeable at a call site.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Real is a Clock backed by the system clock.
type Real struct{}

// New returns a real clock.
func New() Real { return Real{} }

// Now returns the current UTC time. Times are normalized to UTC because SQLite
// stores them as RFC3339 strings, where a non-UTC offset would break the
// lexicographic ordering that lease and cursor comparisons depend on.
func (Real) Now() time.Time { return time.Now().UTC() }

// Since returns the time elapsed since t.
func (Real) Since(t time.Time) time.Duration { return time.Since(t) }

// After returns a channel that receives once d has elapsed.
func (Real) After(d time.Duration) <-chan time.Time { return time.After(d) }

// Sleep blocks for d.
func (Real) Sleep(d time.Duration) { time.Sleep(d) }

// NewTicker returns a ticker driven by the system clock.
func (Real) NewTicker(d time.Duration) Ticker { return &realTicker{t: time.NewTicker(d)} }

type realTicker struct{ t *time.Ticker }

func (r *realTicker) C() <-chan time.Time { return r.t.C }
func (r *realTicker) Stop()               { r.t.Stop() }

// Fake is a Clock under test control. It is safe for concurrent use, so a test
// may advance it while goroutines read it.
type Fake struct {
	mu      sync.Mutex
	now     time.Time
	waiters []*waiter
	tickers []*fakeTicker
}

type waiter struct {
	at time.Time
	ch chan time.Time
}

// NewFake returns a fake clock set to t, normalized to UTC.
func NewFake(t time.Time) *Fake { return &Fake{now: t.UTC()} }

// NewFakeAt returns a fake clock set to a fixed, arbitrary instant. Use it when
// a test needs a clock but does not care what time it reads.
func NewFakeAt() *Fake {
	return NewFake(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC))
}

// Now returns the fake clock's current time.
func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// Since returns the elapsed time according to the fake clock.
func (f *Fake) Since(t time.Time) time.Duration { return f.Now().Sub(t) }

// Set moves the clock to t, firing anything due at or before it.
func (f *Fake) Set(t time.Time) {
	f.mu.Lock()
	f.now = t.UTC()
	due := f.collectDueLocked()
	f.mu.Unlock()
	fire(due)
}

// Advance moves the clock forward by d, firing anything that becomes due.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	f.now = f.now.Add(d)
	due := f.collectDueLocked()
	f.mu.Unlock()
	fire(due)
}

// collectDueLocked removes and returns waiters that are now due, and returns
// ticker channels that should fire. The caller must hold f.mu.
func (f *Fake) collectDueLocked() []chan time.Time {
	var due []chan time.Time

	remaining := f.waiters[:0]
	for _, w := range f.waiters {
		if !w.at.After(f.now) {
			due = append(due, w.ch)
			continue
		}
		remaining = append(remaining, w)
	}
	f.waiters = remaining

	for _, t := range f.tickers {
		if t.stopped || t.interval <= 0 {
			continue
		}
		for !t.next.After(f.now) {
			due = append(due, t.ch)
			t.next = t.next.Add(t.interval)
		}
	}
	return due
}

// fire delivers to each channel without blocking. A ticker channel that nobody
// is reading drops the tick, matching time.Ticker's behaviour.
func fire(chans []chan time.Time) {
	now := time.Now()
	for _, ch := range chans {
		select {
		case ch <- now:
		default:
		}
	}
}

// After returns a channel that receives once the fake clock reaches now+d.
// A non-positive duration fires immediately.
func (f *Fake) After(d time.Duration) <-chan time.Time {
	ch := make(chan time.Time, 1)
	if d <= 0 {
		ch <- f.Now()
		return ch
	}
	f.mu.Lock()
	f.waiters = append(f.waiters, &waiter{at: f.now.Add(d), ch: ch})
	f.mu.Unlock()
	return ch
}

// Sleep blocks until the fake clock has advanced by d. A test that calls this
// from the goroutine under test must advance the clock from another one.
func (f *Fake) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	<-f.After(d)
}

// NewTicker returns a ticker driven by the fake clock.
func (f *Fake) NewTicker(d time.Duration) Ticker {
	t := &fakeTicker{ch: make(chan time.Time, 1), interval: d}
	f.mu.Lock()
	t.next = f.now.Add(d)
	f.tickers = append(f.tickers, t)
	f.mu.Unlock()
	return t
}

type fakeTicker struct {
	ch       chan time.Time
	interval time.Duration
	next     time.Time
	stopped  bool
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }
func (t *fakeTicker) Stop()               { t.stopped = true }

// Compile-time assertions that both clocks satisfy the interface.
var (
	_ Clock = Real{}
	_ Clock = (*Fake)(nil)
)
