// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// toolbar is the filter toolbar and nothing else: from its opening tag to the
// chips row that follows it.
func toolbar(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<div class="tbar" id="tasks-toolbar"`, `<div id="tasks-chips">`)
}

// popoverBlock is one popover of the toolbar, from its opening tag to the
// close button that ends it.
func popoverBlock(t *testing.T, page, id string) string {
	t.Helper()
	return between(t, page, `<div id="`+id+`"`, `popovertarget="`+id+`" popovertargetaction="hide"`)
}

var formOpen = regexp.MustCompile(`<form\b[^>]*>`)

func TestTheToolbarIsOneFormWithItsControls(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one task")

	for _, route := range []string{"/tasks", "/projects/infra?view=list"} {
		bar := toolbar(t, b.page(route))
		gets := 0
		for _, tag := range formOpen.FindAllString(bar, -1) {
			if strings.Contains(tag, `class="filterbar"`) {
				gets++
				if !strings.Contains(tag, `method="get"`) || !strings.Contains(tag, `id="filterbar"`) {
					t.Errorf("%s: the filter form is not a GET form named filterbar: %s", route, tag)
				}
			}
		}
		if gets != 1 {
			t.Errorf("%s: the toolbar holds %d filter forms, want one", route, gets)
		}
		for _, want := range []string{
			`name="q"`, `data-shortcut-target="filter"`, `data-suggest="filter-suggest"`,
			`popovertarget="fb-filters"`, `popovertarget="fb-sort"`, `popovertarget="viewprefs"`,
			`class="viewswitch viewlinks"`, `placeholder="Filter, for example tag:ops due:week @raj"`,
		} {
			if !strings.Contains(bar, want) {
				t.Errorf("%s: the toolbar lacks %s", route, want)
			}
		}
		for _, id := range []string{"fb-filters", "fb-sort", "viewprefs"} {
			if !strings.Contains(bar, `<div id="`+id+`" class="flowmenu`) || !strings.Contains(popoverBlock(t, bar, id), " popover>") {
				t.Errorf("%s: %s is not a popover", route, id)
			}
		}
		if strings.Contains(bar, "<details") && strings.Contains(bar, "<details class=\"panel") {
			t.Errorf("%s: the toolbar hides something in a details fold", route)
		}
		// The fields in the menus outside the form name it, or Apply would
		// submit without them.
		sorts := popoverBlock(t, bar, "fb-sort")
		for _, field := range []string{`name="sort" form="filterbar"`, `name="order" form="filterbar"`, `type="submit" form="filterbar"`} {
			if !strings.Contains(sorts, field) {
				t.Errorf("%s: Sort lacks %s", route, field)
			}
		}
		if !strings.Contains(popoverBlock(t, bar, "viewprefs"), `name="limit" form="filterbar"`) {
			t.Errorf("%s: More has no page size of the filter form", route)
		}
	}
}

func TestTheFiltersPopoverHoldsItsPickersInTheForm(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one task")

	page := b.page("/tasks?q=" + url.QueryEscape("status:doing priority:high priority:low"))
	form := between(t, page, `<form method="get" action="/tasks" class="filterbar"`, "</form>")
	pop := popoverBlock(t, form, "fb-filters")
	for _, want := range []string{
		`<select name="project">`, `name="pick_assignee"`, `name="pick_tag"`, `<select name="pick_status">`,
		`<select name="pick_priority">`, `<select name="due">`, `name="all" value="1"`,
		`name="was_status" value="doing"`, `name="was_priority" value="*"`,
	} {
		if !strings.Contains(pop, want) {
			t.Errorf("the Filters popover lacks %s", want)
		}
	}
	if !strings.Contains(pop, `<option value="doing" selected>`) {
		t.Errorf("the status picker does not show the term in force:\n%s", between(t, pop, `<select name="pick_status">`, "</select>"))
	}
	if !strings.Contains(between(t, pop, `<select name="pick_priority">`, "</select>"), `<option value="*" selected>`) {
		t.Error("the priority picker does not say the box holds several")
	}

	project := popoverBlock(t, b.page("/projects/infra?view=list"), "fb-filters")
	if strings.Contains(project, `<select name="project"`) {
		t.Error("the project page offers a project picker")
	}
}

func TestAPickedTermIsFoldedIntoTheExpression(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, tc := range []struct{ route, query, want string }{
		{"/tasks", "q=tag%3Aops+priority%3Alow&pick_priority=high&was_priority=low&pick_status=&was_status=&due=&page=3",
			"/tasks?q=tag%3Aops+priority%3Ahigh"},
		{"/projects/infra", "q=&view=board&pick_tag=ops&pick_status=&was_status=",
			"/projects/infra?q=tag%3Aops&view=board"},
		{"/tasks", "q=status%3Adone&pick_status=todo&was_status=todo", "/tasks?q=status%3Adone"},
	} {
		resp := b.get(tc.route + "?" + tc.query)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther || resp.Header.Get("Location") != tc.want {
			t.Errorf("%s?%s answered %d to %q, want 303 to %q", tc.route, tc.query, resp.StatusCode, resp.Header.Get("Location"), tc.want)
		}
	}
	if resp := b.get("/tasks?q=tag%3Aops"); resp.StatusCode != http.StatusOK {
		t.Errorf("a plain filter answered %d", resp.StatusCode)
	}
}

