// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"context"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

// OverflowKey is the key a held-back count is reported under when it was
// counted while every key was taken.
const OverflowKey = ""

// Dedupe lets one record per key through each window and counts the rest, so
// a client repeating one mistake cannot fill the log. The count is reported
// on the next record let through for that key, or to report once the window
// has passed without one, so it is never dropped.
type Dedupe struct {
	clk    clock.Clock
	window time.Duration
	max    int
	report func(key string, held int)

	mu        sync.Mutex
	seen      map[string]*dedupeEntry
	overflow  dedupeEntry
	nextSweep time.Time
}

type dedupeEntry struct {
	until      time.Time
	suppressed int
}

type heldBack struct {
	key  string
	held int
}

// NewDedupe returns a Dedupe holding at most maxKeys keys, each quiet for
// window after a record is let through. A key arriving while every key is
// taken shares one overflow bucket, reported as OverflowKey. report receives
// each count nothing else will carry; nil discards them.
func NewDedupe(clk clock.Clock, window time.Duration, maxKeys int, report func(key string, held int)) *Dedupe {
	if report == nil {
		report = func(string, int) {}
	}
	return &Dedupe{clk: clk, window: window, max: max(maxKeys, 1), report: report,
		seen: map[string]*dedupeEntry{}, nextSweep: clk.Now().Add(window)}
}

// Allow reports whether a record for key may be written now and, when it
// may, how many records for key were held back since the last one written.
func (d *Dedupe) Allow(key string) (bool, int) {
	now := d.clk.Now()
	d.mu.Lock()
	ok, held := d.allowLocked(key, now)
	var due []heldBack
	if !now.Before(d.nextSweep) {
		due = d.sweepLocked(now, false)
	}
	d.mu.Unlock()
	d.emit(due)
	return ok, held
}

func (d *Dedupe) allowLocked(key string, now time.Time) (bool, int) {
	if e, ok := d.seen[key]; ok {
		if now.Before(e.until) {
			e.suppressed++
			return false, 0
		}
		held := e.suppressed
		*e = dedupeEntry{until: now.Add(d.window)}
		return true, held
	}
	if len(d.seen) < d.max {
		d.seen[key] = &dedupeEntry{until: now.Add(d.window)}
		return true, 0
	}
	if now.Before(d.overflow.until) {
		d.overflow.suppressed++
		return false, 0
	}
	d.overflow.until = now.Add(d.window)
	return true, 0
}

// Sweep reports the count of every key whose window has passed and forgets
// the key. Allow sweeps once a window on its own; Run sweeps when nothing
// calls Allow.
func (d *Dedupe) Sweep() {
	d.mu.Lock()
	due := d.sweepLocked(d.clk.Now(), false)
	d.mu.Unlock()
	d.emit(due)
}

// Run sweeps once a window until ctx is done, then reports every count still
// held, so a count outlives neither a quiet server nor its shutdown.
func (d *Dedupe) Run(ctx context.Context) error {
	ticker := d.clk.NewTicker(d.window)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			d.mu.Lock()
			due := d.sweepLocked(d.clk.Now(), true)
			d.mu.Unlock()
			d.emit(due)
			return ctx.Err()
		case <-ticker.C():
			d.Sweep()
		}
	}
}

// sweepLocked drops every expired key, or every key when all is set, and
// returns the counts they held.
func (d *Dedupe) sweepLocked(now time.Time, all bool) []heldBack {
	var due []heldBack
	for k, e := range d.seen {
		if all || !now.Before(e.until) {
			if e.suppressed > 0 {
				due = append(due, heldBack{k, e.suppressed})
			}
			delete(d.seen, k)
		}
	}
	if d.overflow.suppressed > 0 && (all || !now.Before(d.overflow.until)) {
		due = append(due, heldBack{OverflowKey, d.overflow.suppressed})
		d.overflow.suppressed = 0
	}
	d.nextSweep = now.Add(d.window)
	return due
}

func (d *Dedupe) emit(due []heldBack) {
	for _, h := range due {
		d.report(h.key, h.held)
	}
}
