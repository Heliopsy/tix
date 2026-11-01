// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// getDialog fetches a screen the way assets/taskopen.js does: with the header
// asking for the dialog's drawing of it.
func (b *browser) getDialog(path string) *http.Response {
	b.t.Helper()
	req, err := http.NewRequest(http.MethodGet, b.fix.server.URL+path, nil)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	b.stamp(req)
	req.Header.Set(web.DialogHeader, "1")
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("getting %s: %v", path, err)
	}
	return resp
}

// dialogHost is the dialog element the layout draws for Dialog mode.
const dialogHost = `<dialog id="taskdlg"`

// Both preferences are off until chosen, each toggle writes its own value,
// and a value the toggle never writes reads as the default.
func TestTaskOpenAndTitleClickArePreferencesOffByDefault(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "named in a row")

	page := b.page("/tasks")
	if strings.Contains(page, dialogHost) {
		t.Fatal("an untouched browser opens tasks in a dialog")
	}
	if strings.Contains(page, "data-edit-title") || strings.Contains(page, `class="title-open"`) {
		t.Fatal("an untouched browser edits titles in place")
	}

	for _, c := range []struct {
		route, cookie, on, off string
	}{
		{web.RouteTaskOpen, web.TaskOpenCookie, "dialog", "page"},
		{web.RouteTitleClick, web.TitleClickCookie, "edit", "open"},
	} {
		resp := b.post(c.route, url.Values{"next": {"/tasks"}})
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		if got := b.cookie(c.cookie); got != c.on {
			t.Errorf("turning %s on stored %q, want %q", c.route, got, c.on)
		}
		if got := resp.Header.Get("Location"); got != "/tasks" {
			t.Errorf("%s returned to %q", c.route, got)
		}
	}
	page = b.page("/tasks")
	if !strings.Contains(page, dialogHost) {
		t.Error("Dialog mode does not put the dialog on the task list")
	}
	if !strings.Contains(page, "data-edit-title=") {
		t.Error("Edits the title does not mark the titles")
	}

	for _, c := range []struct{ route, cookie, off string }{
		{web.RouteTaskOpen, web.TaskOpenCookie, "page"},
		{web.RouteTitleClick, web.TitleClickCookie, "open"},
	} {
		resp := b.post(c.route, url.Values{"next": {"/tasks"}})
		_ = resp.Body.Close()
		if got := b.cookie(c.cookie); got != c.off {
			t.Errorf("turning %s back off stored %q, want %q", c.route, got, c.off)
		}
	}

	b.setCookie(web.TaskOpenCookie, "1")
	b.setCookie(web.TitleClickCookie, "yes")
	page = b.page("/tasks")
	if strings.Contains(page, dialogHost) || strings.Contains(page, "data-edit-title") {
		t.Error("a value the toggles never write turned a preference on")
	}
}

// The dialog is on every screen a task can be followed from, and never on
// the task screen itself, which is what it would show.
func TestTheDialogIsOnEveryScreenButTheTaskScreen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "opened in a dialog")
	b.setCookie(web.TaskOpenCookie, "dialog")

	for _, path := range []string{"/tasks", "/activity", "/projects/infra", "/stats"} {
		if !strings.Contains(b.page(path), dialogHost) {
			t.Errorf("%s carries no dialog to open a task in", path)
		}
	}
	if strings.Contains(b.page("/tasks/"+ref), dialogHost) {
		t.Error("the task screen carries a dialog for itself")
	}
}

// Settings has a switch for each, in Listings, saying which way it is set.
func TestSettingsDrawsTheTaskOpenAndTitleClickSwitches(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	listings := between(t, contentOf(t, b.page("/settings")), `id="listings"`, "</section>")

	for _, c := range []struct{ route, state, next string }{
		{web.RouteTaskOpen, "Page", "Task&#43;opens&#43;in&#43;saved#listings"},
		{web.RouteTitleClick, "Opens the task", "Clicking&#43;a&#43;title&#43;saved#listings"},
	} {
		form := prefForm(t, listings, c.route)
		if !strings.Contains(form, `role="switch" aria-checked="false"`) {
			t.Errorf("%s is not a switch reading off:\n%s", c.route, form)
		}
		if !strings.Contains(form, `<span class="switch-state">`+c.state+`</span>`) {
			t.Errorf("%s does not name the position in force, %q:\n%s", c.route, c.state, form)
		}
		if !strings.Contains(form, c.next) {
			t.Errorf("%s does not return to Listings with a toast:\n%s", c.route, form)
		}
		if !strings.Contains(form, `class="field-info"`) && !strings.Contains(form, "field-info") {
			t.Errorf("%s has no (i) explanation:\n%s", c.route, form)
		}
	}

	b.setCookie(web.TaskOpenCookie, "dialog")
	b.setCookie(web.TitleClickCookie, "edit")
	listings = between(t, contentOf(t, b.page("/settings")), `id="listings"`, "</section>")
	for _, c := range []struct{ route, state string }{
		{web.RouteTaskOpen, "Dialog"},
		{web.RouteTitleClick, "Edits the title"},
	} {
		form := prefForm(t, listings, c.route)
		if !strings.Contains(form, `role="switch" aria-checked="true"`) ||
			!strings.Contains(form, `<span class="switch-state">`+c.state+`</span>`) {
			t.Errorf("%s does not read %q once chosen:\n%s", c.route, c.state, form)
		}
	}
}

