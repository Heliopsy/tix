// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// addChannel posts the Notifications form and returns where it sent the
// browser, which is the new channel's own screen.
func addChannel(t *testing.T, b *browser, form url.Values) string {
	t.Helper()
	resp := b.post("/settings/notifications", form)
	if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
		t.Fatalf("adding a channel answered %d:\n%s", resp.StatusCode, body(t, resp))
	}
	loc, _ := url.Parse(resp.Header.Get("Location"))
	return loc.Path
}

func TestSettingsAddsAChannelAndShowsItsSecretOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	section := between(t, b.page("/settings"), `<section class="setgroup" id="notifications">`, "</section>\n\n<section")
	form := formAt(t, section, "/settings/notifications")
	for _, want := range []string{`name="name"`, `name="url"`, `<select id="ch-format" name="format"`,
		`name="kinds" value="mention" checked`, `name="kinds" value="comment" checked`, `name="active" value="on" checked`} {
		if !strings.Contains(form, want) {
			t.Errorf("the add form lacks %s:\n%s", want, form)
		}
	}

	page := addChannel(t, b, url.Values{"name": {"phone"}, "url": {"https://ntfy.sh/alice-tix"},
		"format": {"ntfy"}, "kinds": {"mention", "assigned"}, "active": {"on"}})
	if !strings.HasPrefix(page, "/settings/notifications/") {
		t.Fatalf("adding a channel sent the browser to %q", page)
	}
	channels, err := f.svc.ListInboxChannels(f.ctx())
	if err != nil || len(channels) != 1 || len(channels[0].Kinds) != 2 || channels[0].Format != "ntfy" {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	first := b.page(page)
	if !strings.Contains(first, `id="channel-secret"`) {
		t.Fatal("the new channel's screen does not show its secret")
	}
	if strings.Contains(b.page(page), `id="channel-secret"`) {
		t.Fatal("the secret is shown a second time")
	}
	for _, want := range []string{`action="/settings/notifications/` + channels[0].ID + `/test">`,
		`action="/settings/notifications/` + channels[0].ID + `/edit">`, "Recent deliveries"} {
		if !strings.Contains(first, want) {
			t.Errorf("the channel screen lacks %s", want)
		}
	}
	if list := between(t, b.page("/settings"), `<ul class="plain channels">`, "</ul>"); !strings.Contains(list, "phone") {
		t.Fatalf("settings does not list the channel:\n%s", list)
	}
}

func TestChannelFormsRefuseInPlaceTestAndRemove(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post("/settings/notifications", url.Values{"name": {"bad"}, "url": {"ftp://example.com/x"}, "kinds": {"mention"}})
	refused := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(refused, `id="url-error"`) ||
		!strings.Contains(refused, `value="bad"`) {
		t.Fatalf("a refused channel answered %d without the message beside the url:\n%s", resp.StatusCode, refused)
	}

	page := addChannel(t, b, url.Values{"name": {"hook"}, "url": {"https://example.com/hook"}, "kinds": {"mention"}, "active": {"on"}})
	id := strings.TrimPrefix(page, "/settings/notifications/")
	resp = b.post(page+"/edit", url.Values{"name": {"hook"}, "url": {"https://example.com/hook"}, "format": {"discord"}})
	refused = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(refused, `id="kinds-error"`) {
		t.Fatalf("saving with no kind answered %d:\n%s", resp.StatusCode, refused)
	}
	b.post(page+"/edit", url.Values{"name": {"renamed"}, "url": {"https://example.com/hook"}, "format": {"discord"},
		"kinds": {"status"}, "active": {"on"}})
	channels, _ := f.svc.ListInboxChannels(f.ctx())
	if channels[0].Name != "renamed" || channels[0].Format != "discord" || channels[0].Kinds[0] != "status" {
		t.Fatalf("after saving = %+v", channels[0])
	}

	b.post(page+"/test", url.Values{})
	screen := b.page(page)
	if !strings.Contains(between(t, screen, `<table class="deliveries">`, "</table>"), "test") {
		t.Fatal("the test delivery is not listed")
	}
	if !strings.Contains(screen, `action="`+page+`/redeliver">`) {
		t.Fatal("a delivery not yet delivered offers no Redeliver")
	}
	b.post(page+"/delete", url.Values{})
	if left, _ := f.svc.ListInboxChannels(f.ctx()); len(left) != 0 {
		t.Fatalf("removing left %v", left)
	}
	if resp := b.get("/settings/notifications/" + id); resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a removed channel's screen answered %d", resp.StatusCode)
	}
}

func TestTheInboxListRedrawsLiveOnInboxUpdated(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	page := f.as("alice").page("/inbox?show=unread")
	binding := between(t, page, `<div id="live-screen"`, ">")
	for _, want := range []string{`data-live="/inbox?show=unread"`, `data-live-regions="inbox-bar inbox-list"`,
		`inbox.updated`} {
		if !strings.Contains(binding, want) {
			t.Errorf("the inbox's live binding lacks %s: %s", want, binding)
		}
	}
	if !strings.Contains(page, `<div class="inbox-bar" id="inbox-bar">`) {
		t.Error("the bar the redraw replaces has no id")
	}
}
