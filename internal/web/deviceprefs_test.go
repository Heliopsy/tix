// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/web"
)

func TestTheStartScreenDecidesWhereTheRootLands(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	root := func() string {
		t.Helper()
		resp := b.get("/")
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		return resp.Header.Get("Location")
	}
	if got := root(); got != "/projects" {
		t.Fatalf("an untouched browser lands on %q, want /projects", got)
	}
	form := prefForm(t, contentOf(t, b.page("/settings")), web.RouteStartScreen)
	for _, want := range []string{`<option value="inbox" >Inbox</option>`, `<option value="" selected>Projects</option>`, "On this device"} {
		if !strings.Contains(form, want) {
			t.Errorf("the start screen picker lacks %q:\n%s", want, form)
		}
	}
	for start, want := range map[string]string{"inbox": "/inbox", "mine": "/mine", "tasks": "/tasks", "nowhere": "/projects", "": "/projects"} {
		resp := b.post(web.RouteStartScreen, url.Values{"start": {start}, "next": {"/settings"}})
		_ = resp.Body.Close()
		wantStatus(t, resp, http.StatusSeeOther)
		if got := root(); got != want {
			t.Errorf("start %q lands on %q, want %q", start, got, want)
		}
	}
	b.setCookie(web.StartScreenCookie, "inbox")
	if !strings.Contains(prefForm(t, contentOf(t, b.page("/settings")), web.RouteStartScreen), `<option value="inbox" selected>`) {
		t.Error("the picker does not show the chosen screen")
	}
	if form := formAt(t, body(t, f.as("").get("/login")), "/login"); !strings.Contains(form, `name="next" value="/"`) {
		t.Errorf("a sign-in with no link to follow does not go through the start screen:\n%s", form)
	}
}

func TestReduceMotionIsAPreferenceOnTheRootElement(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	if tag := htmlTag(t, b.page("/tasks")); strings.Contains(tag, "data-motion") {
		t.Fatalf("an untouched browser has reduce motion on: %s", tag)
	}
	if form := prefForm(t, contentOf(t, b.page("/settings")), web.RouteMotion); !strings.Contains(form, `role="switch" aria-checked="false"`) {
		t.Fatalf("reduce motion is not a switch reading off:\n%s", form)
	}
	on := b.post(web.RouteMotion, url.Values{"next": {"/tasks"}})
	_ = on.Body.Close()
	wantStatus(t, on, http.StatusSeeOther)
	if tag := htmlTag(t, b.page("/projects")); !strings.Contains(tag, `data-motion="reduce"`) {
		t.Fatalf("turning reduce motion on left the root without it: %s", tag)
	}
	if form := prefForm(t, contentOf(t, b.page("/settings")), web.RouteMotion); !strings.Contains(form, `aria-checked="true"`) {
		t.Fatalf("the switch does not read on:\n%s", form)
	}
	off := b.post(web.RouteMotion, url.Values{"next": {"/tasks"}})
	_ = off.Body.Close()
	if tag := htmlTag(t, b.page("/tasks")); strings.Contains(tag, "data-motion") {
		t.Errorf("turning it off left the attribute: %s", tag)
	}
}

func TestEverySettingSaysWhereItIsKept(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	main := contentOf(t, b.page("/settings"))
	for _, c := range []struct{ action, scope string }{
		{"/tint", "device"}, {"/language", "device"}, {"/hover", "device"}, {"/taskopen", "device"},
		{"/projectlabel", "device"}, {"/keyscheme", "device"}, {web.RouteMotion, "device"},
		{"/timezone", "account"}, {"/timeformat", "account"}, {"/settings/inbox", "account"},
	} {
		if form := prefForm(t, main, c.action); !strings.Contains(form, `<span class="setscope" data-scope="`+c.scope+`"`) {
			t.Errorf("the setting posting to %s is not labelled %s:\n%s", c.action, c.scope, form)
		}
	}
	for _, id := range []string{"second-factor", "sessions", "calendar"} {
		head := between(t, main, `id="`+id+`"`, "</h2>")
		_, rest, _ := strings.Cut(main, `id="`+id+`"`)
		after := between(t, rest, "</h2>", "</div>")
		if head == "" || !strings.Contains(after, `data-scope="account"`) {
			t.Errorf("section %s does not say it is kept with the account: %q", id, after)
		}
	}
	if !strings.Contains(main, `data-setfind`) || !strings.Contains(main, "No setting matches") {
		t.Error("settings carries no search box")
	}
	tenant := contentOf(t, b.page("/admin/tenant"))
	if head, _, _ := strings.Cut(tenant, "<h2"); !strings.Contains(head, `data-scope="tenant" title=`) ||
		!strings.Contains(head, ">Whole tenant</span>") {
		t.Error("the tenant screen does not say its settings are for the whole tenant")
	}
}

func TestBrowserNotificationsAreOfferedOnSettings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	main := contentOf(t, f.as("alice").page("/settings"))
	section := between(t, main, `id="notifications"`, "</section>")
	for _, want := range []string{
		`data-notify-toggle`, `data-notify-hide`, "Notify me about new inbox items", "Hide what they say",
		`<div class="setrow" data-needs-js>`,
	} {
		if !strings.Contains(section, want) {
			t.Errorf("the Notifications section lacks %q", want)
		}
	}
	if !strings.Contains(section, `data-scope="device"`) {
		t.Error("the browser notification switches do not say they are kept on this device")
	}
}
