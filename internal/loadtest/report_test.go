// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"strings"
	"testing"
	"time"
)

func mem(at time.Duration, rssMiB, hwmMiB int64) MemSample {
	return MemSample{At: at, RSS: rssMiB * MiB, HWM: hwmMiB * MiB}
}

func TestSummarizeMemory(t *testing.T) {
	samples := []MemSample{
		mem(0, 40, 40),
		mem(1*time.Second, 200, 200), // a warm-up spike
		mem(2*time.Second, 60, 200),
		mem(3*time.Second, 70, 200),
		mem(4*time.Second, 80, 210), // VmHWM saw a spike between samples
	}
	m := SummarizeMemory(samples, 4*time.Second, 0.5)
	if !m.Available {
		t.Fatal("memory with samples reported as unavailable")
	}
	if m.Start != 40*MiB || m.End != 80*MiB {
		t.Errorf("start, end = %s, %s; want 40 and 80 MiB", FormatSize(m.Start), FormatSize(m.End))
	}
	if m.Peak != 210*MiB {
		t.Errorf("peak = %s, want 210 MiB from the final VmHWM", FormatSize(m.Peak))
	}
	if m.Steady != 70*MiB {
		t.Errorf("steady = %s, want 70 MiB: the median of the samples from 2s on", FormatSize(m.Steady))
	}
}

func TestSummarizeMemoryWithNoSamples(t *testing.T) {
	m := SummarizeMemory(nil, time.Second, 0.25)
	if m.Available || m.Reason == "" {
		t.Fatalf("no samples: %+v, want unavailable with a reason", m)
	}
}

func TestCheckBudget(t *testing.T) {
	measured := Memory{Available: true, Peak: 129 * MiB}
	tests := []struct {
		name    string
		mem     Memory
		max     int64
		checked bool
		pass    bool
		failed  bool
	}{
		{"over the budget fails", measured, 128 * MiB, true, false, true},
		{"at the budget passes", Memory{Available: true, Peak: 128 * MiB}, 128 * MiB, true, true, false},
		{"under the budget passes", Memory{Available: true, Peak: 90 * MiB}, 128 * MiB, true, true, false},
		{"no budget is not checked", measured, 0, false, false, false},
		{"remote memory is not checked", Memory{Reason: "remote"}, 128 * MiB, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			b := CheckBudget(tt.mem, tt.max)
			if b.Checked != tt.checked || b.Pass != tt.pass || b.Reason == "" {
				t.Fatalf("CheckBudget = %+v, want checked %v pass %v and a reason", b, tt.checked, tt.pass)
			}
			r := Report{Budget: b}
			if r.Failed() != tt.failed {
				t.Fatalf("Failed() = %v, want %v", r.Failed(), tt.failed)
			}
		})
	}
}

func TestSummarizeRoutes(t *testing.T) {
	var samples []Sample
	for i := 1; i <= 100; i++ {
		samples = append(samples, Sample{Route: "GET /tasks", Status: 200, Latency: time.Duration(i) * time.Millisecond})
	}
	samples = append(samples,
		Sample{Route: "POST /tasks", Status: 201, Latency: time.Millisecond},
		Sample{Route: "POST /tasks", Status: 409, Latency: time.Millisecond},
		Sample{Route: "POST /tasks", Status: 503, Latency: time.Millisecond},
		Sample{Route: "POST /tasks", Err: "connection refused", Latency: time.Millisecond},
	)
	got := Summarize(samples)
	if len(got) != 2 || got[0].Route != "GET /tasks" || got[1].Route != "POST /tasks" {
		t.Fatalf("routes = %+v, want GET /tasks then POST /tasks", got)
	}
	list := got[0]
	if list.P50ms != 50 || list.P95ms != 95 || list.P99ms != 99 || list.Maxms != 100 {
		t.Errorf("percentiles = %v/%v/%v/%v, want 50/95/99/100", list.P50ms, list.P95ms, list.P99ms, list.Maxms)
	}
	post := got[1]
	if post.Errors != 2 {
		t.Errorf("errors = %d, want 2: the 503 and the refused connection, not the 409", post.Errors)
	}
	want := map[string]int{"201": 1, "409": 1, "503": 1, "error": 1}
	for k, v := range want {
		if post.Statuses[k] != v {
			t.Errorf("statuses[%s] = %d, want %d", k, post.Statuses[k], v)
		}
	}
}

func TestBuildAndRenderReport(t *testing.T) {
	samples := []Sample{{Route: "GET /tasks", Status: 200, Latency: 5 * time.Millisecond}, {Route: "GET /tasks", Status: 500, Latency: time.Millisecond}}
	m := Memory{Available: true, Start: 40 * MiB, Steady: 100 * MiB, Peak: 300 * MiB, End: 90 * MiB}
	r := Build(Report{Mode: "local", Target: "http://x", Users: 4, Mix: DefaultMix, Duration: 2}, samples, map[string]int{"websocket events": 3}, m, 128*MiB)
	if r.Requests != 2 || r.Errors != 1 || r.RPS != 1 {
		t.Fatalf("requests, errors, rps = %d, %d, %v; want 2, 1, 1", r.Requests, r.Errors, r.RPS)
	}
	if !r.Failed() {
		t.Fatal("a 300 MiB peak passed a 128 MiB budget")
	}
	var b strings.Builder
	if err := r.WriteText(&b); err != nil {
		t.Fatal(err)
	}
	text := b.String()
	for _, want := range []string{"GET /tasks", "peak   300.0 MiB", "steady 100.0 MiB", "budget: FAIL", "websocket events: 3"} {
		if !strings.Contains(text, want) {
			t.Errorf("text report lacks %q:\n%s", want, text)
		}
	}
	var j strings.Builder
	if err := r.WriteJSON(&j); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(j.String(), `"peak_rss": 314572800`) || !strings.Contains(j.String(), `"pass": false`) {
		t.Errorf("JSON report lacks the peak or the verdict:\n%s", j.String())
	}
}