func TestActiveTermsAreChipsWithLinksWithoutThem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one task")

	plain := b.page("/tasks")
	if strings.Contains(plain, `class="fbchips"`) || strings.Contains(plain, `class="fbchips-clear"`) {
		t.Error("an unfiltered list draws a chip row")
	}

	page := b.page("/tasks?q=" + url.QueryEscape("tag:ops -tag:chore @raj") + "&due=week&sort=title&view=list")
	chips := between(t, page, `<ul class="fbchips"`, "</ul>")
	links := regexp.MustCompile(`class="fbchip-x" href="([^"]+)"`).FindAllStringSubmatch(chips, -1)
	want := []string{
		"/tasks?q=tag%3Aops+-tag%3Achore+%40raj&sort=title&view=list",
		"/tasks?due=week&q=-tag%3Achore+%40raj&sort=title&view=list",
		"/tasks?due=week&q=tag%3Aops+%40raj&sort=title&view=list",
		"/tasks?due=week&q=tag%3Aops+-tag%3Achore&sort=title&view=list",
	}
	if len(links) != len(want) {
		t.Fatalf("read %d chip links, want %d:\n%s", len(links), len(want), chips)
	}
	for i, w := range want {
		if got := html.UnescapeString(links[i][1]); got != w {
			t.Errorf("chip %d removes to %q, want %q", i, got, w)
		}
	}
	for _, label := range []string{"due: week", "tag: ops", "-tag: chore", "assignee: raj"} {
		if !strings.Contains(chips, ">"+label+"<") {
			t.Errorf("no chip reads %q", label)
		}
	}
	clear := regexp.MustCompile(`class="fbchips-clear" href="([^"]+)"`).FindStringSubmatch(chips)
	if clear == nil || html.UnescapeString(clear[1]) != "/tasks?sort=title&view=list" {
		t.Errorf("Clear all leads to %v, want the list with its sort and view", clear)
	}
	if count := between(t, page, `<span id="tasks-filtercount"`, "</span></span>"); !strings.Contains(count, ">4") {
		t.Errorf("the Filters button does not count the terms: %s", count)
	}
}

func TestTheProjectPageChipsStayOnTheProject(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/projects/infra?view=list&q=tag%3Aops&project=web")
	chips := between(t, page, `<ul class="fbchips"`, "</ul>")
	links := regexp.MustCompile(`class="fbchip-x" href="([^"]+)"`).FindAllStringSubmatch(chips, -1)
	if len(links) != 1 || !strings.HasPrefix(html.UnescapeString(links[0][1]), "/projects/infra?") {
		t.Errorf("project page chips = %v, want one tag chip on the project page", links)
	}
}

func TestTheSortDirectionIsInTheAddress(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "alpha task")
	b.createTask("infra", "beta task")

	listing := func(path string) string {
		return between(t, b.page(path), `<div id="tasks-listing">`, `<div id="live-screen"`)
	}
	asc, desc := listing("/tasks?sort=title"), listing("/tasks?sort=title&order=desc")
	if strings.Index(asc, "alpha task") > strings.Index(asc, "beta task") {
		t.Error("the ascending title sort lists beta first")
	}
	if strings.Index(desc, "beta task") > strings.Index(desc, "alpha task") {
		t.Error("order=desc did not turn the title sort around")
	}
	page := b.page("/tasks?sort=title&order=desc&q=tag%3Aops")
	if !strings.Contains(popoverBlock(t, page, "fb-sort"), `<option value="desc" selected>`) {
		t.Error("the Sort popover does not show the direction in force")
	}
	clear := regexp.MustCompile(`<a class="secondary clear" href="([^"]+)"`).FindStringSubmatch(page)
	if clear == nil || !strings.Contains(html.UnescapeString(clear[1]), "order=desc") {
		t.Errorf("Clear drops the direction: %v", clear)
	}
}

func TestTheMoreMenuListsSavedFilters(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/filters", url.Values{"name": {"Late ops"}, "q": {"tag:ops"}})
	_ = resp.Body.Close()
	more := popoverBlock(t, b.page("/tasks"), "viewprefs")
	if !strings.Contains(more, ">Late ops</a>") {
		t.Errorf("More does not list the saved filter:\n%s", more)
	}
	if strings.Contains(more, "savefilter-open") {
		t.Error("More offers to save with no filter in force")
	}
	if !strings.Contains(popoverBlock(t, b.page("/tasks?q=tag%3Aops"), "viewprefs"), "savefilter-open") {
		t.Error("More does not offer to save the filter in force")
	}
}

func TestTheSuggestionsComeFromTheTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/tasks")
	data := between(t, page, `<script type="application/json" id="filter-suggest">`, "</script>")
	for _, want := range []string{`"keys":["project"`, `"k":"infra"`, `"person":[`, `"alice"`, `"status":["todo"`, `"priority":["highest"`, `"due":["overdue"`} {
		if !strings.Contains(data, want) {
			t.Errorf("the suggestion data lacks %s:\n%s", want, data)
		}
	}
	live := b.page("/tasks/live")
	if strings.Contains(live, `id="filter-suggest"`) {
		t.Error("a live refresh carries the suggestion data, so it lists the directory for nothing")
	}
}

func TestTheSelectToggleIsInTheToolbarAndTheModeSurvivesAFilter(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "one task")
	page := b.page("/tasks?select=1&q=one")
	if bar := toolbar(t, page); !strings.Contains(bar, `data-select-toggle`) {
		t.Error("the Select toggle is not in the toolbar")
	}
	form := between(t, page, `<form method="get" action="/tasks" class="filterbar"`, "</form>")
	if !strings.Contains(form, `<input type="hidden" name="select" value="1">`) {
		t.Error("the filter form drops select mode")
	}
	chips := between(t, page, `<ul class="fbchips"`, "</ul>")
	for _, m := range regexp.MustCompile(`class="fbchip(?:-x|s-clear)" href="([^"]+)"`).FindAllStringSubmatch(chips, -1) {
		if !strings.Contains(html.UnescapeString(m[1]), "select=1") {
			t.Errorf("a chip link drops select mode: %s", m[1])
		}
	}
}
