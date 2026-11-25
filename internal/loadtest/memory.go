// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// MemSample is one reading of the server's memory.
type MemSample struct {
	// At is the offset from the start of the load.
	At time.Duration `json:"-"`
	// Seconds is At in seconds, for the JSON report.
	Seconds float64 `json:"at_s"`
	RSS     int64   `json:"rss"`
	HWM     int64   `json:"hwm"`
	// Heap is the live Go heap after the most recent collection, from gctrace.
	Heap int64 `json:"heap,omitempty"`
}

// ProcStatus reads VmRSS and VmHWM from a /proc/PID/status file.
func ProcStatus(r io.Reader) (rss, hwm int64, err error) {
	sc := bufio.NewScanner(r)
	found := 0
	for sc.Scan() {
		line := sc.Text()
		key, val, ok := strings.Cut(line, ":")
		if !ok || (key != "VmRSS" && key != "VmHWM") {
			continue
		}
		fields := strings.Fields(val)
		if len(fields) != 2 || fields[1] != "kB" {
			return 0, 0, fmt.Errorf("unexpected %s line %q", key, line)
		}
		kb, err := strconv.ParseInt(fields[0], 10, 64)
		if err != nil {
			return 0, 0, fmt.Errorf("reading %s: %w", key, err)
		}
		if key == "VmRSS" {
			rss = kb << 10
		} else {
			hwm = kb << 10
		}
		found++
	}
	if err := sc.Err(); err != nil {
		return 0, 0, fmt.Errorf("reading process status: %w", err)
	}
	if found < 2 {
		return 0, 0, fmt.Errorf("process status carries no VmRSS and VmHWM")
	}
	return rss, hwm, nil
}

// readProc reads one process's memory from /proc. It fails off Linux.
func readProc(pid int) (rss, hwm int64, err error) {
	f, err := os.Open(fmt.Sprintf("/proc/%d/status", pid))
	if err != nil {
		return 0, 0, fmt.Errorf("opening process status: %w", err)
	}
	defer func() { _ = f.Close() }()
	return ProcStatus(f)
}

// gcLine matches the heap sizes in a GODEBUG=gctrace=1 line:
// "gc 12 @3.1s 2%: ... ms cpu, 9->10->5 MB, 11 MB goal, ...". The third
// number is the heap that survived the collection.
var gcLine = regexp.MustCompile(`^gc \d+ @[0-9.]+s .*?, (\d+)->(\d+)->(\d+) MB, (\d+) MB goal`)

// ParseGCTrace returns the live heap in bytes a gctrace line reports.
func ParseGCTrace(line string) (int64, bool) {
	m := gcLine.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	live, err := strconv.ParseInt(m[3], 10, 64)
	if err != nil {
		return 0, false
	}
	return live * MiB, true
}

// heapWatch keeps the live heap from the latest gctrace line it saw.
type heapWatch struct {
	mu   sync.Mutex
	live int64
	peak int64
	tail []string
}

// consume reads the server's stderr line by line, keeping the last lines that are not gctrace.
func (h *heapWatch) consume(r io.Reader) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64<<10), 1<<20)
	for sc.Scan() {
		line := sc.Text()
		h.mu.Lock()
		if live, ok := ParseGCTrace(line); ok {
			h.live = live
			h.peak = max(h.peak, live)
		} else {
			h.tail = append(h.tail, line)
			if len(h.tail) > 40 {
				h.tail = h.tail[len(h.tail)-40:]
			}
		}
		h.mu.Unlock()
	}
}

func (h *heapWatch) current() (live, peak int64) {
	if h == nil {
		return 0, 0
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.live, h.peak
}

func (h *heapWatch) log() string {
	if h == nil {
		return ""
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.Join(h.tail, "\n")
}

// sampler reads a process's memory on a ticker until its context ends.
type sampler struct {
	pid   int
	every time.Duration
	heap  *heapWatch
	start time.Time

	mu      sync.Mutex
	samples []MemSample
	err     error
}

func (s *sampler) run(ctx context.Context) {
	t := time.NewTicker(s.every)
	defer t.Stop()
	s.take()
	for {
		select {
		case <-ctx.Done():
			s.take()
			return
		case <-t.C:
			s.take()
		}
	}
}

func (s *sampler) take() {
	rss, hwm, err := readProc(s.pid)
	s.mu.Lock()
	defer s.mu.Unlock()
	if err != nil {
		if s.err == nil {
			s.err = err
		}
		return
	}
	live, _ := s.heap.current()
	at := time.Since(s.start)
	s.samples = append(s.samples, MemSample{At: at, Seconds: at.Seconds(), RSS: rss, HWM: hwm, Heap: live})
}

func (s *sampler) result() ([]MemSample, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]MemSample(nil), s.samples...), s.err
}
