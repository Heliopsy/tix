// SPDX-License-Identifier: AGPL-3.0-or-later

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
//
// A session is admitted in two steps because the key has not been resolved to
// an actor yet when the first one runs. A reservation answers the capacity
// question without spending capacity, so a key nobody enrolled cannot hold a
// slot an enrolled key then cannot get; the slot itself is taken once the
// identity behind the key is known.
type gate struct {
	maxPerKey int
	max       int

	mu       sync.Mutex
	perKey   map[string]int
	reserved map[string]int
	live     int
}

// newGate returns a gate admitting maxPerKey sessions per key and max overall.
func newGate(maxPerKey, max int) *gate {
	if max < 1 {
		max = 1
	}
	if maxPerKey < 1 || maxPerKey > max {
		maxPerKey = max
	}
	return &gate{maxPerKey: maxPerKey, max: max, perKey: map[string]int{}, reserved: map[string]int{}}
}

// reserve answers the capacity question before a key has been looked up,
// returning the refusal to show the client when there is none.
//
// It counts against the per-key cap and not against the listener's, so the
// capacity strangers can occupy is bounded by their own key rather than shared
// with the sessions enrolled keys are holding. Neither refusal depends on
// anything but the counters, which is what keeps a full listener from
// answering a stranger differently than somebody it has seen before: no
// lookup has happened yet when this decides.
func (g *gate) reserve(fingerprint string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.live >= g.max {
		return g.atCapacity()
	}
	if g.perKey[fingerprint]+g.reserved[fingerprint] >= g.maxPerKey {
		return g.atKeyCapacity()
	}
	g.reserved[fingerprint]++
	return nil
}

// cancel drops a reservation the lookup behind it refused.
func (g *gate) cancel(fingerprint string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.drop(fingerprint)
}

// acquire takes a slot for a fingerprint, consuming any reservation it holds
// and returning the refusal to show the client when there is none.
//
// The caps are read again here rather than trusted from the reservation: the
// listener may have filled while the key was being resolved, and a full
// listener refuses the session it cannot serve with the message every other
// key at that moment gets.
func (g *gate) acquire(fingerprint string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.drop(fingerprint)
	if g.live >= g.max {
		return g.atCapacity()
	}
	if g.perKey[fingerprint] >= g.maxPerKey {
		return g.atKeyCapacity()
	}
	g.perKey[fingerprint]++
	g.live++
	return nil
}

// drop gives a reservation back. The caller holds the lock.
func (g *gate) drop(fingerprint string) {
	switch n := g.reserved[fingerprint]; {
	case n <= 0:
	case n == 1:
		delete(g.reserved, fingerprint)
	default:
		g.reserved[fingerprint] = n - 1
	}
}

// atCapacity is the refusal a listener with no slot left gives every key.
func (g *gate) atCapacity() error {
	return core.Precondition(
		"this demo is serving its limit of %d sessions; try again shortly", g.max)
}

// atKeyCapacity is the refusal a key already holding its share gives.
func (g *gate) atKeyCapacity() error {
	return core.Precondition(
		"this key is at its limit of %d concurrent sessions; close one and reconnect", g.maxPerKey)
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
