package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// The listing used to carry a full edit form -- name, workflow, colour,
// icon, description, save, archive, delete -- squeezed into one narrow table
// cell: opening it produced a roughly 700px tall row with the form crammed
// into a sliver of width and the rest of the row empty. The project's own
// page (board.html) already carries that whole form properly, in a
// disclosure with the room to lay itself out, so the listing links through
// to it instead of duplicating it. Two places to edit the same project is
// also how they drift apart.
func TestProjectListingLinksThroughToEveryProjectControl(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/projects")
	for _, want := range []string{`href="/projects/infra"`, `action="/projects"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the projects listing offers no %s:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "<th>Manage</th>") {
		t.Error("the listing does not name the column carrying the link to a project's controls")
	}
	if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
		t.Error("a project control depends on scripting")
	}

	board := b.page("/projects/infra")
	for _, want := range []string{`action="/projects/infra/update"`,
		`action="/projects/infra/archive"`, `action="/projects/infra/delete"`} {
		if !strings.Contains(board, want) {
			t.Errorf("the project page offers no %s:\n%s", want, board)
		}
	}
}

// The project page's settings form is the only place a project is edited
// now, so it has to carry the whole record -- not just whatever the listing
// happened to show -- or saving there would blank what it did not carry.
func TestProjectPageSettingsFormCarriesTheWholeRecordAndSavingKeepsIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects", url.Values{"key": {"ops"}, "name": {"Ops"},
		"color": {"teal"}, "icon": {"\U0001F9EA"}, "description": {"keeps the lights on"}})
	defer func() { _ = created.Body.Close() }()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/projects/ops")
	for _, want := range []string{`value="Ops"`, `value="teal" selected`,
		"keeps the lights on", `id="workflow_key"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the settings form does not carry %q:\n%s", want, page)
		}
	}

	saved := b.post("/projects/ops/update", url.Values{"name": {"Operations"},
		"color": {"teal"}, "icon": {"\U0001F9EA"}, "description": {"keeps the lights on"},
		"workflow_key": {"default"}})
	defer func() { _ = saved.Body.Close() }()
	wantStatus(t, saved, http.StatusSeeOther)

	after := b.page("/projects")
	if !strings.Contains(after, "Operations") {
		t.Errorf("the edit did not take:\n%s", after)
	}
	reloaded := b.page("/projects/ops")
	if !strings.Contains(reloaded, "keeps the lights on") {
		t.Error("saving blanked a field the form did not change")
	}
}

// Hiding the column is a display choice; it must not be the only way to reach
// the controls, and it must not break the table.
func TestProjectManageColumnCanBeHidden(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/columns", url.Values{"page": {"projects"},
		"column": {"name"}, "next": {"/projects"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/projects")
	if strings.Contains(page, `data-label="Manage"`) {
		t.Error("the hidden column still rendered its cell")
	}
	if !strings.Contains(page, `href="/projects/infra"`) {
		t.Error("the row no longer names the project it stands for")
	}
}
