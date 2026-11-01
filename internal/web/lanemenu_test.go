// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/service"
)

// moreMenu is a card's more actions menu and nothing else of the card.
func moreMenu(t *testing.T, card, ref string) string {
	t.Helper()
	menu := popover(t, card, "lmore-"+ref)
	if menu == "" {
		t.Fatalf("the card for %s has no more actions menu:\n%s", ref, card)
	}
	return menu
}

// menuForm is the one form in a menu posting to action, so an assertion
// about it cannot be satisfied by a neighbour.
func menuForm(t *testing.T, menu, action string) string {
	t.Helper()
	at := strings.Index(menu, `<form method="post" action="`+action+`"`)
	if at < 0 {
		return ""
	}
	end := strings.Index(menu[at:], "</form>")
	if end < 0 {
		t.Fatalf("the form posting to %s is not closed", action)
	}
	return menu[at : at+end]
}

// With every field the View menu can hide hidden, a card still has its dots,
// and the menu behind them reaches every part of the task.
func TestALaneCardsMoreActionsSurviveEveryHiddenField(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	blockedNeedsWord := service.BuiltinWorkflow()
	for i, tr := range blockedNeedsWord.Transitions {
		if tr.To == "blocked" {
			blockedNeedsWord.Transitions[i].RequiresComment = true
		}
	}
	seedWorkflowProject(t, f, "careful", "careful", "Careful", blockedNeedsWord)
	b := f.as("alice")
	ref := b.createTask("careful", "full control")
	tagged := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"ops"}})
	_ = tagged.Body.Close()
	b.chooseLanes("/tasks")
	b.hideTaskFields("star", "status", "priority", "tags", "assignee", "project", "due", "repeat", "updated", "ref")

	card := laneCard(t, b.page("/tasks"), ref)
	for _, gone := range []string{`popovertarget="lflow-`, `popovertarget="lprio-`, `class="starform"`} {
		if strings.Contains(card, gone) {
			t.Fatalf("the fields were not hidden, %s is still on the card:\n%s", gone, card)
		}
	}
	if !strings.Contains(card, `class="lanemore-btn" popovertarget="lmore-`+ref+`" data-focus="more"`) {
		t.Fatalf("the card has no more actions button once its fields are hidden:\n%s", card)
	}
	menu := moreMenu(t, card, ref)

	for _, want := range []string{
		`<a class="flowgo" href="/tasks/` + ref + `?next=%2Ftasks">`,
		`data-copy-link="/tasks/` + ref + `"`,
		`data-copy-text="` + strings.ToUpper(ref) + `"`,
		`data-rename="` + ref + `" data-version="`,
	} {
		if !strings.Contains(menu, want) {
			t.Errorf("the menu has no %s:\n%s", want, menu)
		}
	}
	forms := map[string][]string{
		"/tasks/" + ref + "/star":                 {`name="next" value="/tasks"`},
		"/tasks/" + ref + "/priority":             {`name="priority" value="1"`, `name="priority" value="5"`},
		"/tasks/" + ref:                           {`name="version" value="`, `name="title" value="full control"`, `name="assignee"`, `list="lane-handles"`},
		"/tasks/" + ref + "/tags":                 {`name="tag" required`},
		"/tasks/" + ref + "/tags/remove":          {`name="tag" value="ops"`},
		"/tasks/" + ref + "/delete?next=%2Ftasks": {`Delete task`},
		"/tasks/" + ref + "/transition":           {`name="route" value="doing&gt;done"`},
	}
	for action, wants := range forms {
		form := menuForm(t, menu, action)
		if form == "" {
			t.Errorf("the menu has no form posting to %s:\n%s", action, menu)
			continue
		}
		hx := strings.SplitN(form, ">", 2)[0]
		for _, want := range append([]string{`name="csrf_token"`}, wants...) {
			if !strings.Contains(form, want) {
				t.Errorf("the form posting to %s carries no %s:\n%s", action, want, form)
			}
		}
		for _, want := range []string{`hx-post="` + action + `"`, `hx-target="#t-` + ref + `"`, `hx-select="#t-` + ref + `"`,
			`hx-indicator="#t-` + ref + `"`, `data-lane-control="more"`} {
			if !strings.Contains(hx, want) {
				t.Errorf("the form posting to %s does not swap its card in place, no %s:\n%s", action, want, hx)
			}
		}
	}
	if !strings.Contains(strings.SplitN(menuForm(t, menu, "/tasks/"+ref+"/delete?next=%2Ftasks"), ">", 2)[0], `hx-select-oob="#toast"`) {
		t.Error("a delete from the menu does not bring the toast and its Undo")
	}
	if got := routeButtons(menu); !slices.Contains(got, "doing") || slices.Contains(got, "blocked") {
		t.Errorf("the menu's moves are %v, want doing and never the move needing a comment", got)
	}
	if !strings.Contains(menu, `<a class="flowgo" href="/tasks/`+ref+`?next=%2Ftasks" data-commented>`) ||
		!strings.Contains(menu, `<span class="flowto">Blocked</span>`) {
		t.Errorf("the move needing a comment is not a link to the task:\n%s", menu)
	}
	if !strings.Contains(b.page("/tasks"), `<datalist id="lane-handles">`) {
		t.Error("the assignee fields have no handles to suggest")
	}
}

