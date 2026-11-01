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
	form := totpForm(t, section, web.RouteCalendarCreate)
	if !strings.Contains(form, `name="filter"`) || !strings.Contains(form, "Create feed") {
		t.Fatalf("no create form with a filter field:\n%s", section)
	}
	if !strings.Contains(section, `class="field-info tip"`) {
		t.Errorf("the section carries no (i) tip:\n%s", section)
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
	for _, action := range []string{web.RouteCalendarFilter, web.RouteCalendarRotate, web.RouteCalendarRevoke} {
		totpForm(t, section, action)
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
	totpForm(t, calendarSection(t, b.page(web.RouteSettings)), web.RouteCalendarCreate)
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
