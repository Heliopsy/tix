// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// dueBacklog seeds one project with three tasks: one whose deadline is long
// past, one whose deadline is far enough out that no clock this test can run
// under will have reached it, and one carrying no deadline at all.
//
// The dates are absolute rather than relative to now because the question
// under test is which of them a bound selects, and a fixture that drifts with
// the wall clock cannot answer it.
func dueBacklog(t *testing.T) *cli {
	t.Helper()
	c := newCLI(t)
	c.mustRun("project", "create", "infra", "Infrastructure")
	c.mustRun("task", "add", "-p", "infra", "--due", "2001-01-01", "rotate the keys")
	c.mustRun("task", "add", "-p", "infra", "--due", "2099-01-01", "replace the racks")
	c.mustRun("task", "add", "-p", "infra", "someday, tidy the cupboard")
	return c
}

// refsOf reads the references out of an ndjson listing.
func refsOf(t *testing.T, out string) []string {
	t.Helper()
	var refs []string
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		_, rest, ok := strings.Cut(line, `"title":"`)
		if !ok {
			continue
		}
		title, _, ok := strings.Cut(rest, `"`)
		if !ok {
			t.Fatalf("a title is never closed in %q", line)
		}
		refs = append(refs, title)
	}
	return refs
}

// TestTaskLsSelectsByDeadline is the gap this change exists for: the bounds
// reached the HTTP API and the command line could not ask for them, so "what
// is overdue" was answerable over the wire and not from the terminal, which is
// backwards for a tool whose primary users are agents on a command line.
func TestTaskLsSelectsByDeadline(t *testing.T) {
	c := dueBacklog(t)
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"overdue", []string{"--overdue"}, []string{"rotate the keys"}},
		{"the expression says the same thing", []string{"--filter", "due:overdue"},
			[]string{"rotate the keys"}},
		{"a bound before", []string{"--due-before", "2050-01-01"}, []string{"rotate the keys"}},
		{"a bound after", []string{"--due-after", "2050-01-01"}, []string{"replace the racks"}},
		{"the expression's bounded form", []string{"--filter", "due:>2050-01-01"},
			[]string{"replace the racks"}},
		{"a window that catches neither", []string{"--filter", "due:today"}, []string{"rotate the keys"}},
		{"no deadline term lists everything", nil,
			[]string{"rotate the keys", "replace the racks", "someday, tidy the cupboard"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"task", "ls", "-p", "infra", "-o", "ndjson"}, tc.args...)
			got := refsOf(t, c.mustRun(args...).out)
			if len(got) != len(tc.want) {
				t.Fatalf("tix %s listed %v, want %v", strings.Join(args, " "), got, tc.want)
			}
			for _, want := range tc.want {
				if !contains(got, want) {
					t.Fatalf("tix %s listed %v, want it to hold %q", strings.Join(args, " "), got, want)
				}
			}
		})
	}
}

// TestTaskLsRefusesContradictoryDeadlineFlags keeps a reader from getting a
// listing that answers neither of the two bounds they gave.
func TestTaskLsRefusesContradictoryDeadlineFlags(t *testing.T) {
	c := dueBacklog(t)
	for _, tc := range []struct {
		name string
		args []string
	}{
		{"overdue beside an explicit bound", []string{"--overdue", "--due-before", "2050-01-01"}},
		{"a bound that cannot hold anything", []string{"--due-before", "2001-01-01", "--due-after", "2050-01-01"}},
		{"a date that is not one", []string{"--due-before", "next tuesday"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := c.run(append([]string{"task", "ls", "-p", "infra"}, tc.args...)...)
			if got.code == core.ExitOK {
				t.Fatalf("tix task ls %v succeeded:\n%s", tc.args, got.out)
			}
		})
	}
}

// TestTheDeadlineFlagsBeatTheExpression pins the precedence --limit and --sort
// already set: the flag is the later and more specific word, so a reader who
// gave both is not silently handed the expression's answer.
func TestTheDeadlineFlagsBeatTheExpression(t *testing.T) {
	c := dueBacklog(t)
	// The expression alone selects only the long-past deadline. The flag names
	// the same bound and moves it out, so a listing that still holds one task
	// is one where the expression quietly won.
	got := refsOf(t, c.mustRun("task", "ls", "-p", "infra", "-o", "ndjson",
		"--filter", "due:<2001-06-01", "--due-before", "2099-06-01").out)
	if len(got) != 2 {
		t.Fatalf("listed %v, want the flag's bound to have replaced the expression's", got)
	}
}

func contains(values []string, want string) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}
