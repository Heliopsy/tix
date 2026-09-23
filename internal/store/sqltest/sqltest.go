// SPDX-License-Identifier: AGPL-3.0-or-later

// Package sqltest holds the filter conformance corpus both storage engines are
// measured against. It lives outside either engine on purpose: a weak match or
// a negated term that selected different tasks on SQLite and on PostgreSQL
// would be a product difference nobody chose, and a table duplicated into two
// packages drifts the first time somebody edits one of them.
package sqltest

import (
	"context"
	"fmt"
	"sort"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Task is one row of the corpus, in the shape both engines' fixtures seed.
type Task struct {
	Title    string
	Body     string
	Status   string
	Priority core.Priority
	Tags     []string
}

// Corpus is the task set every conformance case selects from. The values are
// deliberately awkward: a title holding LIKE wildcards, a body repeating a
// word the title does not, and two decoy titles that a wildcard left
// unescaped would wrongly match.
func Corpus() []Task {
	return []Task{
		{Title: "Deploy the API gateway", Body: "roll out the edge", Status: "todo",
			Priority: core.PriorityHigh, Tags: []string{"ops"}},
		{Title: "rotate API keys", Body: "quarterly rotation", Status: "doing",
			Priority: core.PriorityNormal, Tags: []string{"sec"}},
		{Title: "write runbook", Body: "document the API gateway rollout", Status: "todo",
			Priority: core.PriorityLow, Tags: []string{"ops", "docs"}},
		{Title: "100% uptime_goal", Body: "aspirational", Status: "todo",
			Priority: core.PriorityLowest},
		// Decoys for the wildcard cases. An unescaped "_" in a needle matches
		// any single character and an unescaped "%" matches any run of them,
		// so a value that is meant to be literal would pull these in too.
		{Title: "uptimeXgoal", Body: "decoy for the underscore wildcard", Status: "todo",
			Priority: core.PriorityLowest},
		{Title: "1000 things", Body: "decoy for the percent wildcard", Status: "todo",
			Priority: core.PriorityLowest},
	}
}

// Case is one filter and the titles it must select, in any order.
type Case struct {
	Name   string
	Filter core.TaskFilter
	Want   []string
}

// Cases returns the conformance corpus. Every case is answered by both engines
// with the same rows; a case that cannot be is not added here, it is documented
// as an engine difference instead.
func Cases() []Case {
	contains := func(field core.TextField, v string, negate bool) core.TextTerm {
		return core.TextTerm{Field: field, Mode: core.MatchContains, Value: v, Negate: negate}
	}
	exact := func(field core.TextField, v string, negate bool) core.TextTerm {
		return core.TextTerm{Field: field, Mode: core.MatchExact, Value: v, Negate: negate}
	}
	return []Case{
		{"weak title match is a substring", core.TaskFilter{Text: []core.TextTerm{contains(core.TextTitle, "api", false)}},
			[]string{"Deploy the API gateway", "rotate API keys"}},
		{"weak match ignores case", core.TaskFilter{Text: []core.TextTerm{contains(core.TextTitle, "DEPLOY", false)}},
			[]string{"Deploy the API gateway"}},
		{"weak body match reaches text the title lacks", core.TaskFilter{Text: []core.TextTerm{contains(core.TextBody, "gateway", false)}},
			[]string{"write runbook"}},
		{"weak any match spans title and body", core.TaskFilter{Text: []core.TextTerm{contains(core.TextAny, "gateway", false)}},
			[]string{"Deploy the API gateway", "write runbook"}},
		{"negated weak match excludes", core.TaskFilter{Text: []core.TextTerm{contains(core.TextTitle, "api", true)}},
			[]string{"write runbook", "100% uptime_goal", "uptimeXgoal", "1000 things"}},
		{"exact title match is the whole field", core.TaskFilter{Text: []core.TextTerm{exact(core.TextTitle, "rotate api keys", false)}},
			[]string{"rotate API keys"}},
		{"exact title match rejects a substring", core.TaskFilter{Text: []core.TextTerm{exact(core.TextTitle, "api", false)}}, nil},
		{"a percent sign is a character, not a wildcard", core.TaskFilter{Text: []core.TextTerm{contains(core.TextTitle, "100%", false)}},
			[]string{"100% uptime_goal"}},
		{"an underscore is a character, not a wildcard", core.TaskFilter{Text: []core.TextTerm{contains(core.TextTitle, "uptime_goal", false)}},
			[]string{"100% uptime_goal"}},
		{"two weak terms are ANDed", core.TaskFilter{Text: []core.TextTerm{
			contains(core.TextAny, "api", false), contains(core.TextAny, "runbook", false)}},
			[]string{"write runbook"}},
		{"excluded status is dropped", core.TaskFilter{Exclude: core.TaskExclude{Statuses: []string{"todo"}}},
			[]string{"rotate API keys"}},
		{"excluded tag is dropped, and an untagged task is kept",
			core.TaskFilter{Exclude: core.TaskExclude{Tags: []string{"ops"}}},
			[]string{"rotate API keys", "100% uptime_goal", "uptimeXgoal", "1000 things"}},
		{"excluding every tag keeps only the untagged",
			core.TaskFilter{Exclude: core.TaskExclude{Tags: []string{"ops", "sec", "docs"}}},
			[]string{"100% uptime_goal", "uptimeXgoal", "1000 things"}},
		{"excluded priority is dropped",
			core.TaskFilter{Exclude: core.TaskExclude{Priorities: []core.Priority{core.PriorityHigh, core.PriorityNormal}}},
			[]string{"write runbook", "100% uptime_goal", "uptimeXgoal", "1000 things"}},
		{"an exclusion beats the same inclusion", core.TaskFilter{
			Statuses: []string{"todo"}, Exclude: core.TaskExclude{Statuses: []string{"todo"}}}, nil},
		{"exclusion keeps a row whose column is null",
			core.TaskFilter{Exclude: core.TaskExclude{AssigneeIDs: []string{"nobody"}}},
			[]string{"Deploy the API gateway", "rotate API keys", "write runbook", "100% uptime_goal",
				"uptimeXgoal", "1000 things"}},
		{"inclusion and exclusion combine",
			core.TaskFilter{Statuses: []string{"todo"}, Exclude: core.TaskExclude{Tags: []string{"docs"}}},
			[]string{"Deploy the API gateway", "100% uptime_goal", "uptimeXgoal", "1000 things"}},
	}
}

// Seed writes the corpus into a tenant, attaching each task's tags.
func Seed(ctx context.Context, s store.Store, scope core.TenantScope, projectID, actorID string) error {
	return s.Update(ctx, scope, func(tx store.Tx) error {
		for _, want := range Corpus() {
			task := core.Task{
				ProjectID: projectID, Title: want.Title, Body: want.Body,
				Status: want.Status, Priority: want.Priority, CreatorActorID: actorID,
			}
			if err := tx.CreateTask(ctx, &task); err != nil {
				return fmt.Errorf("creating %q: %w", want.Title, err)
			}
			for _, name := range want.Tags {
				tag := core.Tag{Name: name}
				if err := tx.PutTag(ctx, &tag); err != nil {
					return fmt.Errorf("putting tag %q: %w", name, err)
				}
				if err := tx.AttachTag(ctx, task.ID, tag.ID); err != nil {
					return fmt.Errorf("attaching tag %q: %w", name, err)
				}
			}
		}
		return nil
	})
}

// Titles returns the sorted titles of a result page, for comparison with Want.
func Titles(tasks []core.Task) []string {
	out := make([]string, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.Title)
	}
	sort.Strings(out)
	return out
}

// Sorted returns want sorted, so a case may list its titles readably.
func Sorted(want []string) []string {
	out := append([]string(nil), want...)
	sort.Strings(out)
	return out
}
