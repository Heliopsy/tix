// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// quickBox is the one-line add form as the listing draws it.
func quickBox(t *testing.T, page string) string {
	t.Helper()
	box := between(t, page, `<form method="post" action="/tasks" class="quickadd">`, "</form>")
	if box == "" {
		t.Fatalf("the listing has no one-line add box")
	}
	return box
}

// quickPreviewRow is the row under the box, up to the next region.
func quickPreviewRow(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `<div id="quickadd-preview"`)
	if start < 0 {
		t.Fatalf("the listing has no preview row under the add box")
	}
	rest := page[start:]
	end := strings.Index(rest, `<div id="addmore"`)
	if end < 0 {
		end = strings.Index(rest, `<div id="filterpop"`)
	}
	return rest[:end]
}

// taskOf reads a created task back through the service.
func (f *fixture) taskOf(t *testing.T, ref string) *core.Task {
	t.Helper()
	parsed, err := core.ParseTaskRef(ref)
	if err != nil {
		t.Fatalf("parsing %q: %v", ref, err)
	}
	task, err := f.svc.GetTask(f.ctx(), parsed)
	if err != nil {
		t.Fatalf("reading %s: %v", ref, err)
	}
	return task
}

func TestQuickAddBoxReadsTheSyntax(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	seedProject(t, f.store, f.tenantA.ID, "web")
	raj := seedActor(t, f.store, f.tenantA.ID, "raj", core.RoleAdmin)
	b := f.as("alice")

	resp := b.post("/tasks", url.Values{"quick": {"1"}, "project_ref": {"infra"},
		"title": {"Fix login !high #auth @raj due:2026-11-20 +web status:doing"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	ref := createdRef(resp.Header.Get("Location"))
	if !strings.HasPrefix(ref, "web-") {
		t.Fatalf("the line said +web and the task is %q", ref)
	}
	task := f.taskOf(t, ref)
	if task.Title != "Fix login" || task.Priority != core.PriorityHigh || task.Status != "doing" {
		t.Errorf("title %q, priority %d, status %q", task.Title, task.Priority, task.Status)
	}
	if !slices.Equal(task.Tags, []string{"auth"}) || task.AssigneeActorID != raj.ID {
		t.Errorf("tags %q, assignee %q (raj is %s)", task.Tags, task.AssigneeActorID, raj.ID)
	}
	if task.DueAt == nil || task.DueAt.Format(time.DateOnly) != "2026-11-20" {
		t.Errorf("due %v", task.DueAt)
	}
}

// The picker beside the box is where the task goes when the line names no
// project, and Add with details, which has a field per attribute, stores its
// title as typed.
func TestQuickAddDefaultsAndTheDetailsFormStayLiteral(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/tasks", url.Values{"quick": {"1"}, "project_ref": {"infra"}, "title": {"Fix login #auth"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if task := f.taskOf(t, createdRef(resp.Header.Get("Location"))); task.Title != "Fix login" ||
		!strings.HasPrefix(task.Ref, "infra-") {
		t.Errorf("a line naming no project made %s %q", task.Ref, task.Title)
	}

	resp = b.post("/tasks", url.Values{"project_ref": {"infra"}, "title": {"Fix login #auth !high"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if task := f.taskOf(t, createdRef(resp.Header.Get("Location"))); task.Title != "Fix login #auth !high" ||
		len(task.Tags) != 0 || task.Priority != core.PriorityNormal {
		t.Errorf("Add with details parsed its title: %q tags %q priority %d", task.Title, task.Tags, task.Priority)
	}
}

func TestQuickAddRefusalKeepsTheLineInTheBox(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	tests := []struct {
		name, line, mention string
	}{
		{"unknown handle", "Fix login @nobody", "nobody"},
		{"unknown project", "Fix login +nope", "nope"},
		{"unknown parent", "Fix login ^infra-99", "infra-99"},
		{"bad date", "Fix login due:someday", "someday"},
		{"two priorities", "Fix login !high !low", "!low"},
		{"no title", "#auth", "#auth"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := b.post("/tasks", url.Values{"quick": {"1"}, "project_ref": {"infra"}, "title": {tt.line}})
			page := body(t, resp)
			if resp.StatusCode < 400 || resp.StatusCode >= 500 {
				t.Fatalf("a refused line answered %d", resp.StatusCode)
			}
			box := quickBox(t, page)
			if !strings.Contains(html.UnescapeString(box), `value="`+tt.line+`"`) {
				t.Errorf("the box lost the line %q:\n%s", tt.line, box)
			}
			row := quickPreviewRow(t, page)
			refusal := between(t, row, `<p class="field-error" id="quickadd-error" role="alert">`, "</p>")
			if !strings.Contains(html.UnescapeString(refusal), tt.mention) {
				t.Errorf("the refusal under the box %q does not name %q", refusal, tt.mention)
			}
			if strings.Contains(page, `class="addpop is-inplace"`) {
				t.Errorf("a refused one-line add opened Add with details instead of answering under the box")
			}
		})
	}
	listed, err := f.svc.ListTasks(f.ctx(), core.TaskFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(listed.Tasks) != 0 {
		t.Errorf("refused lines created %d tasks", len(listed.Tasks))
	}
}

func TestQuickAddPreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	tests := []struct {
		name    string
		line    string
		want    []string
		without []string
	}{
		{"empty line says nothing", "", nil, []string{"qchip", "field-error"}},
		{"plain title says nothing", "buy milk", nil, []string{"qchip", "field-error"}},
		{"chips in order", "Fix !high #auth due:2026-11-20 ^infra-3 status:doing",
			[]string{`<span class="qchip-kind">Priority</span> high`, `<span class="qchip-kind">Tag</span> auth`,
				`<span class="qchip-kind">Due date</span> 2026-11-20`, `<span class="qchip-kind">Parent task</span> infra-3`,
				`<span class="qchip-kind">Status</span> doing`}, []string{"is-unknown", "field-error"}},
		{"known project and handle", "Fix +infra @alice", []string{"Project</span> infra", "Assignee</span> alice"},
			[]string{"is-unknown"}},
		{"unknown project", "Fix +nope", []string{`qchip is-unknown`, "no such project"}, nil},
		{"unknown handle", "Fix @nobody", []string{`qchip is-unknown`, "nobody has this handle"}, nil},
		{"refusal", "Fix due:someday", []string{`id="quickadd-error"`, "someday"}, []string{"qchip"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := html.UnescapeString(b.page(web.RouteQuickAdd + "?title=" + url.QueryEscape(tt.line)))
			last := -1
			for _, w := range tt.want {
				at := strings.Index(got, w)
				if at < 0 {
					t.Errorf("the preview lacks %q:\n%s", w, got)
					continue
				}
				if at < last {
					t.Errorf("the preview put %q out of the order typed:\n%s", w, got)
				}
				last = at
			}
			for _, w := range tt.without {
				if strings.Contains(got, w) {
					t.Errorf("the preview has %q:\n%s", w, got)
				}
			}
		})
	}
}

// "today" is the reader's today, in the zone chosen under Settings. Auckland
// and Los Angeles are twenty-one hours apart, so for most of a day a handler
// counting in one zone gets one of them wrong; quickadd_internal_test.go pins it.
func TestQuickAddCountsDaysInTheReadersZone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	for _, zone := range []string{"Pacific/Auckland", "America/Los_Angeles"} {
		loc, err := time.LoadLocation(zone)
		if err != nil {
			t.Fatalf("loading %s: %v", zone, err)
		}
		b := f.as("alice")
		resp := b.post("/timezone", url.Values{"timezone": {zone}, "next": {"/tasks"}})
		_ = resp.Body.Close()
		got := b.page(web.RouteQuickAdd + "?title=" + url.QueryEscape("x due:today"))
		if want := time.Now().In(loc).Format(time.DateOnly); !strings.Contains(got, want) {
			t.Errorf("due:today for a reader in %s previewed as\n%s\nwant %s", zone, got, want)
		}
	}
}

// The box asks for the preview as the reader types, and its tip names every
// attribute the parser reads.
func TestQuickAddBoxOffersThePreviewAndTheSyntax(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	box := quickBox(t, f.as("alice").page("/tasks"))
	for _, attr := range []string{`hx-get="` + web.RouteQuickAdd + `"`, `hx-target="#quickadd-preview"`,
		`name="quick" value="1"`, `aria-describedby="quickadd-preview"`} {
		if !strings.Contains(box, attr) {
			t.Errorf("the box lacks %s:\n%s", attr, box)
		}
	}
	tip := html.UnescapeString(between(t, box, `<details class="field-info tip">`, "</details>"))
	for _, form := range []string{"!high", "!1", "#tag", "@handle", "due:fri", "due:tomorrow", "due:+3d",
		"due:2026-11-20", "+project", "^infra-3", "status:doing", "backslash", "quotes"} {
		if !strings.Contains(tip, form) {
			t.Errorf("the quick add tip does not mention %s:\n%s", form, tip)
		}
	}
}
