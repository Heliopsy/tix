// SPDX-License-Identifier: AGPL-3.0-or-later

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
			// Page is the shared parser's business, not this test's: the
			// handler overwrites it with the cursor and sort from the query
			// string straight after parsing. Comparing it here only pinned
			// the old web-only parser's habit of leaving it empty.
			got.Page = core.Page{}
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
		// "priority:high" belongs in the accepted set now. The old web-only
		// parser took numbers alone, so a name that worked on the command
		// line was rejected in the browser. Sharing the parser fixed that.
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
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want the listing the reader was on", resp.StatusCode)
	}
	if strings.Contains(page, "should not be listed") {
		t.Fatalf("a malformed filter silently returned every task")
	}
	if !strings.Contains(page, "nosuchkey") {
		t.Fatalf("the parse error does not name the offending term:\n%s", page)
	}
	if !strings.Contains(page, `class="filterfault"`) {
		t.Fatalf("the parse error is not reported on the filter bar:\n%s", page)
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

// TestFilterBarUnderstandsTheWholeLanguage is the regression for the bug this
// shim exists to kill.
//
// The browser used to run its own parser. When the language gained negation
// and weak matching, "-tag:ops" failed here with a clear message, which is
// survivable, but "title~api" was swallowed as free text: it became a Query
// nothing matched, so the board came back empty with no error at all. A person
// reads that as their tasks having disappeared.
//
// Every operator is checked for the property that matters. Not that it parses
// to some particular struct, but that it is either understood or refused, and
// never quietly turned into a different question.
func TestFilterBarUnderstandsTheWholeLanguage(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		expr string
		want func(core.TaskFilter) bool
	}{
		{"weak title", "title~api", func(f core.TaskFilter) bool { return len(f.Text) == 1 }},
		{"weak body", "body~gateway", func(f core.TaskFilter) bool { return len(f.Text) == 1 }},
		{"weak text", "text~deploy", func(f core.TaskFilter) bool { return len(f.Text) == 1 }},
		{"negated tag", "-tag:ops", func(f core.TaskFilter) bool { return len(f.Exclude.Tags) == 1 }},
		{"negated status", "-status:done", func(f core.TaskFilter) bool { return len(f.Exclude.Statuses) == 1 }},
		{"negated weak", "-title~wip", func(f core.TaskFilter) bool {
			return len(f.Text) == 1 && f.Text[0].Negate
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := web.ParseFilter(tc.expr)
			if err != nil {
				t.Fatalf("the filter bar refused %q: %v\n"+
					"the browser must understand every operator the CLI does", tc.expr, err)
			}
			if !tc.want(got) {
				t.Fatalf("the filter bar accepted %q and built nothing from it: %+v\n"+
					"a term swallowed into free text returns an empty board with no error, "+
					"which reads as missing data", tc.expr, got)
			}
			if got.Query != "" {
				t.Fatalf("%q landed in free text as %q, which is the silent-empty-board bug",
					tc.expr, got.Query)
			}
		})
	}
}

// A filter naming somebody this tenant does not have is a typo in a box. It
// used to be answered with the full-page error screen, which took the reader
// off the listing and threw away the expression they would have to correct.
func TestARefusedFilterReportsOnTheBarAndKeepsTheListing(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "still reachable")

	resp := b.get("/tasks?q=" + url.QueryEscape("assignee:nobody-here"))
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusOK)
	page := readAll(t, resp)

	if !strings.Contains(page, `class="filterfault"`) {
		t.Fatalf("a refused filter did not report on the filter bar:\n%s", page)
	}
	if strings.Contains(page, `<p class="flash error">`) {
		t.Errorf("a refused filter still renders the full error screen:\n%s", page)
	}
	if !strings.Contains(page, `name="q" value="assignee:nobody-here"`) {
		t.Errorf("the expression was thrown away, so it cannot be corrected:\n%s",
			between(t, page, `class="filterbar"`, "</form>"))
	}
	if !strings.Contains(page, `action="/tasks" class="filterbar"`) {
		t.Errorf("the reader was taken off the listing")
	}
	if strings.Contains(page, "Nothing on the list<") {
		t.Errorf("the empty listing claims there is no work rather than no answer")
	}
}

// Every way a filter term can be refused reports the same way, so the next
// term that learns to refuse does not need its own case here.
func TestEveryRefusedFilterTermReportsOnTheBar(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "still reachable")

	for _, expression := range []string{"assignee:nobody-here", "project:no-such-project",
		"priority:enormous", "due:yesterdayish", "|"} {
		resp := b.get("/tasks?q=" + url.QueryEscape(expression))
		page := readAll(t, resp)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%q answered %d, so a boosted browser swaps nothing", expression, resp.StatusCode)
			continue
		}
		if strings.Contains(page, `<p class="flash error">`) {
			t.Errorf("%q still renders the full error screen:\n%s", expression, page)
		}
		if !strings.Contains(page, `class="filterbar"`) {
			t.Errorf("%q took the reader off the listing", expression)
		}
		if !strings.Contains(page, `name="q" value="`+expression+`"`) {
			t.Errorf("%q was thrown away rather than left in the box", expression)
		}
	}
}

// A failure that is not the expression's fault still fails the page, so a
// filter box cannot swallow a real error.
func TestAnErrorThatIsNotTheFiltersFaultStillFailsThePage(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	resp := f.as("").get("/tasks?q=" + url.QueryEscape("status:todo"))
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusOK {
		t.Fatalf("an unauthenticated listing answered 200 with a filter in the box")
	}
}
