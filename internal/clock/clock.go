// Package clock provides an injectable time source.
package clock

import (
	"sync"
	"time"
)

// Clock is a source of time.
type Clock interface {
	Now() time.Time
	Since(t time.Time) time.Duration
	NewTicker(d time.Duration) Ticker
	After(d time.Duration) <-chan time.Time
	Sleep(d time.Duration)
}

// Ticker fires repeatedly.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// Real is a Clock backed by the system clock.
type Real struct{}

// New returns a real clock.
func New() Real { return Real{} }

// Now returns the current UTC time.
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

// Fake is a Clock under test control.
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

// NewFakeAt returns a fake clock set to a fixed, arbitrary instant.
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

// collectDueLocked returns channels that are now due. Caller holds f.mu.
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
		if t.interval <= 0 {
			continue
		}
		for !t.next.After(f.now) {
			due = append(due, t.ch)
			t.next = t.next.Add(t.interval)
		}
	}
	return due
}

// fire delivers to each channel without blocking.
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

// Sleep blocks until the fake clock has advanced by d.
func (f *Fake) Sleep(d time.Duration) {
	if d <= 0 {
		return
	}
	<-f.After(d)
}

// NewTicker returns a ticker driven by the fake clock.
func (f *Fake) NewTicker(d time.Duration) Ticker {
	t := &fakeTicker{ch: make(chan time.Time, 1), interval: d, fake: f}
	f.mu.Lock()
	t.next = f.now.Add(d)
	f.tickers = append(f.tickers, t)
	f.mu.Unlock()
	return t
}

// dropTicker unregisters t, so a stopped ticker costs nothing to later sweeps.
func (f *Fake) dropTicker(t *fakeTicker) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, cur := range f.tickers {
		if cur != t {
			continue
		}
		last := len(f.tickers) - 1
		f.tickers[i] = f.tickers[last]
		f.tickers[last] = nil
		f.tickers = f.tickers[:last]
		return
	}
}

// Tickers reports how many live tickers the clock is driving.
func (f *Fake) Tickers() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tickers)
}

// fakeTicker is registered with its Fake, whose mutex guards every field the
// clock touches. Stop is called from the goroutine owning the ticker while the
// test goroutine advances the clock, so it may not write without that lock.
type fakeTicker struct {
	fake *Fake

	ch       chan time.Time
	interval time.Duration
	next     time.Time
}

func (t *fakeTicker) C() <-chan time.Time { return t.ch }

// Stop unregisters the ticker. It is safe to call more than once.
func (t *fakeTicker) Stop() { t.fake.dropTicker(t) }

// Compile-time assertions that both clocks satisfy the interface.
var (
	_ Clock = Real{}
	_ Clock = (*Fake)(nil)
)
