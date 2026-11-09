// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"html"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

// selectToggle is the Select toggle's tag, and fails when the screen draws
// none.
func selectToggle(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<a class="btn secondary selecttoggle"`, ">")
}

// attr reads one attribute's value out of a tag, unescaped.
func attr(t *testing.T, tag, name string) string {
	t.Helper()
	tag = " " + strings.TrimSpace(tag)
	at := strings.Index(tag, " "+name+`="`)
	if at < 0 {
		t.Fatalf("%s carries no %s:\n%s", tag, name, tag)
	}
	value, _, ok := strings.Cut(tag[at+len(name)+3:], `"`)
	if !ok {
		t.Fatalf("%s has an unterminated %s:\n%s", tag, name, tag)
	}
	return html.UnescapeString(value)
}

// selectSheet is the stylesheet, for the rules that show the boxes.
func selectSheet(t *testing.T, f *fixture) string {
	t.Helper()
	return body(t, f.as("alice").get("/assets/app.css"))
}

const selectOn = `.wrap:has([data-select-toggle][aria-pressed="true"])`

func TestRowsShowOnlyTheDoneControlOutsideSelectMode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")

	page := b.page("/tasks?q=title~one")
	toggle := selectToggle(t, page)
	if got := attr(t, toggle, "aria-pressed"); got != "false" {
		t.Errorf("the toggle is pressed on a plain listing: %s", toggle)
	}
	href, err := url.Parse(attr(t, toggle, "href"))
	if err != nil {
		t.Fatal(err)
	}
	if href.Path != "/tasks" || href.Query().Get(web.SelectParam) != "1" || href.Query().Get("q") != "title~one" {
		t.Errorf("the toggle leads to %s, want this listing with select=1", href)
	}
	row := between(t, page, `<li id="t-`+ref+`"`, "</li>")
	if !strings.Contains(row, `class="check" data-focus="tick"`) {
		t.Errorf("the row has no round done control:\n%s", row)
	}

	sheet := selectSheet(t, f)
	if d := declarations(t, sheet, "input.rowpick {"); !strings.Contains(d, "display: none") {
		t.Errorf("the selection box is shown outside select mode: %s", d)
	}
	if d := declarations(t, sheet, selectOn+" input.rowpick {"); !strings.Contains(d, "display: inline-block") {
		t.Errorf("select mode does not show the box: %s", d)
	}
	if d := declarations(t, sheet, `.wrap:not(:has([data-select-toggle][aria-pressed="true"])) .bulkbar {`); !strings.Contains(d, "display: none") {
		t.Errorf("the bar is shown outside select mode: %s", d)
	}
	if d := declarations(t, sheet, selectOn+" button.check {"); !strings.Contains(d, "opacity") {
		t.Errorf("select mode leaves the done control at full weight: %s", d)
	}
}

func TestSelectParamTurnsSelectModeOnAndIsKept(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")

	page := b.page("/tasks?q=title~one&select=1")
	toggle := selectToggle(t, page)
	if got := attr(t, toggle, "aria-pressed"); got != "true" {
		t.Errorf("select=1 leaves the toggle unpressed: %s", toggle)
	}
	off := attr(t, toggle, "href")
	if strings.Contains(off, "select=") || !strings.Contains(off, "q=title") {
		t.Errorf("the pressed toggle leads to %s, want this listing without select", off)
	}
	row := between(t, page, `<li id="t-`+ref+`"`, "</li>")
	box := `<input type="checkbox" class="rowpick" name="ref" value="` + ref + `" form="bulkbar" aria-label="Select ` + strings.ToUpper(ref) + `" data-pick>`
	if !strings.Contains(row, box) {
		t.Errorf("the row has no labelled box for the bar:\n%s", row)
	}
	bar := bulkBar(t, page)
	cancel := between(t, bar, `<a class="btn secondary"`, ">")
	if attr(t, cancel, "href") != off {
		t.Errorf("Cancel leads to %s, want the toggle's %s", attr(t, cancel, "href"), off)
	}
	if !strings.Contains(bar, `role="status" data-bulk-count`) {
		t.Error("the bar's count is not announced")
	}
	next := between(t, bar, `<input type="hidden" name="next"`, ">")
	if !strings.Contains(attr(t, next, "value"), "select=1") {
		t.Errorf("the bar returns to %s, which leaves select mode", next)
	}

	filter := between(t, page, `class="filterbar"`, "</form>")
	if !strings.Contains(filter, `<input type="hidden" name="select" value="1">`) {
		t.Error("the filter form drops select mode")
	}
	clear := between(t, page, `<a class="secondary clear"`, ">")
	if !strings.Contains(attr(t, clear, "href"), "select=1") {
		t.Errorf("Clear drops select mode: %s", clear)
	}
	views := between(t, page, `id="tasks-views"`, "</nav>")
	if strings.Count(views, "select=1") != len(web.TaskViews) {
		t.Errorf("a view switch link drops select mode:\n%s", views)
	}
}

