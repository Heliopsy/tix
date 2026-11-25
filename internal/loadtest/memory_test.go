// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestProcStatus(t *testing.T) {
	status := "Name:\ttix\nVmPeak:\t  900000 kB\nVmHWM:\t  262144 kB\nVmRSS:\t  131072 kB\nThreads:\t12\n"
	rss, hwm, err := ProcStatus(strings.NewReader(status))
	if err != nil {
		t.Fatal(err)
	}
	if rss != 128*MiB || hwm != 256*MiB {
		t.Fatalf("rss, hwm = %s, %s; want 128 and 256 MiB", FormatSize(rss), FormatSize(hwm))
	}
	if _, _, err := ProcStatus(strings.NewReader("Name:\ttix\nVmRSS:\t 1 kB\n")); err == nil {
		t.Fatal("a status without VmHWM was accepted")
	}
	if _, _, err := ProcStatus(strings.NewReader("VmRSS:\t 1 MB\nVmHWM:\t 1 kB\n")); err == nil {
		t.Fatal("a status in an unexpected unit was accepted")
	}
}

func TestReadProcReadsThisProcess(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc, which only Linux has")
	}
	rss, hwm, err := readProc(os.Getpid())
	if err != nil || rss <= 0 || hwm < rss {
		t.Fatalf("readProc(self) = %d, %d, %v", rss, hwm, err)
	}
}

func TestParseGCTrace(t *testing.T) {
	tests := []struct {
		line string
		want int64
		ok   bool
	}{
		{"gc 12 @3.104s 2%: 0.021+1.2+0.003 ms clock, 0.25+0.4/2.1/0+0.04 ms cpu, 9->10->5 MB, 11 MB goal, 0 MB stacks, 0 MB globals, 12 P", 5 * MiB, true},
		{"gc 1 @0.010s 1%: 0.01+0.2+0.01 ms clock, 0.1+0/0.1/0+0.1 ms cpu, 4->4->137 MB, 4 MB goal, 0 MB stacks, 0 MB globals, 8 P (forced)", 137 * MiB, true},
		{"time=2026-11-09 level=WARN msg=slow", 0, false},
	}
	for _, tt := range tests {
		got, ok := ParseGCTrace(tt.line)
		if ok != tt.ok || got != tt.want {
			t.Errorf("ParseGCTrace(%q) = %d, %v; want %d, %v", tt.line, got, ok, tt.want, tt.ok)
		}
	}
}

func TestHeapWatchKeepsThePeakAndTheLog(t *testing.T) {
	h := &heapWatch{}
	h.consume(strings.NewReader("gc 1 @0.1s 1%: x ms cpu, 4->4->20 MB, 8 MB goal\nserver says hello\ngc 2 @0.2s 1%: x ms cpu, 4->4->7 MB, 8 MB goal\n"))
	live, peak := h.current()
	if live != 7*MiB || peak != 20*MiB {
		t.Fatalf("live, peak = %s, %s; want 7 and 20 MiB", FormatSize(live), FormatSize(peak))
	}
	if h.log() != "server says hello" {
		t.Fatalf("log = %q, want only the line that is not gctrace", h.log())
	}
}
