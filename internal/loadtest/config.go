// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

// Kind is one behaviour a virtual user follows.
type Kind string

// The behaviours a virtual user can follow.
const (
	// KindBrowser signs in with a password, reads pages and assets, and keeps
	// a WebSocket open on the event stream.
	KindBrowser Kind = "browser"
	// KindMobile long-polls the event stream with a token, as the phone app does.
	KindMobile Kind = "mobile"
	// KindAgent lists, reads, creates, updates and transitions tasks and comments.
	KindAgent Kind = "agent"
)

// Kinds lists every behaviour in the order a mix assigns them.
var Kinds = []Kind{KindBrowser, KindMobile, KindAgent}

// DefaultMix is two browsers for each phone and each agent.
const DefaultMix = "browser=2,mobile=1,agent=1"

// MiB is one mebibyte.
const MiB = 1 << 20

// DefaultMaxRSS is the budget for the default four-user profile.
const DefaultMaxRSS = 128 * MiB

// Mix weights the behaviours virtual users are given.
type Mix map[Kind]int

// ParseMix reads "browser=2,mobile=1,agent=1". A kind left out weighs zero.
func ParseMix(s string) (Mix, error) {
	m := Mix{}
	for part := range strings.SplitSeq(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		name, weight, ok := strings.Cut(part, "=")
		if !ok {
			return nil, fmt.Errorf("mix entry %q is not kind=weight", part)
		}
		k := Kind(strings.TrimSpace(name))
		if !k.valid() {
			return nil, fmt.Errorf("mix names unknown kind %q (have browser, mobile, agent)", name)
		}
		w, err := strconv.Atoi(strings.TrimSpace(weight))
		if err != nil || w < 0 {
			return nil, fmt.Errorf("mix weight %q for %s is not a whole number", weight, k)
		}
		m[k] = w
	}
	if m.total() == 0 {
		return nil, fmt.Errorf("mix %q gives every kind a weight of zero", s)
	}
	return m, nil
}

func (k Kind) valid() bool {
	for _, known := range Kinds {
		if k == known {
			return true
		}
	}
	return false
}

func (m Mix) total() int {
	n := 0
	for _, w := range m {
		n += w
	}
	return n
}

// Assign spreads n users over the mix by largest remainder, so four users on
// the default mix are two browsers, one phone and one agent, and every kind
// with a weight gets at least one user once n allows it.
func (m Mix) Assign(n int) []Kind {
	if n <= 0 {
		return nil
	}
	total := m.total()
	counts := map[Kind]int{}
	type rem struct {
		k Kind
		r int
	}
	var rems []rem
	given := 0
	for _, k := range Kinds {
		w := m[k]
		if w == 0 {
			continue
		}
		counts[k] = n * w / total
		given += counts[k]
		rems = append(rems, rem{k, n * w % total})
	}
	sort.SliceStable(rems, func(i, j int) bool { return rems[i].r > rems[j].r })
	for i := 0; given < n; i++ {
		counts[rems[i%len(rems)].k]++
		given++
	}
	out := make([]Kind, 0, n)
	for _, k := range Kinds {
		for range counts[k] {
			out = append(out, k)
		}
	}
	return out
}

// String renders the mix in the form ParseMix reads.
func (m Mix) String() string {
	var parts []string
	for _, k := range Kinds {
		if w, ok := m[k]; ok {
			parts = append(parts, fmt.Sprintf("%s=%d", k, w))
		}
	}
	return strings.Join(parts, ",")
}

// ParseSize reads a byte size: 128MiB, 512KiB, 1GiB, 64MB or a bare number of bytes.
// Zero disables the budget.
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	units := []struct {
		suffix string
		mult   int64
	}{
		{"GiB", 1 << 30}, {"MiB", 1 << 20}, {"KiB", 1 << 10},
		{"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3}, {"B", 1},
	}
	for _, u := range units {
		if num, ok := strings.CutSuffix(s, u.suffix); ok {
			v, err := strconv.ParseFloat(strings.TrimSpace(num), 64)
			if err != nil || v < 0 {
				return 0, fmt.Errorf("size %q is not a number of %s", s, u.suffix)
			}
			return int64(v * float64(u.mult)), nil
		}
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil || v < 0 {
		return 0, fmt.Errorf("size %q needs a unit such as MiB", s)
	}
	return v, nil
}

// FormatSize renders bytes in MiB with one decimal.
func FormatSize(b int64) string {
	return fmt.Sprintf("%.1f MiB", float64(b)/MiB)
}

// Seed sizes the dataset a local run writes before the load starts.
type Seed struct {
	Tenants        int
	UsersPerTenant int
	Projects       int
	Tasks          int
	MinBody        int
	MaxBody        int
}

// DefaultSeed is three tenants, four people each, and a thousand tasks with
// descriptions between 5 and 50 KB.
var DefaultSeed = Seed{Tenants: 3, UsersPerTenant: 4, Projects: 4, Tasks: 1000, MinBody: 5 << 10, MaxBody: 50 << 10}

// Config is one load run.
type Config struct {
	Users    int
	Duration time.Duration
	Mix      Mix
	// Think is the mean pause between two actions of one user.
	Think time.Duration
	// MaxRSS fails the run when the server's peak resident memory exceeds it.
	// Zero disables the check.
	MaxRSS int64
	// Sample is how often the server's memory is read.
	Sample time.Duration
	// Warmup is the share of the run left out of the steady-state figure.
	Warmup float64
	Seed   Seed
}

// Validate checks the run makes sense before anything starts.
func (c Config) Validate() error {
	switch {
	case c.Users < 1:
		return fmt.Errorf("--users must be at least 1, not %d", c.Users)
	case c.Duration <= 0:
		return fmt.Errorf("--duration must be positive, not %s", c.Duration)
	case c.Mix.total() == 0:
		return fmt.Errorf("the mix gives every kind a weight of zero")
	case c.Think < 0:
		return fmt.Errorf("--think must not be negative")
	case c.Warmup < 0 || c.Warmup >= 1:
		return fmt.Errorf("--warmup must be a fraction in [0, 1), not %v", c.Warmup)
	}
	return nil
}
