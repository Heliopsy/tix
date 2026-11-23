// SPDX-License-Identifier: AGPL-3.0-or-later

package logging_test

import (
	"context"
	"errors"
	"maps"
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/logging"
)

func TestDedupeLetsOneRecordAWindowThroughAndCountsTheRest(t *testing.T) {
	clk := clock.NewFakeAt()
	d := logging.NewDedupe(clk, time.Minute, 16, nil)
	type step struct {
		advance time.Duration
		key     string
		ok      bool
		held    int
	}
	steps := []step{
		{0, "a", true, 0},
		{time.Second, "a", false, 0},
		{time.Second, "a", false, 0},
		{0, "b", true, 0},
		{58 * time.Second, "a", true, 2},
		{time.Second, "a", false, 0},
		{time.Minute, "a", true, 1},
		{time.Minute, "a", true, 0},
		{0, "b", true, 0},
	}
	for i, s := range steps {
		clk.Advance(s.advance)
		ok, held := d.Allow(s.key)
		if ok != s.ok || held != s.held {
			t.Errorf("step %d (%s): Allow = %v, %d; want %v, %d", i, s.key, ok, held, s.ok, s.held)
		}
	}
}

// reports collects what a Dedupe reports outside a record.
type reports struct {
	mu  sync.Mutex
	got map[string]int
}

func (r *reports) add(key string, held int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.got[key] += held
}

func (r *reports) snapshot() map[string]int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return maps.Clone(r.got)
}

func newReports() *reports { return &reports{got: map[string]int{}} }

func TestDedupeReportsACountNoLaterRecordCarried(t *testing.T) {
	clk := clock.NewFakeAt()
	r := newReports()
	d := logging.NewDedupe(clk, time.Minute, 16, r.add)
	for range 10_000 {
		d.Allow("a")
	}
	d.Allow("b")
	d.Sweep()
	if got := r.snapshot(); len(got) != 0 {
		t.Fatalf("reported %v inside the window", got)
	}
	clk.Advance(time.Minute)
	d.Sweep()
	if got := r.snapshot(); !maps.Equal(got, map[string]int{"a": 9_999}) {
		t.Errorf("after the window reported %v, want a: 9999 and nothing for b", got)
	}
	if ok, held := d.Allow("a"); !ok || held != 0 {
		t.Errorf("after the report Allow(a) = %v, %d; want true, 0, the count already reported", ok, held)
	}
}

func TestDedupeAllowReportsExpiredKeysOnceAWindow(t *testing.T) {
	clk := clock.NewFakeAt()
	r := newReports()
	d := logging.NewDedupe(clk, time.Minute, 16, r.add)
	d.Allow("a")
	d.Allow("a")
	d.Allow("a")
	clk.Advance(time.Minute)
	d.Allow("b")
	if got := r.snapshot(); !maps.Equal(got, map[string]int{"a": 2}) {
		t.Errorf("an Allow after the window reported %v, want a: 2", got)
	}
}

func TestDedupeRunReportsOnATickAndWhatIsLeftAtTheEnd(t *testing.T) {
	clk := clock.NewFakeAt()
	r := newReports()
	d := logging.NewDedupe(clk, time.Minute, 16, r.add)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	for clk.Tickers() == 0 {
		runtime.Gosched()
	}
	d.Allow("a")
	d.Allow("a")
	clk.Advance(time.Minute)
	deadline := time.After(10 * time.Second)
	for r.snapshot()["a"] != 1 {
		select {
		case <-deadline:
			t.Fatalf("the tick reported %v, want a: 1", r.snapshot())
		default:
			runtime.Gosched()
		}
	}
	d.Allow("b")
	d.Allow("b")
	d.Allow("b")
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("Run returned %v", err)
	}
	if got := r.snapshot(); !maps.Equal(got, map[string]int{"a": 1, "b": 2}) {
		t.Errorf("at the end reported %v, want a: 1 and b: 2 still inside its window", got)
	}
	if clk.Tickers() != 0 {
		t.Error("Run left its ticker running")
	}
}

func TestDedupeSharesOneBucketOnceFullAndKeepsLiveKeys(t *testing.T) {
	clk := clock.NewFakeAt()
	r := newReports()
	d := logging.NewDedupe(clk, time.Minute, 2, r.add)
	for _, k := range []string{"a", "b"} {
		if ok, _ := d.Allow(k); !ok {
			t.Fatalf("first record for %s held back", k)
		}
	}
	if ok, _ := d.Allow("c"); !ok {
		t.Error("the first key past the bound was held back, want one overflow record let through")
	}
	for _, k := range []string{"d", "e", "c"} {
		if ok, _ := d.Allow(k); ok {
			t.Errorf("key %s past the bound was let through inside the overflow window", k)
		}
	}
	if ok, _ := d.Allow("a"); ok {
		t.Error("a live key was evicted to make room, so its repeat was let through")
	}
	clk.Advance(time.Minute)
	d.Sweep()
	if got := r.snapshot(); !maps.Equal(got, map[string]int{"a": 1, logging.OverflowKey: 3}) {
		t.Errorf("reported %v, want a: 1 and the overflow: 3", got)
	}
	if ok, _ := d.Allow("c"); !ok {
		t.Error("a key that found room after the sweep was held back")
	}
}
