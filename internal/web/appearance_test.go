// SPDX-License-Identifier: AGPL-3.0-or-later

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
	// The colour is the project's mark, not a swatch beside the word "teal"
	// in a Colour column: the listing used to print the name of the colour
	// as data. What has to hold is that the saved colour reaches the list.
	list := b.page("/projects")
	if !strings.Contains(list, `class="projrow proj-teal"`) {
		t.Errorf("the project list does not carry the saved colour:\n%s", list)
	}

	board := b.page("/projects/ops")
	if !strings.Contains(board, `value="teal" checked`) {
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
	if strings.Contains(cleared, `class="projrow proj-teal"`) || strings.Contains(cleared, "\U0001F9EA") {
		t.Errorf("the cleared project still shows its appearance:\n%s", cleared)
	}
}

// A colour outside the palette is refused by the service, not merely by the
// form, so the browser gets the invalid screen rather than a saved value.
func TestProjectScreenRefusesAColourOutsideThePalette(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/projects/infra/update", url.Values{
		"name": {"INFRA"}, "color": {"custom"}, "color_custom": {"#ff00f"}, "icon": {""},
	})
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != core.KindInvalid.HTTPStatus() {
		t.Fatalf("invalid colour = %d, want %d", resp.StatusCode, core.KindInvalid.HTTPStatus())
	}
	if page := b.page("/projects"); strings.Contains(page, "#ff00f") {
		t.Errorf("the refused colour reached a page:\n%s", page)
	}
}

// A theme names two accents, one for light schemes and a lighter one for the
// dark ones, and the second exists because the first is unreadable on a dark
// background. Both rules are emitted from the same struct, so a page carrying
// the accent somewhere says nothing about which rule got which colour.
func TestTheDarkSchemeGetsTheThemesOwnDarkAccent(t *testing.T) {
	t.Parallel()
	page := newFixture(t).as("alice").page("/projects")

	light := between(t, page, "\n:root { --accent: ", ";")
	dark := between(t, page, `:root[data-theme="dark"], :root[data-theme="dim"] { --accent: `, ";")
	if light == "" || dark == "" {
		t.Fatalf("the page emits no accent rules:\n%s", between(t, page, "<style>", "</style>"))
	}
	if light == dark {
		t.Errorf("the dark scheme is given the light accent %q, which is what it exists to replace", dark)
	}
}

// colourFieldOf is the colour control posting name, and nothing around it.
func colourFieldOf(t *testing.T, page, name string) string {
	t.Helper()
	field := between(t, page, `aria-labelledby="`+name+`-label" data-colour-field>`, "</div>")
	if field == "" {
		t.Fatalf("the page has no colour field for %q:\n%s", name, page)
	}
	return field
}

// Both project forms offer the palette, none, and a custom colour through
// the same control.
func TestProjectFormsOfferThePaletteAndACustomColour(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	for _, path := range []string{"/projects", "/projects/infra"} {
		field := colourFieldOf(t, b.page(path), "color")
		for _, c := range core.ProjectColors() {
			if !strings.Contains(field, `value="`+string(c)+`"`) {
				t.Errorf("%s: the colour field does not offer %s", path, c)
			}
		}
		for _, want := range []string{`name="color" value=""`, `name="color" value="custom"`,
			`type="color"`, `name="color_custom"`, "data-colour-preview"} {
			if !strings.Contains(field, want) {
				t.Errorf("%s: the colour field has no %s:\n%s", path, want, field)
			}
		}
	}
}

// A custom colour chosen in the browser is saved, shown as the current
// choice, and draws the project's rows, board cards and list entry.
func TestACustomColourIsSavedFromTheBrowserAndDrawn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	resp := b.post("/projects/infra/update", url.Values{
		"name": {"INFRA"}, "color": {"custom"}, "color_custom": {"#1A7F5A"}, "icon": {""},
	})
	wantStatus(t, resp, http.StatusSeeOther)
	wantStatus(t, b.post("/tasks", url.Values{"title": {"tinted"}, "project_ref": {"infra"}}), http.StatusSeeOther)

	field := colourFieldOf(t, b.page("/projects/infra"), "color")
	for _, want := range []string{`value="custom" checked`, `value="#1a7f5a" placeholder`, `type="color" value="#1a7f5a"`} {
		if !strings.Contains(field, want) {
			t.Errorf("the settings form does not show the saved custom colour as %s:\n%s", want, field)
		}
	}
	if strings.Contains(field, `checked><span class="colourswatch" style`) {
		t.Errorf("a palette swatch is checked as well as custom:\n%s", field)
	}

	style := `style="--proj-l: #1a7f5a; --proj-m: `
	row := between(t, b.page("/tasks"), `<li id="t-`, ">")
	if !strings.Contains(row, "is-accented proj-custom") || !strings.Contains(row, style) {
		t.Errorf("the task row is not drawn in the custom colour: %s", row)
	}
	list := between(t, b.page("/projects"), `<li class="projrow`, ">")
	if !strings.Contains(list, "proj-custom") || !strings.Contains(list, style) {
		t.Errorf("the project list entry is not drawn in the custom colour: %s", list)
	}
	card := between(t, b.page("/projects/infra"), `<article class="card`, ">")
	if !strings.Contains(card, "proj-custom") || !strings.Contains(card, style) {
		t.Errorf("the board card is not drawn in the custom colour: %s", card)
	}
}

// The tenant theme is chosen with the same control, and a custom accent
// saved there becomes the page's accent.
func TestTheTenantThemeTakesACustomAccent(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	field := colourFieldOf(t, b.page("/admin/tenant"), "theme")
	for _, want := range []string{`value="ocean"`, `name="theme" value=""`, `name="theme" value="custom"`, `name="theme_custom"`} {
		if !strings.Contains(field, want) {
			t.Errorf("the theme field has no %s:\n%s", want, field)
		}
	}

	resp := b.post("/admin/tenant", url.Values{"name": {"Acme"}, "theme": {"custom"}, "theme_custom": {"#7a3ef0"}})
	wantStatus(t, resp, http.StatusSeeOther)
	page := b.page("/admin/tenant")
	if got := between(t, page, "\n:root { --accent: ", ";"); got != "#7a3ef0" {
		t.Errorf("the page accent is %q, want the custom %q", got, "#7a3ef0")
	}
	if !strings.Contains(colourFieldOf(t, page, "theme"), `value="custom" checked`) {
		t.Error("the tenant form does not show the custom accent as chosen")
	}
}
