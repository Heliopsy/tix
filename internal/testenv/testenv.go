// SPDX-License-Identifier: AGPL-3.0-or-later

// Package testenv reports the optional capabilities a test run could not use,
// so a green suite says what it actually proved and not merely that it passed.
package testenv

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

// Marker prefixes the machine-readable line written for every unavailable
// capability, so a wrapper around `go test` can collect one summary for a run
// that spans many package binaries.
const Marker = "tix-test-env:"

// LogEnv names an optional file the report is appended to, one marked line per
// unavailable capability. It exists because `go test` prints nothing at all
// from a package whose tests pass, so a wrapper cannot read the report off
// stdout: `just test` points this at a scratch file and summarises it for the
// whole run. The path must be absolute, since a test binary runs with its own
// package directory as the working directory.
const LogEnv = "TIX_TEST_ENV_LOG"

// PostgresEnv names the environment variable that points the suite at a
// PostgreSQL database. Without it every PostgreSQL-backed test skips.
const PostgresEnv = "TIX_TEST_POSTGRES_DSN"

// PostgresHow is the command that makes the PostgreSQL suite runnable.
const PostgresHow = "just pg-up && just test-postgres"

// Capability is something the tests need from the environment but can do
// without: its absence skips tests rather than failing them.
type Capability struct {
	// Name identifies the capability, for example "postgres".
	Name string
	// Why states what is missing, as a fact about this machine.
	Why string
	// How is the command that makes the capability available.
	How string
}

var (
	mu      sync.Mutex
	missing = map[string]Capability{}
)

// Record notes that a capability was unavailable, without skipping anything.
// Recording the same name twice keeps the first reason.
func Record(c Capability) {
	mu.Lock()
	defer mu.Unlock()
	if _, seen := missing[c.Name]; !seen {
		missing[c.Name] = c
	}
}

// Skip records an unavailable capability and skips the calling test.
func Skip(tb testing.TB, c Capability) {
	tb.Helper()
	Record(c)
	tb.Skipf("skipped: %s unavailable (%s); enable with: %s", c.Name, c.Why, c.How)
}

// Missing returns every capability recorded so far, ordered by name.
func Missing() []Capability {
	mu.Lock()
	defer mu.Unlock()
	out := make([]Capability, 0, len(missing))
	for _, c := range missing {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// PostgresCapability describes the PostgreSQL engine suite, so every package
// that gates on a database reports the same gap in the same words.
func PostgresCapability() Capability {
	return Capability{
		Name: "postgres",
		Why:  PostgresEnv + " is not set, so only SQLite was exercised",
		How:  PostgresHow,
	}
}

// PostgresDSN returns the DSN of the test database, skipping the calling test
// when it is not configured. It is the single gate for the PostgreSQL suites.
func PostgresDSN(tb testing.TB) string {
	tb.Helper()
	dsn := strings.TrimSpace(os.Getenv(PostgresEnv))
	if dsn == "" {
		Skip(tb, PostgresCapability())
	}
	return dsn
}

// Report writes the summary of what this run could not prove. It writes
// nothing when the environment allowed everything.
func Report(w io.Writer) {
	gaps := Missing()
	if len(gaps) == 0 {
		return
	}
	_, _ = io.WriteString(w, render(gaps))
}

func render(gaps []Capability) string {
	var b strings.Builder
	rule := strings.Repeat("=", 72)
	b.WriteString("\n" + rule + "\n")
	fmt.Fprintf(&b, "PARTIAL RUN: %s unavailable, so part of this package's suite did not run.\n",
		plural(len(gaps)))
	b.WriteString("Passing here does NOT mean the skipped paths work.\n")
	for _, c := range gaps {
		fmt.Fprintf(&b, "  - %s: %s\n      enable with: %s\n", c.Name, c.Why, c.How)
	}
	b.WriteString(rule + "\n")
	b.WriteString(markers(gaps))
	return b.String()
}

func markers(gaps []Capability) string {
	var b strings.Builder
	for _, c := range gaps {
		fmt.Fprintf(&b, "%s SKIPPED | %s | %s | %s\n", Marker, c.Name, c.Why, c.How)
	}
	return b.String()
}

// AppendLog appends one marked line per unavailable capability to the file
// named by LogEnv. It does nothing when that variable is unset, and it never
// fails a run: a report that cannot be written is not worth an exit code.
func AppendLog() {
	gaps := Missing()
	path := strings.TrimSpace(os.Getenv(LogEnv))
	if len(gaps) == 0 || path == "" {
		return
	}
	// The path comes from LogEnv, which the justfile sets to a location inside
	// the repository. This helper is only ever linked into test binaries, so the
	// only person who can steer the path is the one already running the tests
	// and therefore already running arbitrary code. No trust boundary is crossed.
	// #nosec G304,G703 -- developer-controlled path in test-only code
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o600)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()
	_, _ = io.WriteString(f, markers(gaps))
}

func plural(n int) string {
	if n == 1 {
		return "1 capability was"
	}
	return fmt.Sprintf("%d capabilities were", n)
}

// Main runs a package's tests and then reports what the environment did not
// allow. Use it as the whole body of TestMain.
func Main(m *testing.M) {
	code := m.Run()
	AppendLog()
	Report(os.Stderr)
	os.Exit(code)
}
