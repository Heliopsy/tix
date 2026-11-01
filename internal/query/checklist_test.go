// SPDX-License-Identifier: AGPL-3.0-or-later

package query_test

import (
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/query"
	"github.com/heliopsy/tix/internal/store/sqltest"
)

// The checklist terms are answered in SQL by both engines against the shared
// corpus. The terminal answers them in memory, so it is held to the same
// corpus and the same expected titles.
func TestChecklistTermsMatchInMemoryAsTheStoresDo(t *testing.T) {
	t.Parallel()
	cases := map[string][]string{
		"has:checklist":   {"Deploy the API gateway", "rotate API keys"},
		"-has:checklist":  {"write runbook", "100% uptime_goal", "uptimeXgoal", "1000 things"},
		"checklist:open":  {"rotate API keys"},
		"checklist:done":  {"Deploy the API gateway"},
		"-checklist:open": {"Deploy the API gateway", "write runbook", "100% uptime_goal", "uptimeXgoal", "1000 things"},
	}
	for expr, want := range cases {
		f, err := query.Parse(expr)
		if err != nil {
			t.Fatalf("Parse(%q): %v", expr, err)
		}
		var got []string
		for _, task := range sqltest.Corpus() {
			if query.Matches(f, core.Task{Title: task.Title, Body: task.Body}, "infra", time.Now()) {
				got = append(got, task.Title)
			}
		}
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("%q matched %v, want %v", expr, got, want)
		}
	}
}

func TestChecklistTermsRefuseWhatTheyCannotMean(t *testing.T) {
	t.Parallel()
	for _, expr := range []string{"has:tags", "checklist:half", "-checklist:done"} {
		if _, err := query.Parse(expr); err == nil {
			t.Errorf("Parse(%q) accepted a term it cannot answer", expr)
		}
	}
}
