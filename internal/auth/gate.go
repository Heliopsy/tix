// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"math"
	"runtime"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// ReasonHashBusy is the "reason" detail on ErrHashBusy, so a client can tell a
// refusal it should retry from one it should not.
const ReasonHashBusy = "password_hashing_busy"

// ErrHashBusy reports that password hashing was saturated for as long as the
// caller could wait. It is the same answer whoever asked, so it tells a caller
// nothing about whether the account exists.
func ErrHashBusy() error {
	return core.Precondition("the server is busy checking other passwords; retry in a few seconds").
		WithDetail("reason", ReasonHashBusy)
}

// HashWait is the longest a hash or verification waits for a slot, unless the
// caller's context ends first. It is under the server's default request
// timeout, so a queued sign-in is answered rather than cut off.
const HashWait = 10 * time.Second

// gate bounds how many argon2id derivations may run at once, and how many
// callers may queue behind them.
//
// One derivation allocates the hash's memory parameter in one piece. Without a
// bound, N concurrent sign-ins allocate N times that, so the login route is a
// memory exhaustion primitive. Queued callers wait up to their context or the
// gate's patience; callers beyond the queue are refused without allocating.
type gate struct {
	slots     chan struct{}
	waiting   atomic.Int64
	maxQueued int64
	patience  time.Duration
}

// newGate returns a gate admitting limit derivations with queue callers waiting.
func newGate(limit, queue int) *gate {
	if limit < 1 {
		limit = 1
	}
	if queue < 0 {
		queue = 0
	}
	return &gate{slots: make(chan struct{}, limit), maxQueued: int64(limit + queue), patience: HashWait}
}

// limit is how many derivations the gate admits at once.
func (g *gate) limit() int { return cap(g.slots) }

// do runs fn holding a slot. It refuses, without running fn, when the queue is
// full, ctx ends, or no slot frees within the gate's patience. It never runs
// fn outside a slot.
func (g *gate) do(ctx context.Context, fn func()) error {
	if ctx.Err() != nil {
		return ErrHashBusy()
	}
	if g.waiting.Add(1) > g.maxQueued {
		g.waiting.Add(-1)
		return ErrHashBusy()
	}
	defer g.waiting.Add(-1)

	select {
	case g.slots <- struct{}{}:
	default:
		timer := time.NewTimer(g.patience)
		defer timer.Stop()
		select {
		case g.slots <- struct{}{}:
		case <-ctx.Done():
			return ErrHashBusy()
		case <-timer.C:
			return ErrHashBusy()
		}
	}
	defer func() { <-g.slots }()
	fn()
	return nil
}

// queueFactor is how many callers may wait per running derivation.
const queueFactor = 8

// HashBudgetShare is the share of the runtime's soft memory limit that
// concurrent password hashing may hold. The rest is for everything else the
// server does while people sign in.
const HashBudgetShare = 0.25

// unboundedConcurrency is the bound when the process has no soft memory limit
// to size from: enough that two people signing in at once do not queue, and no
// more, since nothing says how much memory there is.
const unboundedConcurrency = 2

// ConcurrencyFor is how many derivations at memKiB each fit in HashBudgetShare
// of a soft limit of limitBytes, at least 1 and at most procs. A limitBytes of
// 0 or math.MaxInt64 means no limit, which gets unboundedConcurrency.
func ConcurrencyFor(limitBytes int64, memKiB uint32, procs int) int {
	procs = max(procs, 1)
	if limitBytes <= 0 || limitBytes == math.MaxInt64 || memKiB == 0 {
		return min(unboundedConcurrency, procs)
	}
	n := int(float64(limitBytes) * HashBudgetShare / (float64(memKiB) * 1024))
	return min(max(n, 1), procs)
}

// hashing is the process-wide gate, built on first use so that it is sized
// from the soft memory limit in force by then rather than at package load.
var hashing struct {
	mu       sync.Mutex
	gate     *gate
	override int
	memKiB   uint32
}

// SetMaxConcurrentHashes fixes how many argon2id derivations may run at once
// in this process, sized for hashes of memKiB each. An n of 0 sizes it from
// the runtime's soft memory limit (GOMEMLIMIT, or what tix serve derives from
// the container's limit) when the first hash runs.
func SetMaxConcurrentHashes(n int, memKiB uint32) {
	hashing.mu.Lock()
	defer hashing.mu.Unlock()
	if hashing.gate != nil && hashing.override == n && hashing.memKiB == memKiB {
		return
	}
	hashing.override, hashing.memKiB, hashing.gate = n, memKiB, nil
}

// MaxConcurrentHashes reports the bound in force, sizing the gate if no hash
// has run yet.
func MaxConcurrentHashes() int { return hashGate().limit() }

// hashGate returns the process-wide gate, building it if needed.
func hashGate() *gate {
	hashing.mu.Lock()
	defer hashing.mu.Unlock()
	if hashing.gate == nil {
		n := hashing.override
		if n <= 0 {
			mem := hashing.memKiB
			if mem == 0 {
				mem = DefaultParams().Memory
			}
			n = ConcurrencyFor(debug.SetMemoryLimit(-1), mem, runtime.GOMAXPROCS(0))
		}
		hashing.gate = newGate(n, n*queueFactor)
	}
	return hashing.gate
}

// swapHashGate installs g as the process-wide gate and returns the one it
// replaced. Tests use it to saturate or observe the bound.
func swapHashGate(g *gate) *gate {
	hashing.mu.Lock()
	defer hashing.mu.Unlock()
	previous := hashing.gate
	hashing.gate = g
	return previous
}

// IsHashBusy reports whether err is ErrHashBusy.
func IsHashBusy(err error) bool {
	var domain *core.Error
	return errors.As(err, &domain) && domain.Kind == core.KindPrecondition && domain.Details["reason"] == ReasonHashBusy
}

// HoldHashSlotsInTests fills every slot of the process-wide gate and turns
// waiting off, so the next hash or verification is refused, until release is
// called. It is for a test of a caller's busy path. Never call it outside a
// _test.go file.
func HoldHashSlotsInTests() (release func()) {
	g := newGate(1, 0)
	g.slots <- struct{}{}
	g.waiting.Store(1)
	previous := swapHashGate(g)
	return func() { swapHashGate(previous) }
}