// A reader who may only read is offered what reading allows: opening,
// copying and their own star.
func TestALaneMenuOffersAReaderOnlyWhatTheyMayDo(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	ref := f.as("alice").createTask("infra", "read only")
	viewer := f.as("viewer")
	viewer.chooseLanes("/tasks")
	page := viewer.page("/tasks")
	menu := moreMenu(t, laneCard(t, page, ref), ref)
	for _, want := range []string{`href="/tasks/` + ref + `?next=`, `data-copy-link=`, `data-copy-text=`, `action="/tasks/` + ref + `/star"`} {
		if !strings.Contains(menu, want) {
			t.Errorf("a reader is not offered %s:\n%s", want, menu)
		}
	}
	for _, refused := range []string{"/transition\"", "/priority\"", `action="/tasks/` + ref + `"`, "/tags\"", "/tags/remove\"", "/delete", "data-rename", "data-copy-to"} {
		if strings.Contains(menu, refused) {
			t.Errorf("a reader who may only read is offered %s:\n%s", refused, menu)
		}
	}
	if strings.Contains(page, `id="lane-handles"`) {
		t.Error("a reader who may not assign is sent the tenant's handles")
	}
}

// Each of the menu's forms the existing routes did not already return from
// lands back on the listing at the card, and does what it says.
func TestALaneMenuActsAndReturnsToTheCard(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "acted on")
	b.chooseLanes("/tasks")
	page := b.page("/tasks")
	version := between(t, menuForm(t, moreMenu(t, laneCard(t, page, ref), ref), "/tasks/"+ref), `name="version" value="`, `"`)

	back := func(what string, resp *http.Response) {
		t.Helper()
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		loc := resp.Header.Get("Location")
		if !strings.HasPrefix(loc, "/tasks?") || !strings.HasSuffix(loc, "#t-"+ref) {
			t.Errorf("%s returns to %q, not the listing at the card", what, loc)
		}
	}
	back("assigning", b.post("/tasks/"+ref, url.Values{"version": {version}, "title": {"acted on"}, "assignee": {"alice"}, "next": {"/tasks"}}))
	back("tagging", b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"menu"}, "next": {"/tasks"}}))
	card := laneCard(t, b.page("/tasks"), ref)
	if !strings.Contains(card, `href="/tasks?q=assignee:alice"`) && !strings.Contains(moreMenu(t, card, ref), `value="alice"`) {
		t.Errorf("the card is not assigned to alice:\n%s", card)
	}
	if !strings.Contains(menuForm(t, moreMenu(t, card, ref), "/tasks/"+ref+"/tags/remove"), `value="menu"`) {
		t.Errorf("the tag added from the menu is not on the card:\n%s", card)
	}
	back("untagging", b.post("/tasks/"+ref+"/tags/remove", url.Values{"tag": {"menu"}, "next": {"/tasks"}}))
	if strings.Contains(laneCard(t, b.page("/tasks"), ref), `value="menu"`) {
		t.Error("the tag removed from the menu is still on the card")
	}

	deleted := b.post("/tasks/"+ref+"/delete?next=%2Ftasks", url.Values{})
	_ = deleted.Body.Close()
	wantStatus(t, deleted, http.StatusSeeOther)
	loc := deleted.Header.Get("Location")
	if !strings.HasPrefix(loc, "/tasks?") || !strings.Contains(loc, "flashundo=restore") {
		t.Fatalf("a delete from the lanes returns to %q, not the listing with an Undo", loc)
	}
	listing := b.page(loc)
	if strings.Contains(listing, `data-lane-task="`+ref+`"`) {
		t.Error("the deleted card is still drawn")
	}
	undo := between(t, listing, `<form id="flash-undo"`, `</form>`)
	if !strings.Contains(undo, `action="/tasks/`+ref+`/restore"`) || !strings.Contains(undo, `name="next" value="/tasks`) {
		t.Fatalf("the toast offers no Undo that returns here:\n%s", undo)
	}
	back("undoing the delete", b.post("/tasks/"+ref+"/restore", url.Values{"next": {"/tasks"}}))
	if !strings.Contains(b.page("/tasks"), `data-lane-task="`+ref+`"`) {
		t.Error("the Undo did not bring the card back")
	}
}

// Without a next the edit, tag and restore routes stay on the task screen,
// which is where their own forms are.
func TestTaskScreenFormsStillReturnToTheTask(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "stays")
	version := between(t, b.page("/tasks/"+ref), `name="version" value="`, `"`)
	for what, resp := range map[string]*http.Response{
		"saving":  b.post("/tasks/"+ref, url.Values{"version": {version}, "title": {"stays"}}),
		"tagging": b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"x"}}),
	} {
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/tasks/"+ref) {
			t.Errorf("%s from the task screen goes to %q", what, loc)
		}
	}
}
