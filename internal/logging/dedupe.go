// SPDX-License-Identifier: AGPL-3.0-or-later

package logging

import (
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

// Dedupe lets one record per key through each window and counts the rest, so
// a client repeating one mistake cannot fill the log. The count is reported
// on the next record let through for that key.
type Dedupe struct {
	clk    clock.Clock
	window time.Duration
	max    int

	mu   sync.Mutex
	seen map[string]*dedupeEntry
}

type dedupeEntry struct {
	until      time.Time
	suppressed int
}

// NewDedupe returns a Dedupe holding at most max keys, each quiet for window
// after a record is let through.
func NewDedupe(clk clock.Clock, window time.Duration, maxKeys int) *Dedupe {
	return &Dedupe{clk: clk, window: window, max: max(maxKeys, 1), seen: map[string]*dedupeEntry{}}
}

// Allow reports whether a record for key may be written now and, when it
// may, how many records for key were held back since the last one written.
func (d *Dedupe) Allow(key string) (bool, int) {
	now := d.clk.Now()
	d.mu.Lock()
	defer d.mu.Unlock()
	e, ok := d.seen[key]
	if ok && now.Before(e.until) {
		e.suppressed++
		return false, 0
	}
	held := 0
	if ok {
		held = e.suppressed
	} else {
		d.makeRoom(now)
	}
	d.seen[key] = &dedupeEntry{until: now.Add(d.window)}
	return true, held
}

// makeRoom drops every key whose window has passed and, if the map is still
// full, the one whose window ends first. Its held-back count is lost, which
// is the price of a bound on memory when many clients misbehave at once.
func (d *Dedupe) makeRoom(now time.Time) {
	if len(d.seen) < d.max {
		return
	}
	oldest, at := "", time.Time{}
	for k, e := range d.seen {
		if !now.Before(e.until) {
			delete(d.seen, k)
			continue
		}
		if oldest == "" || e.until.Before(at) {
			oldest, at = k, e.until
		}
	}
	if len(d.seen) >= d.max {
		delete(d.seen, oldest)
	}
}
