package sshd

import (
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/clock"
)

// limiter is a token bucket per source address. Buckets that have refilled to
// full carry no state worth keeping, so they are dropped once the map grows
// past its bound rather than accumulating one entry per address ever seen.
type limiter struct {
	clk      clock.Clock
	interval time.Duration
	burst    int
	max      int

	mu      sync.Mutex
	buckets map[string]*bucket
}

// bucket is one source's allowance and the instant it was last refilled.
type bucket struct {
	tokens float64
	at     time.Time
}

// newLimiter returns a limiter that refills one token every interval, up to
// burst tokens, tracking at most max sources.
func newLimiter(clk clock.Clock, interval time.Duration, burst, max int) *limiter {
	if burst < 1 {
		burst = 1
	}
	if max < 1 {
		max = 1
	}
	return &limiter{clk: clk, interval: interval, burst: burst, max: max, buckets: map[string]*bucket{}}
}

// allow reports whether a connection from source may proceed, spending a token
// when it may.
func (l *limiter) allow(source string) bool {
	if source == "" {
		return true
	}
	now := l.clk.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	b, ok := l.buckets[source]
	if !ok {
		if len(l.buckets) >= l.max {
			l.evict(now)
		}
		b = &bucket{tokens: float64(l.burst), at: now}
		l.buckets[source] = b
	}
	if l.interval > 0 {
		b.tokens += now.Sub(b.at).Seconds() / l.interval.Seconds()
	}
	if b.tokens > float64(l.burst) {
		b.tokens = float64(l.burst)
	}
	b.at = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

// evict drops the sources whose allowance has already refilled, because
// forgetting a full bucket is indistinguishable from keeping it.
func (l *limiter) evict(now time.Time) {
	for source, b := range l.buckets {
		refilled := b.tokens
		if l.interval > 0 {
			refilled += now.Sub(b.at).Seconds() / l.interval.Seconds()
		}
		if refilled >= float64(l.burst) {
			delete(l.buckets, source)
		}
	}
	if len(l.buckets) < l.max {
		return
	}
	// Every source is still spending. Dropping the oldest keeps the map bounded
	// and costs that source nothing but a reset to a full allowance.
	var oldest string
	var at time.Time
	for source, b := range l.buckets {
		if oldest == "" || b.at.Before(at) {
			oldest, at = source, b.at
		}
	}
	delete(l.buckets, oldest)
}
