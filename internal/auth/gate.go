// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"runtime"
	"sync/atomic"

	"github.com/heliopsy/tix/internal/core"
)

// ErrVerifyOverloaded reports that too many password verifications were already
// queued. It is the same answer whoever asked, so it tells a caller nothing
// about whether the account exists.
var ErrVerifyOverloaded = core.Precondition("too many password verifications in flight; retry shortly")

// gate bounds how many argon2id derivations may run at once, and how many
// callers may queue behind them.
//
// One verification allocates the hash's memory parameter, 64 MiB under
// DefaultParams. Without a bound, N unauthenticated logins allocate N times
// that, so the login route is a memory exhaustion primitive. Queued callers
// wait; callers beyond the queue are refused without allocating.
type gate struct {
	slots   chan struct{}
	waiting atomic.Int64
	maxWait int64
}

// newGate returns a gate admitting limit derivations with queue callers waiting.
func newGate(limit, queue int) *gate {
	if limit < 1 {
		limit = 1
	}
	if queue < 0 {
		queue = 0
	}
	return &gate{slots: make(chan struct{}, limit), maxWait: int64(limit + queue)}
}

// do runs fn holding a slot, or refuses when the queue is already full.
func (g *gate) do(fn func()) error {
	if g.waiting.Add(1) > g.maxWait {
		g.waiting.Add(-1)
		return ErrVerifyOverloaded
	}
	defer g.waiting.Add(-1)

	g.slots <- struct{}{}
	defer func() { <-g.slots }()
	fn()
	return nil
}

// defaultVerifyQueueFactor is how many callers may wait per running derivation.
const defaultVerifyQueueFactor = 8

// verifyGate is the process-wide bound on concurrent password verification.
var verifyGate = newGate(defaultVerifyLimit(), defaultVerifyLimit()*defaultVerifyQueueFactor)

// defaultVerifyLimit keeps the resident cost of verification proportional to
// the cores that can make progress on it rather than to inbound request count.
func defaultVerifyLimit() int {
	if n := runtime.GOMAXPROCS(0); n > 2 {
		return n
	}
	return 2
}
