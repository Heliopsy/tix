package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// Every mutation but create used to live one screen down with nothing on the
// listing hinting it existed.
func TestProjectListingOffersEveryProjectControl(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/projects")
	for _, want := range []string{
		`action="/projects/infra/update"`,
		`action="/projects/infra/archive"`,
		`action="/projects/infra/delete"`,
		`action="/projects"`,
	} {
		if !strings.Contains(page, want) {
			t.Errorf("the projects listing offers no %s:\n%s", want, page)
		}
	}
	if !strings.Contains(page, "<th>Manage</th>") {
		t.Error("the listing does not name the column carrying the controls")
	}
	if strings.Contains(page, "hx-post") || strings.Contains(page, "hx-get") {
		t.Error("a project control depends on scripting")
	}
}

// The per-row form carries the values it is editing, so a save from the
// listing does not blank what it did not show.
func TestProjectListingEditsInPlace(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	created := b.post("/projects", url.Values{"key": {"ops"}, "name": {"Ops"},
		"color": {"teal"}, "icon": {"\U0001F9EA"}, "description": {"keeps the lights on"}})
	defer func() { _ = created.Body.Close() }()
	wantStatus(t, created, http.StatusSeeOther)

	page := b.page("/projects")
	for _, want := range []string{`id="name-ops"`, `value="Ops"`, `id="color-ops"`,
		`value="teal" selected`, `id="icon-ops"`, "keeps the lights on", `id="workflow-ops"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the row's form does not carry %q:\n%s", want, page)
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
	if !strings.Contains(after, "keeps the lights on") {
		t.Error("saving from the listing blanked a field the row did not show")
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
	if strings.Contains(page, `action="/projects/infra/delete"`) {
		t.Error("the hidden column still rendered its controls")
	}
	if !strings.Contains(page, `href="/projects/infra"`) {
		t.Error("the row no longer names the project it stands for")
	}
}
