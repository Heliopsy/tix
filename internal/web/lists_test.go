package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// seedLists gives the fixture a second project with a task in each, so a
// choice between lists is observable.
func seedLists(t *testing.T, b *browser) {
	t.Helper()
	resp := b.post("/projects", url.Values{"key": {"ops"}, "name": {"Ops"}, "color": {"teal"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	b.createTask("infra", "infra work")
	b.createTask("ops", "ops work")
}

// An untouched installation shows every list, exactly as it did before the
// control existed.
func TestTaskListShowsEveryListByDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	page := b.page("/tasks")
	for _, want := range []string{"infra work", "ops work"} {
		if !strings.Contains(page, want) {
			t.Errorf("the default list omits %q:\n%s", want, page)
		}
	}
	for _, unwanted := range []string{"list is hidden", "lists are hidden", "Every list is hidden"} {
		if strings.Contains(page, unwanted) {
			t.Errorf("an untouched list claims %q", unwanted)
		}
	}
	if !strings.Contains(page, `class="listform"`) {
		t.Error("the task list offers no list visibility control")
	}
	if !strings.Contains(page, `class="swatch proj-teal"`) {
		t.Error("the control does not show each list with its own colour")
	}
}

// Hiding a list takes it off the screen, says so, and stays that way.
func TestHidingAListPersistsAndIsExplained(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	resp := b.post("/lists", url.Values{"list": {"infra"}, "next": {"/tasks"}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/tasks")
	if !strings.Contains(page, "infra work") {
		t.Errorf("the list that was kept is gone:\n%s", page)
	}
	if strings.Contains(page, "ops work") {
		t.Errorf("the hidden list is still shown:\n%s", page)
	}
	if !strings.Contains(page, "1 list is hidden") {
		t.Errorf("the shorter list is not explained:\n%s", page)
	}
	if !strings.Contains(b.page("/tasks"), "1 list is hidden") {
		t.Error("the choice did not survive the next request")
	}
	if other := f.as("alice"); strings.Contains(other.page("/tasks"), "is hidden") {
		t.Error("the choice leaked into another browser")
	}
}

// What is stored is the hidden set, not an allow list, so a project made after
// the choice is visible without anyone re-ticking it.
func TestANewListAppearsDespiteAnEarlierChoice(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	resp := b.post("/lists", url.Values{"list": {"infra"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	created := b.post("/projects", url.Values{"key": {"web"}, "name": {"Web"}})
	_ = created.Body.Close()
	b.createTask("web", "web work")

	if page := b.page("/tasks"); !strings.Contains(page, "web work") {
		t.Errorf("a list created after the choice was hidden by it:\n%s", page)
	}
}

// Naming a list in the filter is asking for it, so the filter wins over the
// visibility choice and the screen says which rule applied.
func TestAnExplicitFilterOverridesTheHiddenLists(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	resp := b.post("/lists", url.Values{"list": {"infra"}, "next": {"/tasks"}})
	_ = resp.Body.Close()

	page := b.page("/tasks?q=" + url.QueryEscape("project:ops"))
	if !strings.Contains(page, "ops work") {
		t.Errorf("an explicit filter did not reach the hidden list:\n%s", page)
	}
	if !strings.Contains(page, "shown even though") {
		t.Errorf("the override is not explained:\n%s", page)
	}
}

// Hiding everything is a choice someone can make, and must not read as a
// broken tool.
func TestHidingEveryListExplainsTheEmptyScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	resp := b.post("/lists", url.Values{"next": {"/tasks"}})
	_ = resp.Body.Close()

	page := b.page("/tasks")
	if !strings.Contains(page, "Every list is hidden") {
		t.Errorf("an empty screen is not explained:\n%s", page)
	}
	if strings.Contains(page, "infra work") || strings.Contains(page, "ops work") {
		t.Error("a hidden list still rendered its tasks")
	}
}

// Showing all is one button, and clears the stored value rather than storing
// a full list.
func TestShowAllRestoresEveryList(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	hide := b.post("/lists", url.Values{"list": {"infra"}, "next": {"/tasks"}})
	_ = hide.Body.Close()
	show := b.post("/lists", url.Values{"reset": {"1"}, "next": {"/tasks"}})
	_ = show.Body.Close()

	page := b.page("/tasks")
	if !strings.Contains(page, "ops work") || !strings.Contains(page, "infra work") {
		t.Errorf("showing all did not restore every list:\n%s", page)
	}
	if b.cookie(web.ListsCookie) != "" {
		t.Error("showing all left a value behind instead of clearing the choice")
	}
}

// A value another program left behind, or one that was truncated, shows
// everything rather than emptying the screen.
func TestAMalformedListChoiceFallsBackToShowingEverything(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	seedLists(t, b)

	for _, raw := range []string{"INFRA;drop", strings.Repeat("a", 2000), "../../etc"} {
		b.setCookie(web.ListsCookie, raw)
		page := b.page("/tasks")
		if !strings.Contains(page, "infra work") || !strings.Contains(page, "ops work") {
			t.Errorf("a cookie value of %.20q hid a list:\n%s", raw, page)
		}
	}
}
