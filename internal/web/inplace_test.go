// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// openTag returns the start tag of the first element whose start tag contains
// marker, so an assertion reads one element and not the page around it.
func openTag(t *testing.T, page, marker string) string {
	t.Helper()
	at := strings.Index(page, marker)
	if at < 0 {
		return ""
	}
	start := strings.LastIndex(page[:at], "<")
	end := strings.Index(page[at:], ">")
	if start < 0 || end < 0 {
		return ""
	}
	return page[start : at+end+1]
}

// A list row's star, status and priority forms swap that row, and the task
// screen's forms swap the content and not the sidebar. Both are declared on
// the element the forms sit in, which is what assets/feel.js reads; in the
// task dialog the dialog's own body is the swap target already, so nothing
// there may claim main.wrap, which in a dialog is the listing behind it.
func TestRowsAndTheTaskScreenSayWhatTheirFormsSwap(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "acted on in place")

	row := openTag(t, b.page("/tasks"), `id="t-`+ref+`"`)
	if !strings.Contains(row, "data-inplace") {
		t.Errorf("the row does not mark itself as what its forms swap:\n%s", row)
	}

	screen := b.page("/tasks/" + ref)
	for _, marker := range []string{`class="titlebar taskhead"`, `id="taskmenu-` + ref + `"`, `id="flow-` + ref + `"`, `id="prio-` + ref + `"`, `id="snooze-` + ref + `"`, `class="detail taskgrid"`} {
		tag := openTag(t, screen, marker)
		if !strings.Contains(tag, `data-inplace="main.wrap"`) {
			t.Errorf("the task screen's %s does not swap the content in place:\n%s", marker, tag)
		}
	}

	// A lane card's own forms carry hx-post for the card already; the star,
	// which is the shared partial, is the one the card's data-inplace arms.
	switched := b.post("/taskview", url.Values{"view": {"lanes"}, "next": {"/tasks"}})
	_ = switched.Body.Close()
	card := openTag(t, b.page("/tasks"), `data-lane-task="`+ref+`"`)
	if !strings.Contains(card, "data-inplace") {
		t.Errorf("the lane card does not mark itself as what its star swaps:\n%s", card)
	}

	resp := b.getDialog("/tasks/" + ref)
	dialog := readAll(t, resp)
	if strings.Contains(dialog, "data-inplace") {
		t.Error("the task dialog claims main.wrap, which in a dialog is the listing behind it")
	}
}

var undoForm = regexp.MustCompile(`(?s)<form id="flash-undo" method="post" action="([^"]+)" hidden>(.*?)</form>`)

// followed reads the screen a redirect leads to.
func followed(t *testing.T, b *browser, resp *http.Response) string {
	t.Helper()
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	return b.page(resp.Header.Get("Location"))
}

// A move offers to take itself back when the workflow allows the way back,
// and the Undo is an ordinary form that does it.
func TestAMoveOffersAnUndoThatMovesItBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "moved and moved back")

	page := followed(t, b, b.post("/tasks/"+ref+"/transition", url.Values{"route": {"doing"}, "next": {"/tasks"}}))
	toast := between(t, page, `<p class="flash">`, "</p>")
	if !strings.Contains(toast, `form="flash-undo"`) {
		t.Fatalf("the toast for a move offers no Undo:\n%s", toast)
	}
	m := undoForm.FindStringSubmatch(page)
	if m == nil {
		t.Fatal("the Undo button names a form that is not on the page")
	}
	if m[1] != "/tasks/"+ref+"/transition" || !strings.Contains(m[2], `name="route" value="todo"`) {
		t.Fatalf("Undo posts to %s with %s, want the route back to todo", m[1], m[2])
	}
	if !strings.Contains(m[2], `name="next" value="/tasks"`) {
		t.Errorf("Undo does not come back to the listing, or carries the toast into it: %s", m[2])
	}

	back := followed(t, b, b.post(m[1], url.Values{"route": {"todo"}, "next": {"/tasks"}}))
	row := between(t, back, `<li id="t-`+ref+`"`, "</li>")
	if pill := between(t, row, `<span class="pill status `, "</span>"); !strings.HasPrefix(pill, `todo">todo`) {
		t.Errorf("after Undo the row reads %q, want todo", pill)
	}
}

