// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

// paneDivider is the separator the host draws between the listing and the
// panel.
const paneDivider = `data-pane-resize`

// csrfField is a form's token, which differs between any two pages.
var csrfField = regexp.MustCompile(`name="csrf_token" value="[^"]*"`)

// hostTag is the dialog element's opening tag, or "" when the page has none.
func hostTag(page string) string {
	at := strings.Index(page, dialogHost)
	if at < 0 {
		return ""
	}
	end := strings.Index(page[at:], ">")
	return page[at : at+end+1]
}

// Panel is a third position of Task opens in: stored as pane, marked
// current alone on Settings, and left for the page by a post naming none.
func TestPanelIsAThirdPositionOfTaskOpensIn(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	for _, step := range []struct{ named, want string }{
		{"pane", "pane"},
		{"pane", "pane"},
		{"", "page"},
		{"pane", "pane"},
		{"dialog", "dialog"},
		{"pane", "pane"},
	} {
		form := url.Values{"next": {"/settings"}}
		if step.named != "" {
			form.Set("open", step.named)
		}
		resp := b.post(web.RouteTaskOpen, form)
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		if got := b.cookie(web.TaskOpenCookie); got != step.want {
			t.Errorf("posting open=%q stored %q, want %q", step.named, got, step.want)
		}
	}

	for _, c := range []struct{ cookie, current string }{
		{"pane", "pane"},
		{"dialog", "dialog"},
		{"page", "page"},
	} {
		b.setCookie(web.TaskOpenCookie, c.cookie)
		listings := between(t, contentOf(t, b.page("/settings")), `id="listings"`, "</section>")
		form := prefForm(t, listings, web.RouteTaskOpen)
		for _, pos := range []struct{ value, label string }{{"page", "Page"}, {"dialog", "Dialog"}, {"pane", "Panel"}} {
			seg := segmentOf(t, form, "open", pos.value)
			if !strings.HasSuffix(seg, ">"+pos.label) {
				t.Errorf("with %s, the %s segment does not read %q: %s", c.cookie, pos.value, pos.label, seg)
			}
			if marked := strings.Contains(seg, `aria-current="true"`); marked != (pos.value == c.current) {
				t.Errorf("with %s, the %s segment is marked current=%v: %s", c.cookie, pos.value, marked, seg)
			}
		}
	}
}

// The host is marked for the panel and carries its divider only with
// Panel; Dialog draws the bare dialog, Page none, and the task screen never
// carries a host for itself.
func TestThePanelHostIsDrawnOnlyWithPanel(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "opened beside the list")

	b.setCookie(web.TaskOpenCookie, "pane")
	for _, path := range []string{"/tasks", "/inbox", "/projects/infra"} {
		page := b.page(path)
		tag := hostTag(page)
		if !strings.Contains(tag, "data-task-pane") {
			t.Errorf("%s with Panel draws a host not marked for it: %q", path, tag)
		}
		host := between(t, page, dialogHost, "</dialog>")
		divider := between(t, host, `<div class="pane-resize"`, "</div>")
		if divider == "" {
			t.Fatalf("%s with Panel has no divider in the host:\n%s", path, host)
		}
		for _, want := range []string{paneDivider, `role="separator"`, `aria-orientation="vertical"`, `aria-controls="taskdlg"`, `aria-label="Resize task panel"`, `tabindex="0"`} {
			if !strings.Contains(divider, want) {
				t.Errorf("%s: the divider lacks %s: %s", path, want, divider)
			}
		}
	}
	if strings.Contains(b.page("/tasks/"+ref), dialogHost) {
		t.Error("the task screen carries a panel for itself")
	}

	b.setCookie(web.TaskOpenCookie, "dialog")
	page := b.page("/tasks")
	if tag := hostTag(page); tag == "" || strings.Contains(tag, "data-task-pane") {
		t.Errorf("Dialog draws no host, or one marked for the panel: %q", tag)
	}
	if strings.Contains(page, paneDivider) {
		t.Error("Dialog draws the panel's divider")
	}

	b.setCookie(web.TaskOpenCookie, "page")
	if page := b.page("/tasks"); strings.Contains(page, dialogHost) || strings.Contains(page, paneDivider) {
		t.Error("Page draws a host or a divider")
	}
}

// Without scripting Panel changes nothing a listing draws: a row is the same
// markup, its title the same link to the task's own page, as with Page.
func TestPanelLeavesEveryRowAPlainLink(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "a row with a link")

	rowOf := func(cookie string) string {
		b.setCookie(web.TaskOpenCookie, cookie)
		row := between(t, contentOf(t, b.page("/tasks")), `<li id="t-`+ref+`"`, "</li>")
		if row == "" {
			t.Fatalf("with %s the list has no row for %s", cookie, ref)
		}
		return csrfField.ReplaceAllString(row, `name="csrf_token" value=""`)
	}
	page, pane := rowOf("page"), rowOf("pane")
	if pane != page {
		t.Errorf("Panel draws the row differently from Page:\npage: %s\npane: %s", page, pane)
	}
	if !strings.Contains(pane, `href="/tasks/`+ref) {
		t.Errorf("the row's title is not a link to the task's page: %s", pane)
	}
}

// The panel is fixed beside the page, which gives it the room, its divider
// is drawn only while it is the panel, and it moves only for a reader who
// has not asked for less motion.
func TestThePanelSitsBesideThePageAndMovesOnlyWhenAllowed(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	sheet := body(t, f.as("alice").get("/assets/app.css"))

	rule := declarations(t, sheet, "\ndialog.taskdlg[data-shown=\"pane\"] {")
	for _, want := range []string{"position: fixed", "inset: 0 0 0 auto", "width: var(--pane-w, 460px)", "height: 100dvh"} {
		if !strings.Contains(rule, want) {
			t.Errorf("the panel lacks %s: %s", want, rule)
		}
	}
	if rule := declarations(t, sheet, ":root[data-pane-open] .shell"); !strings.Contains(rule, "margin-right: var(--pane-w, 460px)") {
		t.Errorf("the page does not give the panel its room: %s", rule)
	}
	if rule := declarations(t, sheet, "\n.pane-resize {"); !strings.Contains(rule, "display: none") {
		t.Errorf("the divider is drawn while the element is a dialog: %s", rule)
	}
	if rule := declarations(t, sheet, "[data-shortcut-row][aria-current=\"true\"]"); !strings.Contains(rule, "background-color") {
		t.Errorf("the row in the panel is not marked: %s", rule)
	}
	part := between(t, sheet, "The task panel (Task opens in: Panel)", "@keyframes pane-in")
	motion := between(t, part, "@media (prefers-reduced-motion: no-preference) {", "\n}")
	if !strings.Contains(motion, "animation: pane-in") {
		t.Errorf("the panel's entrance is not under prefers-reduced-motion: %s", motion)
	}
	if outside := strings.Replace(part, motion, "", 1); strings.Contains(outside, "animation:") || strings.Contains(outside, "transition:") {
		t.Error("the panel moves outside the reduced-motion guard")
	}
}
