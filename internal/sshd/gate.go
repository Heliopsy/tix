package sshd

import (
	"sync"

	"github.com/heliopsy/tix/internal/core"
)

// gate caps how many sessions are live at once: one number for a single key,
// one for the listener as a whole.
//
// The rate limiter counts connections per hour from one source address, which
// says nothing about how many of them are still open. One key holding fifty
// sessions is fifty programs, each with its own event subscription, from one
// address that never exceeded its allowance.
type gate struct {
	maxPerKey int
	max       int

	mu     sync.Mutex
	perKey map[string]int
	live   int
}

// newGate returns a gate admitting maxPerKey sessions per key and max overall.
func newGate(maxPerKey, max int) *gate {
	if max < 1 {
		max = 1
	}
	if maxPerKey < 1 || maxPerKey > max {
		maxPerKey = max
	}
	return &gate{maxPerKey: maxPerKey, max: max, perKey: map[string]int{}}
}

// acquire takes a slot for a fingerprint, returning the refusal to show the
// client when there is none.
//
// Neither refusal depends on anything but the counters, which is what keeps a
// full listener from answering a stranger differently than somebody it has
// seen before: no lookup has happened yet when this decides.
func (g *gate) acquire(fingerprint string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.live >= g.max {
		return core.Precondition(
			"this demo is serving its limit of %d sessions; try again shortly", g.max)
	}
	if g.perKey[fingerprint] >= g.maxPerKey {
		return core.Precondition(
			"this key is at its limit of %d concurrent sessions; close one and reconnect", g.maxPerKey)
	}
	g.perKey[fingerprint]++
	g.live++
	return nil
}

// release gives a slot back, dropping the fingerprint once it holds none.
func (g *gate) release(fingerprint string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.perKey[fingerprint] <= 1 {
		delete(g.perKey, fingerprint)
	} else {
		g.perKey[fingerprint]--
	}
	if g.live > 0 {
		g.live--
	}
}

// counts reports the sessions one fingerprint holds and the listener total.
func (g *gate) counts(fingerprint string) (int, int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.perKey[fingerprint], g.live
}
