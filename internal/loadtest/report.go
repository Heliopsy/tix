// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"
	"time"
)

// Memory is what the report says about the server's resident memory.
type Memory struct {
	// Available is false when the server is remote or /proc cannot be read.
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
	// Start is the first reading, taken before any user starts.
	Start int64 `json:"start_rss,omitempty"`
	// Peak is the highest of every sampled VmRSS and the final VmHWM, which
	// also catches a spike that fell between two samples.
	Peak int64 `json:"peak_rss,omitempty"`
	// Steady is the median VmRSS after the warm-up share of the run.
	Steady int64 `json:"steady_rss,omitempty"`
	// End is the last reading.
	End      int64       `json:"end_rss,omitempty"`
	PeakHeap int64       `json:"peak_heap,omitempty"`
	Samples  []MemSample `json:"samples,omitempty"`
}

// Budget is the verdict on the memory budget.
type Budget struct {
	Max     int64  `json:"max_rss"`
	Checked bool   `json:"checked"`
	Pass    bool   `json:"pass"`
	Reason  string `json:"reason"`
}

// Report is everything one run measured.
type Report struct {
	Mode     string         `json:"mode"`
	Target   string         `json:"target"`
	Engine   string         `json:"engine,omitempty"`
	Users    int            `json:"users"`
	Mix      string         `json:"mix"`
	Duration float64        `json:"duration_s"`
	Seeded   string         `json:"seeded,omitempty"`
	Requests int            `json:"requests"`
	Errors   int            `json:"errors"`
	RPS      float64        `json:"rps"`
	Routes   []RouteStats   `json:"routes"`
	Notes    map[string]int `json:"notes,omitempty"`
	Memory   Memory         `json:"memory"`
	Budget   Budget         `json:"budget"`
}

// Failed reports whether the run should exit non-zero.
func (r *Report) Failed() bool { return r.Budget.Checked && !r.Budget.Pass }

// SummarizeMemory reduces the samples to start, peak, steady state and end.
// Samples taken before warmup of the load window are left out of Steady.
func SummarizeMemory(samples []MemSample, load time.Duration, warmup float64) Memory {
	if len(samples) == 0 {
		return Memory{Reason: "no memory sample was taken"}
	}
	m := Memory{Available: true, Samples: samples, Start: samples[0].RSS, End: samples[len(samples)-1].RSS}
	cut := time.Duration(float64(load) * warmup)
	var steady []int64
	for _, s := range samples {
		m.Peak = max(m.Peak, s.RSS, s.HWM)
		m.PeakHeap = max(m.PeakHeap, s.Heap)
		if s.At >= cut {
			steady = append(steady, s.RSS)
		}
	}
	if len(steady) == 0 {
		steady = []int64{m.End}
	}
	sort.Slice(steady, func(i, j int) bool { return steady[i] < steady[j] })
	m.Steady = steady[len(steady)/2]
	return m
}

// CheckBudget compares the peak against the budget. A budget of zero, or a
// run whose memory was not readable, is reported but not enforced.
func CheckBudget(m Memory, maxRSS int64) Budget {
	b := Budget{Max: maxRSS}
	switch {
	case maxRSS <= 0:
		b.Reason = "no memory budget set"
	case !m.Available:
		b.Reason = "memory not measured: " + m.Reason
	case m.Peak > maxRSS:
		b.Checked = true
		b.Reason = fmt.Sprintf("peak RSS %s exceeds the budget of %s", FormatSize(m.Peak), FormatSize(maxRSS))
	default:
		b.Checked, b.Pass = true, true
		b.Reason = fmt.Sprintf("peak RSS %s is within the budget of %s", FormatSize(m.Peak), FormatSize(maxRSS))
	}
	return b
}

// Build assembles a report from what a run recorded.
func Build(base Report, samples []Sample, notes map[string]int, mem Memory, maxRSS int64) Report {
	r := base
	r.Routes = Summarize(samples)
	r.Notes = notes
	r.Requests = len(samples)
	for _, st := range r.Routes {
		r.Errors += st.Errors
	}
	if r.Duration > 0 {
		r.RPS = float64(r.Requests) / r.Duration
	}
	r.Memory = mem
	r.Budget = CheckBudget(mem, maxRSS)
	return r
}

// WriteJSON writes the report as indented JSON.
func (r *Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("encoding the report: %w", err)
	}
	return nil
}

// WriteText writes the report for a person to read.
func (r *Report) WriteText(w io.Writer) error {
	var b strings.Builder
	fmt.Fprintf(&b, "tix load report: %s %s", r.Mode, r.Target)
	if r.Engine != "" {
		fmt.Fprintf(&b, " (%s)", r.Engine)
	}
	fmt.Fprintf(&b, "\nusers %d (%s) for %.0fs", r.Users, r.Mix, r.Duration)
	if r.Seeded != "" {
		fmt.Fprintf(&b, ", seeded %s", r.Seeded)
	}
	fmt.Fprintf(&b, "\nrequests %d, %.1f/s, errors %d\n\n", r.Requests, r.RPS, r.Errors)

	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', tabwriter.AlignRight)
	fmt.Fprintln(tw, "route\tcount\tp50 ms\tp95 ms\tp99 ms\tmax ms\terrors\tstatuses\t")
	for _, st := range r.Routes {
		fmt.Fprintf(tw, "%s\t%d\t%.1f\t%.1f\t%.1f\t%.1f\t%d\t%s\t\n",
			st.Route, st.Count, st.P50ms, st.P95ms, st.P99ms, st.Maxms, st.Errors, statusList(st.Statuses))
	}
	if err := tw.Flush(); err != nil {
		return fmt.Errorf("rendering the route table: %w", err)
	}
	if len(r.Notes) > 0 {
		keys := make([]string, 0, len(r.Notes))
		for k := range r.Notes {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		b.WriteString("\n")
		for _, k := range keys {
			fmt.Fprintf(&b, "%s: %d\n", k, r.Notes[k])
		}
	}
	b.WriteString("\nserver memory\n")
	if r.Memory.Available {
		fmt.Fprintf(&b, "  start  %s\n  steady %s (median after warm-up)\n  peak   %s (max of VmRSS samples and VmHWM)\n  end    %s\n",
			FormatSize(r.Memory.Start), FormatSize(r.Memory.Steady), FormatSize(r.Memory.Peak), FormatSize(r.Memory.End))
		if r.Memory.PeakHeap > 0 {
			fmt.Fprintf(&b, "  live heap peak %s (gctrace)\n", FormatSize(r.Memory.PeakHeap))
		}
	} else {
		fmt.Fprintf(&b, "  not measured: %s\n", r.Memory.Reason)
	}
	verdict := "not checked"
	if r.Budget.Checked {
		verdict = "PASS"
		if !r.Budget.Pass {
			verdict = "FAIL"
		}
	}
	fmt.Fprintf(&b, "budget: %s, %s\n", verdict, r.Budget.Reason)
	_, err := io.WriteString(w, b.String())
	if err != nil {
		return fmt.Errorf("writing the report: %w", err)
	}
	return nil
}

func statusList(m map[string]int) string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	parts := make([]string, len(keys))
	for i, k := range keys {
		parts[i] = fmt.Sprintf("%s:%d", k, m[k])
	}
	return strings.Join(parts, " ")
}
