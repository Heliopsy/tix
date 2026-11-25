// SPDX-License-Identifier: AGPL-3.0-or-later

// Package memlimit sets the Go runtime's soft memory limit from the container
// the process runs in, so the collector works harder as the heap nears the
// container's limit instead of letting it reach twice the live heap and the
// kernel kill the process.
package memlimit

import (
	"fmt"
	"os"
	"runtime/debug"
	"strconv"
	"strings"
)

// Ratio is the share of the container's limit the heap may grow to.
// The rest is for what the container is charged and the runtime does not
// count: the binary's pages, socket buffers, and the page cache charged for
// the database files it reads.
const Ratio = 0.85

// cgroupFiles are where the container's limit is read from: cgroup v2 first,
// then v1.
var cgroupFiles = []string{
	"/sys/fs/cgroup/memory.max",
	"/sys/fs/cgroup/memory/memory.limit_in_bytes",
}

// Env reads one environment variable.
type Env func(name string) (string, bool)

// ReadFile reads one file whole.
type ReadFile func(name string) ([]byte, error)

// Decision is the limit Decide chose and why.
type Decision struct {
	// Limit is the soft limit in bytes, 0 to leave the runtime's alone.
	Limit int64
	// Container is the container's limit it was taken from, 0 for none.
	Container int64
	// Reason says where it came from, or why there is none.
	Reason string
}

// Decide chooses the soft limit. GOMEMLIMIT, when set, is the runtime's own
// setting and wins, including "off". Otherwise the limit is Ratio of
// the cgroup memory limit, when there is one.
func Decide(env Env, read ReadFile) Decision {
	if v, ok := env("GOMEMLIMIT"); ok && strings.TrimSpace(v) != "" {
		return Decision{Reason: "GOMEMLIMIT is set to " + strings.TrimSpace(v)}
	}
	for _, name := range cgroupFiles {
		raw, err := read(name)
		if err != nil {
			continue
		}
		limit, ok := parseLimit(raw)
		if !ok {
			return Decision{Reason: "the container sets no memory limit"}
		}
		return Decision{Limit: int64(float64(limit) * Ratio), Container: limit,
			Reason: fmt.Sprintf("%.0f%% of the container's limit in %s", Ratio*100, name)}
	}
	return Decision{Reason: "no cgroup memory limit to read"}
}

// parseLimit reads a cgroup limit. "max", and v1's near-2^63 stand-in for no
// limit, are none.
func parseLimit(raw []byte) (int64, bool) {
	v := strings.TrimSpace(string(raw))
	if v == "" || v == "max" {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil || n <= 0 || n >= 1<<62 {
		return 0, false
	}
	return n, true
}

// Apply decides the limit from the process's environment and cgroup and sets
// it, returning what it decided.
func Apply() Decision {
	d := Decide(os.LookupEnv, os.ReadFile)
	if d.Limit > 0 {
		debug.SetMemoryLimit(d.Limit)
	}
	return d
}