func TestBoardAndLaneCardsSelectInSelectMode(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")
	box := `<input type="checkbox" class="rowpick" name="ref" value="` + ref + `" form="bulkbar" aria-label="Select ` + strings.ToUpper(ref) + `" data-pick>`

	for _, view := range []string{web.TaskViewBoard, web.TaskViewLanes} {
		page := b.page("/tasks?view=" + view + "&select=1")
		card := between(t, page, `<article id="t-`+ref+`"`, "</article>")
		if !strings.Contains(card, box) {
			t.Errorf("the %s card has no box for the bar:\n%s", view, card)
		}
		if !strings.Contains(bulkBar(t, page), `formaction="/tasks/bulk?action=star"`) {
			t.Errorf("the %s has no bulk bar", view)
		}
		if got := attr(t, selectToggle(t, page), "aria-pressed"); got != "true" {
			t.Errorf("the %s toggle is not pressed for select=1", view)
		}
	}
}

func TestSelectModeBulkActionWorksWithoutScripting(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	one, two := b.createTask("infra", "one"), b.createTask("infra", "two")

	page := b.page("/tasks?select=1")
	bar := bulkBar(t, page)
	next := attr(t, between(t, bar, `<input type="hidden" name="next"`, ">"), "value")
	resp := b.post("/tasks/bulk?action=priority", url.Values{"ref": {one, two}, "priority": {"1"}, "next": {next}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusSeeOther)
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	if loc.Path != "/tasks" || loc.Query().Get(web.SelectParam) != "1" {
		t.Errorf("returned to %s, want the listing still in select mode", loc)
	}
	for _, ref := range []string{one, two} {
		if got := readTask(t, f, ref).Priority; got != core.PriorityHighest {
			t.Errorf("%s priority = %d, want highest", ref, got)
		}
	}
	back := b.page(loc.RequestURI())
	if got := attr(t, selectToggle(t, back), "aria-pressed"); got != "true" {
		t.Error("the screen the action returns to is out of select mode")
	}
}

func TestTheLiveFragmentDrawsTheBoxesItsRowsNeed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "one")
	live := b.page("/tasks/live")
	if !strings.Contains(between(t, live, `<li id="t-`+ref+`"`, "</li>"), `data-pick>`) {
		t.Error("a row drawn by a live refresh has no box, so it would leave select mode's selection behind")
	}
}

func TestSelectionKeysAreBoundInEveryScheme(t *testing.T) {
	t.Parallel()
	for _, scheme := range web.KeySchemes() {
		table := web.KeyBindingsFor(scheme)
		for action, want := range map[string]string{"extendUp": "shift+ArrowUp", "extendDown": "shift+ArrowDown", "complete": "d", "select": "x"} {
			if !containsKey(table[action], want) {
				t.Errorf("scheme %q %s = %v, want %s", scheme, action, table[action], want)
			}
		}
	}
}
