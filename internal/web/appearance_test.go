package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// The task list has to mark a project's rows without turning the colour into
// something a reader mistakes for a status.
func TestTaskListShowsTheProjectAccentAndIcon(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	if resp := b.post("/projects/infra/update", url.Values{
		"name": {"INFRA"}, "color": {"violet"}, "icon": {"\U0001F680"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("updating the project = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}
	if resp := b.post("/tasks", url.Values{
		"title": {"marked task"}, "project_ref": {"infra"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("creating a task = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}

	page := b.page("/tasks")
	if !strings.Contains(page, `class="task is-accented proj-violet"`) {
		t.Errorf("the task row carries no project accent:\n%s", page)
	}
	if !strings.Contains(page, `class="proj-icon"`) || !strings.Contains(page, "\U0001F680") {
		t.Errorf("the task row carries no project icon:\n%s", page)
	}
	if strings.Contains(page, `class="badge proj-violet"`) {
		t.Error("the project colour rendered as a status badge")
	}
}

// A project with neither still renders a plain row.
func TestTaskListRendersCleanlyWithoutAnAccent(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	if resp := b.post("/tasks", url.Values{
		"title": {"plain task"}, "project_ref": {"infra"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("creating a task = %d, want 303", resp.StatusCode)
	}

	page := b.page("/tasks")
	if !strings.Contains(page, "plain task") {
		t.Fatalf("the task is missing from the list:\n%s", page)
	}
	if strings.Contains(page, "is-accented") || strings.Contains(page, `class="proj-icon"`) {
		t.Errorf("a project with no colour or icon still marked its rows:\n%s", page)
	}
}

// Setting from the project screen, then clearing again, both have to stick.
func TestProjectScreenSetsAndClearsTheAppearance(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	if resp := b.post("/projects", url.Values{
		"key": {"ops"}, "name": {"Ops"}, "color": {"teal"}, "icon": {"\U0001F9EA"},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("creating a project = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}
	list := b.page("/projects")
	if !strings.Contains(list, `class="swatch proj-teal"`) {
		t.Errorf("the project list shows no swatch:\n%s", list)
	}

	board := b.page("/projects/ops")
	if !strings.Contains(board, `value="teal" selected`) {
		t.Errorf("the project form does not preselect the saved colour:\n%s", board)
	}
	if !strings.Contains(board, `value="\U0001F9EA"`) && !strings.Contains(board, "\U0001F9EA") {
		t.Errorf("the project form does not carry the saved icon:\n%s", board)
	}

	if resp := b.post("/projects/ops/update", url.Values{
		"name": {"Ops"}, "color": {""}, "icon": {""},
	}); resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("clearing = %d, want 303: %s", resp.StatusCode, body(t, resp))
	}
	cleared := b.page("/projects")
	if strings.Contains(cleared, "proj-teal") || strings.Contains(cleared, "\U0001F9EA") {
		t.Errorf("the cleared project still shows its appearance:\n%s", cleared)
	}
}

// A colour outside the palette is refused by the service, not merely by the
// form, so the browser gets the invalid screen rather than a saved value.
func TestProjectScreenRefusesAColourOutsideThePalette(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/projects/infra/update", url.Values{
		"name": {"INFRA"}, "color": {"#ff00ff"}, "icon": {""},
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != core.KindInvalid.HTTPStatus() {
		t.Fatalf("invalid colour = %d, want %d", resp.StatusCode, core.KindInvalid.HTTPStatus())
	}
	if page := b.page("/projects"); strings.Contains(page, "#ff00ff") {
		t.Errorf("the refused colour reached a page:\n%s", page)
	}
}
