package web_test

import (
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

func TestParseFilterSharesTheCLIGrammar(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		in   string
		want core.TaskFilter
	}{
		{"empty", "", core.TaskFilter{}},
		{"project", "project:infra", core.TaskFilter{ProjectKeys: []string{"infra"}}},
		{"repeated", "status:todo status:doing",
			core.TaskFilter{Statuses: []string{"todo", "doing"}}},
		{"tag and assignee", "tag:ops assignee:alice",
			core.TaskFilter{Tags: []string{"ops"}, AssigneeIDs: []string{"alice"}}},
		{"creator", "creator:bob", core.TaskFilter{CreatorIDs: []string{"bob"}}},
		{"priority", "priority:2", core.TaskFilter{Priorities: []core.Priority{core.PriorityHigh}}},
		{"claimed", "claimed:yes", core.TaskFilter{Claimed: core.Yes}},
		{"unclaimed", "claimed:no", core.TaskFilter{Claimed: core.No}},
		{"blocked", "blocked:yes", core.TaskFilter{Blocked: core.Yes}},
		{"deleted", "deleted:yes", core.TaskFilter{IncludeDeleted: true}},
		{"parent", "parent:abcdefgh", core.TaskFilter{ParentID: "abcdefgh"}},
		{"free text", "database migration", core.TaskFilter{Query: "database migration"}},
		{"quoted text", `"two words" tag:ops`,
			core.TaskFilter{Tags: []string{"ops"}, Query: "two words"}},
		{"mixed", "project:infra urgent", core.TaskFilter{
			ProjectKeys: []string{"infra"}, Query: "urgent"}},
		{"custom field", "field.severity:high",
			core.TaskFilter{CustomFields: map[string]any{"severity": "high"}}},
		{"two custom fields", "field.severity:high field.team:infra",
			core.TaskFilter{CustomFields: map[string]any{"severity": "high", "team": "infra"}}},
		{"numeric custom field", "field.points:3",
			core.TaskFilter{CustomFields: map[string]any{"points": float64(3)}}},
		{"custom field keeps its case", "field.storyPoints:3",
			core.TaskFilter{CustomFields: map[string]any{"storyPoints": float64(3)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := web.ParseFilter(tc.in)
			if err != nil {
				t.Fatalf("parsing %q: %v", tc.in, err)
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("filter = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func TestParseFilterAcceptsDates(t *testing.T) {
	t.Parallel()
	got, err := web.ParseFilter("due-before:2026-01-02 due-after:2025-01-02")
	if err != nil {
		t.Fatalf("parsing dates: %v", err)
	}
	if got.DueBefore == nil || got.DueAfter == nil {
		t.Fatalf("filter = %+v, want both bounds set", got)
	}
	if got.DueBefore.Year() != 2026 || got.DueAfter.Year() != 2025 {
		t.Fatalf("filter bounds parsed wrongly: %+v", got)
	}
}

func TestParseFilterReportsMalformedExpressions(t *testing.T) {
	t.Parallel()
	cases := []string{
		"nosuchkey:value",
		"priority:high",
		"priority:9",
		"claimed:maybe",
		"status:",
		`tag:"unclosed`,
		"due-before:never",
		"field.:high",
		"field.bad$key:high",
		"field.severity:",
		"field.severity:null",
	}
	for _, expression := range cases {
		t.Run(expression, func(t *testing.T) {
			if _, err := web.ParseFilter(expression); err == nil {
				t.Fatalf("parsing %q succeeded, want a parse error", expression)
			} else if !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("error kind = %q, want invalid", core.KindOf(err))
			}
		})
	}
}

func TestTaskListAppliesTheFilterExpression(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "in the todo column")
	moved := b.createTask("infra", "moved along")

	resp := b.post("/projects/infra/move", url.Values{"ref": {moved}, "to": {"doing"}})
	_ = resp.Body.Close()

	page := b.page("/tasks?q=" + url.QueryEscape("status:doing"))
	if !strings.Contains(page, "moved along") {
		t.Fatalf("the filtered list omits the matching task")
	}
	if strings.Contains(page, "in the todo column") {
		t.Fatalf("the filtered list shows a task the filter excludes")
	}
}

func TestTaskListExplainsAMalformedFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "should not be listed")

	resp := b.get("/tasks?q=" + url.QueryEscape("nosuchkey:value"))
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	if strings.Contains(page, "should not be listed") {
		t.Fatalf("a malformed filter silently returned every task")
	}
	if !strings.Contains(page, "nosuchkey") {
		t.Fatalf("the parse error does not name the offending term:\n%s", page)
	}
}

func TestTaskListPaginatesByCursor(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, title := range []string{"alpha", "bravo", "charlie"} {
		b.createTask("infra", title)
	}

	first := b.page("/tasks?limit=2")
	if !strings.Contains(first, "alpha") {
		t.Fatalf("the first page omits the first task")
	}

	sorted := b.page("/tasks?sort=title")
	for _, title := range []string{"alpha", "bravo", "charlie"} {
		if !strings.Contains(sorted, title) {
			t.Fatalf("the sorted listing omits %q", title)
		}
	}

	invalid := b.get("/tasks?sort=nonsense")
	defer func() { _ = invalid.Body.Close() }()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for an unknown sort field", invalid.StatusCode)
	}
}