// A priority change offers the old priority back.
func TestAPriorityChangeOffersTheOldOneBack(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "raised")

	resp := b.post("/tasks/"+ref+"/priority", url.Values{"priority": {"1"}, "next": {"/tasks"}})
	wantStatus(t, resp, http.StatusSeeOther)
	_ = resp.Body.Close()
	loc, _ := url.Parse(resp.Header.Get("Location"))
	if got := loc.Query().Get("flashundo"); got != "priority:3" {
		t.Fatalf("flashundo = %q, want priority:3 (normal, what it was)", got)
	}
	m := undoForm.FindStringSubmatch(b.page(resp.Header.Get("Location")))
	if m == nil || m[1] != "/tasks/"+ref+"/priority" || !strings.Contains(m[2], `name="priority" value="3"`) {
		t.Fatalf("the Undo form does not set the priority back: %v", m)
	}

	same := b.post("/tasks/"+ref+"/priority", url.Values{"priority": {"1"}, "next": {"/tasks"}})
	_ = same.Body.Close()
	loc, _ = url.Parse(same.Header.Get("Location"))
	if loc.Query().Has("flashundo") {
		t.Error("setting the priority it already had offers an Undo of nothing")
	}
}

// A soft delete offers to restore; a permanent one has nothing to restore.
func TestADeleteOffersRestoreUnlessItWasPermanent(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	soft := b.createTask("infra", "deleted softly")
	hard := b.createTask("infra", "deleted for good")

	page := followed(t, b, b.post("/tasks/"+soft+"/delete", url.Values{}))
	m := undoForm.FindStringSubmatch(page)
	if m == nil || m[1] != "/tasks/"+soft+"/restore" {
		t.Fatalf("a soft delete offers no restore: %v", m)
	}
	restored := b.post(m[1], url.Values{})
	wantStatus(t, restored, http.StatusSeeOther)
	_ = restored.Body.Close()
	if !strings.Contains(b.page("/tasks"), `id="t-`+soft+`"`) {
		t.Error("Undo did not bring the deleted task back to the listing")
	}

	page = followed(t, b, b.post("/tasks/"+hard+"/delete", url.Values{"hard": {"1"}}))
	if strings.Contains(page, `id="flash-undo"`) {
		t.Error("a permanent delete offers an Undo it cannot honour")
	}
}

// The undo is part of the toast, not of the screen: the screen's own address,
// which every form on it returns to and live.js refreshes from, never
// carries it, or the toast would come back on every later action.
func TestTheUndoDoesNotStickToTheScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "moved once")

	resp := b.post("/tasks/"+ref+"/transition", url.Values{"route": {"doing"}, "next": {"/tasks"}})
	page := followed(t, b, resp)
	page = strings.Replace(page, undoForm.FindString(page), "", 1)
	if strings.Contains(page, "flashundo=") {
		at := strings.Index(page, "flashundo=")
		t.Errorf("the screen carries the undo outside the toast:\n%s", page[max(0, at-300):at+100])
	}
}

// The toast region is on every signed-in screen, empty or not, so an action
// swapped in place can bring its toast with it.
func TestEveryScreenHasARegionForTheToast(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	for _, path := range []string{"/tasks", "/projects", "/settings"} {
		if !strings.Contains(b.page(path), `<div id="toast">`) {
			t.Errorf("%s has no #toast region", path)
		}
	}
}

// The view switch is boosted: switching swaps the body rather than loading
// the page, stylesheet and scripts again.
func TestTheViewSwitchIsBoosted(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	tag := openTag(t, b.page("/tasks"), `action="/taskview"`)
	if tag == "" {
		t.Fatal("the listing has no view switch")
	}
	if strings.Contains(tag, `hx-boost="false"`) {
		t.Errorf("the view switch reloads the whole page:\n%s", tag)
	}
	if !strings.Contains(tag, "show:none") {
		t.Errorf("the view switch scrolls the page to the top on every switch:\n%s", tag)
	}
}

// The keyboard shortcuts can be found without knowing to press "?": the
// sidebar carries a control that opens them, shown only with the script
// that opens them.
func TestTheSidebarOffersTheKeyboardShortcuts(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	side := sidebarOf(t, b.page("/tasks"))
	tag := openTag(t, side, "data-shortcuts-open")
	if tag == "" {
		t.Fatal("the sidebar has no control that opens the keyboard shortcuts")
	}
	if !strings.Contains(tag, `type="button"`) || !strings.Contains(tag, "data-needs-js") {
		t.Errorf("the shortcuts control is not a button shown only with its script:\n%s", tag)
	}
}
