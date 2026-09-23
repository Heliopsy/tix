// SPDX-License-Identifier: AGPL-3.0-or-later

package query

import (
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// The terminal interface filters a loaded page in memory while the store
// filters the same expression in SQL. A task the store would have returned
// must match here, or the filter bar hides rows `tix task ls` shows.
func TestMatchesAgreesWithTheExpression(t *testing.T) {
	clk := clock.NewFakeAt()
	now := clk.Now()
	tasks := map[string]core.Task{
		"gateway": {Title: "Deploy the API gateway", Body: "roll out the edge",
			Status: "todo", Priority: core.PriorityHigh, Tags: []string{"ops"}},
		"keys": {Title: "rotate API keys", Body: "quarterly rotation",
			Status: "doing", Priority: core.PriorityNormal, Tags: []string{"sec"}},
		"runbook": {Title: "write runbook", Body: "document the API gateway rollout",
			Status: "todo", Priority: core.PriorityLow, Tags: []string{"ops", "docs"}},
	}

	tests := []struct {
		name string
		expr string
		want []string
	}{
		{"weak title match", "title~api", []string{"gateway", "keys"}},
		{"weak match ignores case", "title~API", []string{"gateway", "keys"}},
		{"weak body match", "body~gateway", []string{"runbook"}},
		{"weak any match", "text~gateway", []string{"gateway", "runbook"}},
		{"negated weak match", "-title~api", []string{"runbook"}},
		{"exact title match", `title:"rotate api keys"`, []string{"keys"}},
		{"exact rejects a substring", "title:api", nil},
		{"excluded tag", "-tag:ops", []string{"keys"}},
		{"excluded status", "-status:todo", []string{"keys"}},
		{"excluded priority", "-priority:high", []string{"keys", "runbook"}},
		{"exclusion beats the same inclusion", "tag:ops -tag:ops", nil},
		{"inclusion with exclusion", "status:todo -tag:docs", []string{"gateway"}},
		{"two weak terms are ANDed", "text~api text~runbook", []string{"runbook"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("parsing %q: %v", tt.expr, err)
			}
			var got []string
			for _, key := range []string{"gateway", "keys", "runbook"} {
				if Matches(f, tasks[key], "alpha", now) {
					got = append(got, key)
				}
			}
			if !sameKeys(got, tt.want) {
				t.Errorf("%q matched %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func TestMatchesHonoursTheNonTextTerms(t *testing.T) {
	clk := clock.NewFakeAt()
	now := clk.Now()
	due := now.Add(48 * time.Hour)
	deleted := now.Add(-time.Hour)

	base := core.Task{Title: "t", Status: "todo", Priority: core.PriorityNormal, DueAt: &due}
	gone := base
	gone.DeletedAt = &deleted

	tests := []struct {
		name string
		expr string
		task core.Task
		want bool
	}{
		{"a deleted task is hidden by default", "", gone, false},
		{"is:deleted reveals it", "is:deleted", gone, true},
		{"a due bound that holds", "due-before:" + due.Add(time.Hour).Format(time.RFC3339), base, true},
		{"a due bound that does not", "due-before:" + due.Add(-time.Hour).Format(time.RFC3339), base, false},
		{"a project key that matches", "project:alpha", base, true},
		{"a project key that does not", "project:beta", base, false},
		{"an excluded project key", "-project:alpha", base, false},
		{"root only, with no parent", "is:root", base, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, err := Parse(tt.expr)
			if err != nil {
				t.Fatalf("parsing %q: %v", tt.expr, err)
			}
			if got := Matches(f, tt.task, "alpha", now); got != tt.want {
				t.Errorf("%q matched = %v, want %v", tt.expr, got, tt.want)
			}
		})
	}
}

func sameKeys(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
