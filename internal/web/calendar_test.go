// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/web"
)

// calendarSection returns the Calendar feed section of the settings screen,
// so an assertion about the feed reads the feed's controls and not the page.
func calendarSection(t *testing.T, page string) string {
	t.Helper()
	section := between(t, page, `<section class="setgroup" id="calendar">`, "</section>")
	if section == "" {
		t.Fatalf("the settings screen has no calendar section:\n%s", page)
	}
	return section
}

// stateRow is the section's first row, stating whether there is a feed.
func stateRow(t *testing.T, section string) string {
	t.Helper()
	return between(t, section, `<div class="setrow" id="calendar-state">`, `</div>
    </div>`)
}

// feedState reads the value the section's first row states for the feed,
// which is where a reader looks to learn whether there is one at all.
func feedState(t *testing.T, section string) string {
	t.Helper()
	row := between(t, section, `<div class="setrow" id="calendar-state">`, `</div>
    </div>`)
	return between(t, row+"<", `<span class="setvalue">`, `<`)
}

// feedURL reads the one address the issued screen shows.
func feedURL(t *testing.T, page string) string {
	t.Helper()
	addr := between(t, page, `<pre id="calendar-url" class="wraps"><code>`, "</code>")
	if !strings.Contains(addr, "/api/v1/calendar/feeds/tix_cal_") || !strings.HasSuffix(addr, ".ics") {
		t.Fatalf("the issued screen shows no feed address:\n%s", page)
	}
	return addr
}

func TestCalendarFeedSettingsLifecycle(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("cal@example.com", "correct horse battery")
	b := f.asActor(user.ID)

	section := calendarSection(t, b.page(web.RouteSettings))
	if got := feedState(t, section); got != "No feed" {
		t.Errorf("with no feed the section states %q, want No feed:\n%s", got, section)
	}
	form := totpForm(t, section, web.RouteCalendarCreate)
	if !strings.Contains(form, `name="filter"`) || !strings.Contains(form, "Create feed") {
		t.Fatalf("no create form with a filter field:\n%s", section)
	}
	if !strings.Contains(stateRow(t, section), `class="field-info"`) {
		t.Errorf("the state row carries no (i) tip:\n%s", section)
	}
	if strings.Contains(between(t, section, "<h2", "</h2>"), "field-info") {
		t.Errorf("the section heading carries an (i) tip, which no other section's does:\n%s", section)
	}
	if !strings.Contains(b.page(web.RouteSettings), `href="#calendar"`) {
		t.Error("the settings navigation does not lead to the calendar section")
	}

	resp := b.post(web.RouteCalendarCreate, url.Values{"filter": {"project:infra"}})
	issued := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	addr := feedURL(t, issued)
	if !strings.Contains(issued, `data-copy-target="calendar-url"`) || !strings.Contains(issued, `href="webcal://`) {
		t.Errorf("the issued screen lacks the copy button or the subscribe link:\n%s", issued)
	}
	if resp.Header.Get("Cache-Control") != "no-store" {
		t.Errorf("the issued screen may be cached: %q", resp.Header.Get("Cache-Control"))
	}
	token := strings.TrimSuffix(addr[strings.LastIndex(addr, "/")+1:], ".ics")

	settings := b.page(web.RouteSettings)
	section = calendarSection(t, settings)
	if strings.Contains(settings, token) {
		t.Fatal("settings shows the feed secret after it was issued")
	}
	if got := feedState(t, section); got != "Active" {
		t.Errorf("with a feed the section states %q, want Active:\n%s", got, section)
	}
	row := stateRow(t, section)
	if n := strings.Count(row, "Active"); n != 1 {
		t.Errorf("the state row says Active %d times, want once:\n%s", n, row)
	}
	help := between(t, row, `<span class="sethelp">`, "</span>")
	for _, want := range []string{"Created ", "shown only once", "Rotate"} {
		if !strings.Contains(help, want) {
			t.Errorf("the state row does not say where the address went (%q missing): %s", want, help)
		}
	}
	for action, control := range map[string]string{
		web.RouteCalendarFilter: "Save filter", web.RouteCalendarRotate: "Rotate", web.RouteCalendarRevoke: "Revoke",
	} {
		if form := totpForm(t, section, action); !strings.Contains(form, `type="submit"`) || !strings.Contains(form, ">"+control+"</button>") {
			t.Errorf("the form posting to %s has no %s button:\n%s", action, control, form)
		}
	}
	if inputValue(t, section, "calendar-filter") != "project:infra" {
		t.Errorf("the filter field does not hold the stored filter:\n%s", section)
	}

	resp = b.post(web.RouteCalendarFilter, url.Values{"filter": {"tag:ops"}})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	if loc := resp.Header.Get("Location"); !strings.HasSuffix(loc, "#calendar") {
		t.Errorf("saving the filter returns to %q, not the calendar section", loc)
	}

	resp = b.post(web.RouteCalendarRotate, url.Values{})
	rotated := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if next := feedURL(t, rotated); next == addr {
		t.Fatal("rotating showed the same address")
	}

	resp = b.post(web.RouteCalendarRevoke, url.Values{})
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)
	wantSettingsReturn(t, resp, "calendar")
	totpForm(t, calendarSection(t, b.page(web.RouteSettings)), web.RouteCalendarCreate)
}

// wantSettingsReturn asserts a redirect lands on one settings section with its
// toast: the flash in the query and the section as the whole fragment. A flash
// written after the "#" is part of the fragment, so the toast never shows and
// the fragment names no element, which drops the reader at the top.
func wantSettingsReturn(t *testing.T, resp *http.Response, section string) {
	t.Helper()
	loc, err := url.Parse(resp.Header.Get("Location"))
	if err != nil {
		t.Fatalf("parsing the redirect %q: %v", resp.Header.Get("Location"), err)
	}
	if loc.Path != web.RouteSettings || loc.Fragment != section || loc.Query().Get("flash") == "" {
		t.Errorf("redirected to %q, want %s with a flash and the fragment #%s", loc, web.RouteSettings, section)
	}
}

func TestCalendarFilterRefusalKeepsTheText(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	user := f.person("cal@example.com", "correct horse battery")
	b := f.asActor(user.ID)

	resp := b.post(web.RouteCalendarCreate, url.Values{"filter": {"project:nosuch"}})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusNotFound)
	section := calendarSection(t, page)
	if inputValue(t, section, "calendar-filter") != "project:nosuch" {
		t.Errorf("the refused filter was not kept in its field:\n%s", section)
	}
	if !strings.Contains(section, `class="field-error"`) || !strings.Contains(section, `aria-invalid="true"`) {
		t.Errorf("the refusal is not drawn beside the field:\n%s", section)
	}
	totpForm(t, section, web.RouteCalendarCreate)
}

// The feed's events link to the task screen, which httpapi spells on its own
// because it cannot import this package.
func TestFeedLinksPointAtTheTaskScreen(t *testing.T) {
	t.Parallel()
	if want := strings.TrimSuffix(web.RouteTask, "{ref}"); httpapi.TaskPagePrefix != want {
		t.Fatalf("feed events link to %q, the task screen is at %q", httpapi.TaskPagePrefix, want)
	}
}
