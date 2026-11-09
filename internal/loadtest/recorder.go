// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"math"
	"sort"
	"strconv"
	"sync"
	"time"
)

// Sample is one finished request.
type Sample struct {
	Route   string
	Status  int
	Latency time.Duration
	// Err is set when no status came back: a refused connection, a timeout.
	Err string
}

// Recorder collects samples from every virtual user.
type Recorder struct {
	mu      sync.Mutex
	samples []Sample
	notes   map[string]int
}

// NewRecorder returns an empty recorder.
func NewRecorder() *Recorder { return &Recorder{notes: map[string]int{}} }

// Add records one request.
func (r *Recorder) Add(s Sample) {
	r.mu.Lock()
	r.samples = append(r.samples, s)
	r.mu.Unlock()
}

// Note counts something that is not a request, such as a WebSocket event received.
func (r *Recorder) Note(what string) {
	r.mu.Lock()
	r.notes[what]++
	r.mu.Unlock()
}

// Snapshot returns a copy of what has been recorded.
func (r *Recorder) Snapshot() ([]Sample, map[string]int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	notes := make(map[string]int, len(r.notes))
	for k, v := range r.notes {
		notes[k] = v
	}
	return append([]Sample(nil), r.samples...), notes
}

// RouteStats summarises one route.
type RouteStats struct {
	Route    string         `json:"route"`
	Count    int            `json:"count"`
	Errors   int            `json:"errors"`
	Statuses map[string]int `json:"statuses"`
	P50ms    float64        `json:"p50_ms"`
	P95ms    float64        `json:"p95_ms"`
	P99ms    float64        `json:"p99_ms"`
	Maxms    float64        `json:"max_ms"`
}

// Summarize groups samples by route, sorted by route name. A request counts as
// an error when it got no answer or a 5xx; a 4xx is reported by status but is
// not an error, because a refused transition is ordinary agent traffic.
func Summarize(samples []Sample) []RouteStats {
	by := map[string][]Sample{}
	for _, s := range samples {
		by[s.Route] = append(by[s.Route], s)
	}
	out := make([]RouteStats, 0, len(by))
	for route, ss := range by {
		st := RouteStats{Route: route, Count: len(ss), Statuses: map[string]int{}}
		lat := make([]time.Duration, 0, len(ss))
		for _, s := range ss {
			key := statusKey(s)
			st.Statuses[key]++
			if s.Err != "" || s.Status >= 500 {
				st.Errors++
			}
			lat = append(lat, s.Latency)
		}
		sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
		st.P50ms = ms(Percentile(lat, 0.50))
		st.P95ms = ms(Percentile(lat, 0.95))
		st.P99ms = ms(Percentile(lat, 0.99))
		st.Maxms = ms(lat[len(lat)-1])
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Route < out[j].Route })
	return out
}

func statusKey(s Sample) string {
	if s.Err != "" {
		return "error"
	}
	return strconv.Itoa(s.Status)
}

// Percentile reads the nearest-rank percentile of sorted durations.
func Percentile(sorted []time.Duration, p float64) time.Duration {
	if len(sorted) == 0 {
		return 0
	}
	rank := int(math.Ceil(p*float64(len(sorted)))) - 1
	rank = max(0, min(rank, len(sorted)-1))
	return sorted[rank]
}

func ms(d time.Duration) float64 {
	return float64(d.Microseconds()) / 1000
}
