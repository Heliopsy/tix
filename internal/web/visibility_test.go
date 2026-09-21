package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// seedVisibility gives the fixture a second project with a task in each, so a
// choice between projects is observable.
func seedVisibility(t *testing.T, b *browser) {
	t.Helper()
	resp := b.post("/projects", url.Values{"key": {"ops"}, "name": {"Ops"}, "color": {"teal"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	b.createTask("infra", "infra work")
	b.createTask("ops", "ops work")
}

// An untouched installation shows every project, exactly as it did before the
// control existed.
func TestTaskListShowsEveryProjectByDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	page := b.page("/tasks")
	for _, want := range []string{"infra work", "ops work"} {
		if !strings.Contains(page, want) {
			t.Errorf("the default task list omits %q:\n%s", want, page)
		}
	}
	for _, unwanted := range []string{"project is hidden", "projects are hidden", "Every project is hidden"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("an untouched task list claims %q", unwanted)
		}
	}
	if !strings.Contains(page, `class="visibilityform"`) {
		t.Error("the task list offers no project visibility control")
	}
	if !strings.Contains(page, `class="swatch proj-teal"`) {
		t.Error("the control does not show each project with its own colour")
	}
}

// Hiding a project takes it off the screen, says so, and stays that way.
func TestHidingAProjectPersistsAndIsExplained(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	resp := b.post("/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/tasks")
	if !strings.Contains(page, "infra work") {
		t.Errorf("the project that was kept is gone:\n%s", page)
	}
	if strings.Contains(page, "ops work") {
		t.Errorf("the hidden project is still shown:\n%s", page)
	}
	if !strings.Contains(page, "1 project is hidden") {
		t.Errorf("the shorter task list is not explained:\n%s", page)
	}
	if !strings.Contains(b.page("/tasks"), "1 project is hidden") {
		t.Error("the choice did not survive the next request")
	}
	if other := f.as("alice"); strings.Contains(other.page("/tasks"), "is hidden") {
		t.Error("the choice leaked into another browser")
	}
}

// What is stored is the hidden set, not an allow list, so a project made after
// the choice is visible without anyone re-ticking it.
func TestANewProjectAppearsDespiteAnEarlierChoice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	resp := b.post("/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	created := b.post("/projects", url.Values{"key": {"web"}, "name": {"Web"}})
	_ = created.Body.Close()
	b.createTask("web", "web work")

	if page := b.page("/tasks"); !strings.Contains(page, "web work") {
		t.Errorf("a project created after the choice was hidden by it:\n%s", page)
	}
}

// Naming a project in the filter is asking for it, so the filter wins over
// the visibility choice and the screen says which rule applied.
func TestAnExplicitFilterOverridesHiddenProjects(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	resp := b.post("/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	page := b.page("/tasks?q=" + url.QueryEscape("project:ops"))
	if !strings.Contains(page, "ops work") {
		t.Errorf("an explicit filter did not reach the hidden project:\n%s", page)
	}
	if !strings.Contains(page, "shown even though") {
		t.Errorf("the override is not explained:\n%s", page)
	}
}

// Hiding everything is a choice someone can make, and must not read as a
// broken tool.
func TestHidingEveryProjectExplainsTheEmptyScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	resp := b.post("/visibility", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()

	page := b.page("/tasks")
	if !strings.Contains(page, "Every project is hidden") {
		t.Errorf("an empty screen is not explained:\n%s", page)
	}
	if strings.Contains(page, "infra work") || strings.Contains(page, "ops work") {
		t.Error("a hidden project still rendered its tasks")
	}
}

// Showing all is one button, and clears the stored value rather than storing
// a full list.
func TestShowAllRestoresEveryProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	hide := b.post("/visibility", url.Values{"project": {"infra"}, "next": {"/tasks"}})
	_ = hide.Body.Close()
	show := b.post("/visibility", url.Values{"reset": {"1"}, "next": {"/tasks"}})
	_ = show.Body.Close()

	page := b.page("/tasks")
	if !strings.Contains(page, "ops work") || !strings.Contains(page, "infra work") {
		t.Errorf("showing all did not restore every project:\n%s", page)
	}
	if b.cookie(web.HiddenProjectsCookie) != "" {
		t.Error("showing all left a value behind instead of clearing the choice")
	}
}

// A value another program left behind, or one that was truncated, shows
// everything rather than emptying the screen.
func TestAMalformedVisibilityChoiceFallsBackToShowingEverything(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedVisibility(t, b)

	for _, raw := range []string{"INFRA;drop", strings.Repeat("a", 2000), "../../etc"} {
		b.setCookie(web.HiddenProjectsCookie, raw)
		page := b.page("/tasks")
		if !strings.Contains(page, "infra work") || !strings.Contains(page, "ops work") {
			t.Errorf("a cookie value of %.20q hid a project:\n%s", raw, page)
		}
	}
}