// The header asks for the task screen alone: its content and flash in the
// marked wrapper, no layout, no live binding of its own, a Close where the
// back link was, and ids kept apart from a listing's.
func TestTheDialogAsksForTheTaskScreenAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "drawn for the dialog")

	resp := b.getDialog("/tasks/" + ref + "?flash=task+saved")
	wantStatus(t, resp, http.StatusOK)
	if vary := resp.Header.Values("Vary"); !strings.Contains(strings.Join(vary, ","), web.DialogHeader) {
		t.Errorf("the answer does not vary on the header, so a cache could serve one for the other: %q", vary)
	}
	frag := body(t, resp)
	for _, want := range []string{
		`data-task-dialog="` + ref + `"`,
		"<h1 id=\"taskdlg-title\">drawn for the dialog</h1>",
		`data-dialog-close`,
		`<p class="flash" role="status">task saved</p>`,
		`id="dlg-body" name="body"`,
		`<label class="sr-only" for="dlg-body">`,
		`id="dlg-assignee"`,
		`id="dlg-taskmenu-` + ref + `"`,
		`id="dlg-repeat"`,
		`id="dlg-prio-` + ref + `"`,
		`popovertarget="dlg-prio-` + ref + `"`,
	} {
		if !strings.Contains(frag, want) {
			t.Errorf("the dialog's answer lacks %s:\n%s", want, frag)
		}
	}
	for _, unwanted := range []string{"<html", `<aside class="side"`, `id="live-screen"`, `class="backlink"><a`, `id="body"`, `id="prio-`} {
		if strings.Contains(frag, unwanted) {
			t.Errorf("the dialog's answer carries %s", unwanted)
		}
	}

	page := b.page("/tasks/" + ref)
	for _, want := range []string{"<html", `id="live-screen"`, `class="backlink"><a`, `id="body" name="body"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the page without the header lost %s", want)
		}
	}
	if strings.Contains(page, "data-task-dialog") || strings.Contains(page, "data-dialog-close") {
		t.Error("the page without the header is drawn for the dialog")
	}

	listing := body(t, b.getDialog("/tasks"))
	if !strings.Contains(listing, "<html") {
		t.Error("a screen other than the task screen answered the header with a fragment")
	}
}

// A save made from the dialog redirects to the task as from the page, and
// the redirected request, which keeps its header, is the dialog's answer.
func TestASaveFromTheDialogComesBackAsTheDialog(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "saved in the dialog")

	resp := b.post("/tasks/"+ref+"/tags", url.Values{"tag": {"inside"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	location := resp.Header.Get("Location")
	if !strings.HasPrefix(location, "/tasks/"+ref) {
		t.Fatalf("a save on the task went to %q", location)
	}
	frag := body(t, b.getDialog(location))
	if !strings.Contains(frag, `data-task-dialog="`+ref+`"`) || !strings.Contains(frag, "inside") {
		t.Errorf("the redirected save is not the dialog's drawing of the task:\n%s", frag)
	}
}

// versionOf reads a task's version off its own screen.
func versionOf(t *testing.T, page string) int {
	t.Helper()
	m := regexp.MustCompile(`name="version" value="(\d+)"`).FindStringSubmatch(page)
	if m == nil {
		t.Fatalf("the task screen carries no version:\n%s", page)
	}
	n, _ := strconv.Atoi(m[1])
	return n
}

// A rename names only the title and the version. It changes the title and
// leaves the description and the assignee where they were.
func TestRenamingATitleLeavesTheDescriptionAndAssigneeAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "before the rename")

	page := b.page("/tasks/" + ref)
	saved := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(versionOf(t, page))},
		"title": {"before the rename"}, "body": {"kept across the rename"}, "assignee": {"alice"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)

	page = b.page("/tasks/" + ref)
	renamed := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(versionOf(t, page))},
		"title": {"after the rename"}})
	_ = renamed.Body.Close()
	wantStatus(t, renamed, http.StatusSeeOther)

	page = b.page("/tasks/" + ref)
	if !strings.Contains(page, "<h1>after the rename</h1>") {
		t.Fatalf("the rename did not take:\n%s", page)
	}
	if !strings.Contains(page, ">kept across the rename</textarea>") {
		t.Error("renaming cleared the description")
	}
	if got := inputValue(t, page, "assignee"); got != "alice" {
		t.Errorf("renaming left the assignee as %q, want alice", got)
	}

	cleared := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(versionOf(t, page))},
		"title": {"after the rename"}, "body": {""}, "assignee": {""}})
	_ = cleared.Body.Close()
	wantStatus(t, cleared, http.StatusSeeOther)
	page = b.page("/tasks/" + ref)
	if strings.Contains(page, "kept across the rename") {
		t.Error("a save naming an empty description no longer clears it")
	}
	if got := inputValue(t, page, "assignee"); got != "" {
		t.Errorf("a save naming an empty assignee left %q", got)
	}
}

// A rename against a version somebody else has moved past is refused with
// the error screen the field shows the message of, and changes nothing.
func TestRenamingAgainstAnOldVersionIsRefused(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "renamed twice")
	old := versionOf(t, b.page("/tasks/"+ref))

	first := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(old)}, "title": {"first rename"}})
	_ = first.Body.Close()
	wantStatus(t, first, http.StatusSeeOther)

	stale := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(old)}, "title": {"second rename"}})
	if stale.StatusCode != http.StatusConflict {
		t.Fatalf("a stale rename answered %d, want 409", stale.StatusCode)
	}
	refusal := body(t, stale)
	if !strings.Contains(refusal, `class="failure-message"`) || !strings.Contains(refusal, `class="failure-hint"`) {
		t.Errorf("the refusal carries no message and hint for the field to show:\n%s", refusal)
	}
	if !strings.Contains(b.page("/tasks/"+ref), "<h1>first rename</h1>") {
		t.Error("the stale rename changed the title")
	}

	empty := b.post("/tasks/"+ref, url.Values{"version": {strconv.Itoa(old + 1)}, "title": {"  "}})
	if empty.StatusCode < 400 || empty.StatusCode >= 500 {
		t.Errorf("an empty title answered %d, want a refusal", empty.StatusCode)
	}
	_ = empty.Body.Close()
}

// titleOf returns the title link a listing draws for one task, and whatever
// follows it up to the end of its element.
func titleOf(t *testing.T, page, ref string) string {
	t.Helper()
	at := strings.Index(page, `data-edit-title="`+ref+`"`)
	if at < 0 {
		t.Fatalf("no editable title for %s:\n%s", ref, page)
	}
	start := strings.LastIndex(page[:at], "<a")
	end := strings.Index(page[at:], "</svg></a>")
	if end < 0 {
		t.Fatalf("the title of %s has no open link after it", ref)
	}
	return page[start : at+end]
}

// In Edits the title, every drawing of a listing marks its titles with the
// reference and the version the rename is checked against, and draws the
// open link beside each.
func TestEveryListingMarksItsTitlesForRenaming(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "renamed where it is listed")
	version := strconv.Itoa(versionOf(t, b.page("/tasks/"+ref)))
	b.setCookie(web.TitleClickCookie, "edit")

	for _, c := range []struct{ view, path string }{
		{"list", "/tasks"},
		{"board", "/tasks"},
		{"lanes", "/tasks"},
		{"list", "/projects/infra"},
	} {
		b.setCookie(web.TaskViewCookie, c.view)
		link := titleOf(t, contentOf(t, b.page(c.path)), ref)
		for _, want := range []string{
			`data-version="` + version + `"`,
			`>renamed where it is listed</a>`,
			`<a class="title-open" href="/tasks/` + ref,
			`aria-label="Open ` + strings.ToUpper(ref) + `"`,
		} {
			if !strings.Contains(link, want) {
				t.Errorf("%s on %s: the title lacks %s:\n%s", c.view, c.path, want, link)
			}
		}
		if !strings.Contains(link, `href="/tasks/`+ref) {
			t.Errorf("%s on %s: the title is not a link to the task without scripting:\n%s", c.view, c.path, link)
		}
	}
}

// The dialog's element is what makes a click on its backdrop distinguishable
// from one on its content, and on a phone it is the whole screen.
func TestTheDialogFillsAPhoneAndKeepsItsBackdropClickable(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	if rule := declarations(t, sheet, "\ndialog.taskdlg {"); !strings.Contains(rule, "padding: 0;") {
		t.Errorf("the dialog has padding, so a click on it is not only a click on the backdrop: %s", rule)
	}
	if rule := declarations(t, sheet, ".taskdlg-body {"); !strings.Contains(rule, "overflow-y: auto") {
		t.Errorf("the dialog's content does not scroll inside it: %s", rule)
	}
	phone := between(t, sheet, "@media (max-width: 599px) {\n  dialog.taskdlg", "}")
	for _, want := range []string{"width: 100vw", "height: 100dvh", "max-height: none", "border-radius: 0"} {
		if !strings.Contains(phone, want) {
			t.Errorf("the dialog on a phone lacks %s: %s", want, phone)
		}
	}
	if rule := declarations(t, sheet, "dialog.taskdlg::backdrop"); !strings.Contains(rule, "var(--scrim)") {
		t.Errorf("the backdrop does not dim the page with the shared scrim: %s", rule)
	}
}
