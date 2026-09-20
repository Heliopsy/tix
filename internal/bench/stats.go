package bench

import (
	"fmt"
	"slices"
	"time"
)

// Stats is a latency distribution.
type Stats struct {
	N   int
	P50 time.Duration
	P95 time.Duration
	P99 time.Duration
	Max time.Duration
}

// String renders the distribution for a test log.
func (s Stats) String() string {
	return fmt.Sprintf("n=%d p50=%s p95=%s p99=%s max=%s", s.N, s.P50, s.P95, s.P99, s.Max)
}

// Measure times n calls of fn and reports their distribution. The first error
// stops the run and is returned.
func Measure(n int, fn func() error) (Stats, error) {
	if n < 1 {
		return Stats{}, fmt.Errorf("measuring needs at least one iteration, got %d", n)
	}
	samples := make([]time.Duration, 0, n)
	for range n {
		start := time.Now()
		if err := fn(); err != nil {
			return Stats{}, err
		}
		samples = append(samples, time.Since(start))
	}
	return Summarize(samples), nil
}

// Summarize reduces raw samples to a distribution.
func Summarize(samples []time.Duration) Stats {
	if len(samples) == 0 {
		return Stats{}
	}
	sorted := slices.Clone(samples)
	slices.Sort(sorted)
	return Stats{
		N:   len(sorted),
		P50: percentile(sorted, 0.50),
		P95: percentile(sorted, 0.95),
		P99: percentile(sorted, 0.99),
		Max: sorted[len(sorted)-1],
	}
}

// percentile reads the nearest-rank percentile of an already sorted slice.
func percentile(sorted []time.Duration, q float64) time.Duration {
	rank := int(q*float64(len(sorted)) + 0.5)
	if rank < 1 {
		rank = 1
	}
	if rank > len(sorted) {
		rank = len(sorted)
	}
	return sorted[rank-1]
}
