// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

// addChannel posts the Notifications form and returns the new channel's
// screen path and the screen the answer drew, which carries the secret.
func addChannel(t *testing.T, f *fixture, b *browser, form url.Values) (string, string) {
	t.Helper()
	resp := b.post("/settings/notifications", form)
	page := body(t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("adding a channel answered %d:\n%s", resp.StatusCode, page)
	}
	channels, err := f.svc.ListInboxChannels(f.ctx())
	if err != nil || len(channels) == 0 {
		t.Fatalf("no channel after adding one: %v", err)
	}
	for _, c := range channels {
		if c.Name == form.Get("name") {
			return "/settings/notifications/" + c.ID, page
		}
	}
	t.Fatalf("no channel named %q", form.Get("name"))
	return "", ""
}

func TestSettingsAddsAChannelAndShowsItsSecretOnce(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	section := between(t, b.page("/settings"), `<section class="setgroup" id="notifications">`, "</section>\n\n<section")
	assertChannelsInACard(t, section, false)
	form := formAt(t, section, "/settings/notifications")
	for _, want := range []string{`name="name"`, `name="url"`, `<select id="ch-format" name="format"`,
		`name="kinds" value="mention" checked`, `name="kinds" value="comment" checked`, `name="active" value="on" checked`} {
		if !strings.Contains(form, want) {
			t.Errorf("the add form lacks %s:\n%s", want, form)
		}
	}

	page, first := addChannel(t, f, b, url.Values{"name": {"phone"}, "url": {"https://ntfy.invalid/alice-tix"},
		"format": {"ntfy"}, "kinds": {"mention", "assigned"}, "active": {"on"}})
	channels, err := f.svc.ListInboxChannels(f.ctx())
	if err != nil || len(channels) != 1 || len(channels[0].Kinds) != 2 || channels[0].Format != "ntfy" {
		t.Fatalf("channels = %+v, %v", channels, err)
	}
	if !strings.Contains(first, `id="channel-secret"`) {
		t.Fatal("the new channel's screen does not show its secret")
	}
	if strings.Contains(b.page(page), `id="channel-secret"`) {
		t.Fatal("the secret is shown a second time")
	}
	if c := b.cookie("tix_issued_channel"); c != "" {
		t.Fatal("the secret travelled in a cookie")
	}
	for _, want := range []string{`action="/settings/notifications/` + channels[0].ID + `/test">`,
		`action="/settings/notifications/` + channels[0].ID + `/edit">`, "Recent deliveries"} {
		if !strings.Contains(first, want) {
			t.Errorf("the channel screen lacks %s", want)
		}
	}
	settings := b.page("/settings")
	if list := between(t, settings, `<ul class="plain channels">`, "</ul>"); !strings.Contains(list, "phone") {
		t.Fatalf("settings does not list the channel:\n%s", list)
	}
	assertChannelsInACard(t, between(t, settings, `<section class="setgroup" id="notifications">`, "</section>"), true)
}

// assertChannelsInACard checks the Notifications section's shape: its
// description and channels are rows of one card, and Add a channel is a
// button beside the heading opening a dialog that holds the add form, drawn
// shut until it is asked for.
func assertChannelsInACard(t *testing.T, section string, has bool) {
	t.Helper()
	card := between(t, section, `<div class="setlist">`, `<dialog id="channel-new"`)
	if !strings.Contains(card, `<p class="setrow setlead">Send your inbox items`) {
		t.Errorf("the description is not the card's lead row:\n%s", section)
	}
	if has && !strings.Contains(card, `<li class="setrow channelrow">`) {
		t.Errorf("the channels are not rows of the card:\n%s", section)
	}
	head := between(t, section, `<div class="sechead">`, "</div>")
	if !strings.Contains(head, `href="/settings?dialog=channel-new#channel-new" data-dlg-open="channel-new"`) || !strings.Contains(head, "Add a channel") {
		t.Errorf("Add a channel is not a button beside the heading:\n%s", head)
	}
	adder := dialogBlock(section, "channel-new")
	if tag, _, _ := strings.Cut(adder, ">"); strings.Contains(tag, " open") {
		t.Errorf("Add a channel is drawn open before it was asked for:\n%s", tag)
	}
	if !strings.Contains(adder, `action="/settings/notifications"`) {
		t.Errorf("the dialog does not hold the add form:\n%s", adder)
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
	if tag, _, _ := strings.Cut(dialogBlock(refused, "channel-new"), ">"); !strings.HasSuffix(tag, " open") {
		t.Errorf("a refused channel does not come back with its dialog open: %s", tag)
	}

	page, _ := addChannel(t, f, b, url.Values{"name": {"hook"}, "url": {"https://hooks.invalid/hook"}, "kinds": {"mention"}, "active": {"on"}})
	id := strings.TrimPrefix(page, "/settings/notifications/")
	resp = b.post(page+"/edit", url.Values{"name": {"hook"}, "url": {"https://hooks.invalid/hook"}, "format": {"discord"}})
	refused = body(t, resp)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(refused, `id="kinds-error"`) {
		t.Fatalf("saving with no kind answered %d:\n%s", resp.StatusCode, refused)
	}
	b.post(page+"/edit", url.Values{"name": {"renamed"}, "url": {"https://hooks.invalid/hook"}, "format": {"discord"},
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
	resp = b.post(page+"/delete", url.Values{})
	_ = resp.Body.Close()
	wantSettingsReturn(t, resp, "notifications")
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
