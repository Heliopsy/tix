// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// carolMentionsAlice seeds a second person in alice's tenant and has her
// mention alice in a comment, which is one unread item in alice's inbox.
func carolMentionsAlice(t *testing.T, f *fixture, ref string) core.Actor {
	t.Helper()
	carol := seedActor(t, f.store, f.tenantA.ID, "carol", core.RoleMember)
	ctx := core.WithActor(core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID}),
		copyActor(carol, []core.Scope{core.ScopeAll}, core.RoleMember))
	if _, err := f.svc.AddComment(ctx, core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)}, "@alice can you look"); err != nil {
		t.Fatalf("commenting as carol: %v", err)
	}
	return carol
}

// inboxEntry is the sidebar's Inbox link, badge included.
func inboxEntry(t *testing.T, page string) string {
	t.Helper()
	entry := between(t, nav(t, page), `<a href="/inbox"`, "</a>")
	if entry == "" {
		t.Fatalf("the sidebar has no Inbox entry:\n%s", nav(t, page))
	}
	return entry
}

func unreadOnScreen(t *testing.T, f *fixture) int {
	t.Helper()
	n, err := f.svc.CountInbox(f.ctx())
	if err != nil {
		t.Fatal(err)
	}
	return n.Unread
}

func TestTheInboxListsAMentionAndTheSidebarCountsIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cut over")

	if entry := inboxEntry(t, b.page("/tasks")); !strings.Contains(entry, `data-inbox-count="0"`) || !strings.Contains(entry, " hidden>") {
		t.Fatalf("an empty inbox drew a badge: %s", entry)
	}
	carolMentionsAlice(t, f, ref)

	entry := inboxEntry(t, b.page("/tasks"))
	if !strings.Contains(entry, `data-inbox-count="1"`) || strings.Contains(entry, " hidden>") ||
		!strings.Contains(entry, `data-inbox="/inbox/badge"`) {
		t.Fatalf("the badge does not show one unread item: %s", entry)
	}

	page := b.page("/inbox")
	list := between(t, page, `<section id="inbox-list"`, "</section>")
	for _, want := range []string{
		`class="inbox-item is-unread"`, "carol mentioned you",
		`href="/tasks/` + ref + `" data-inbox-open=`, "data-inbox-unread", "cut over", "@alice can you look",
		`action="/inbox/read">`,
	} {
		if !strings.Contains(list, want) {
			t.Errorf("the inbox list lacks %q:\n%s", want, list)
		}
	}
	if !strings.Contains(list, "data-inplace") && !strings.Contains(page, `id="inbox-list" class="card inbox" data-inplace`) {
		t.Error("the inbox list is not an in-place region")
	}

	badge := b.page("/inbox/badge")
	if !strings.HasPrefix(strings.TrimSpace(badge), `<span class="navbadge" id="inbox-badge" data-inbox-count="1"`) {
		t.Fatalf("the badge fragment = %q", badge)
	}
}

func TestInboxFormsMarkReadUnreadAndEverything(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cut over")
	carolMentionsAlice(t, f, ref)
	carolSays := func(body string) {
		t.Helper()
		carol, err := f.svc.GetActor(f.ctx(), "carol")
		if err != nil {
			t.Fatal(err)
		}
		ctx := core.WithActor(core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID}),
			copyActor(*carol, []core.Scope{core.ScopeAll}, core.RoleMember))
		if _, err := f.svc.AddComment(ctx, core.TaskRef{ProjectKey: "infra", Seq: seqOfRef(t, ref)}, body); err != nil {
			t.Fatal(err)
		}
	}
	carolSays("@alice and again")

	items, _, err := f.svc.ListInbox(f.ctx(), core.InboxFilter{})
	if err != nil || len(items) != 2 {
		t.Fatalf("items = %d, err = %v", len(items), err)
	}
	post := func(path string, form url.Values) {
		t.Helper()
		form.Set("next", "/inbox")
		resp := b.post(path, form)
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusFound {
			t.Fatalf("POST %s answered %d", path, resp.StatusCode)
		}
	}
	post("/inbox/read", url.Values{"id": {items[0].ID}})
	if unreadOnScreen(t, f) != 1 {
		t.Fatal("marking one read left two unread")
	}
	page := b.page("/inbox?show=unread")
	if strings.Contains(page, `id="i-`+items[0].ID+`"`) || !strings.Contains(page, `id="i-`+items[1].ID+`"`) {
		t.Fatal("the unread filter does not show exactly the unread item")
	}
	if !strings.Contains(b.page("/inbox"), `action="/inbox/unread">`) {
		t.Fatal("a read item offers no Mark unread")
	}
	post("/inbox/unread", url.Values{"id": {items[0].ID}})
	if unreadOnScreen(t, f) != 2 {
		t.Fatal("marking unread did not")
	}
	if !strings.Contains(b.page("/inbox"), `action="/inbox/read-all">`) {
		t.Fatal("an inbox with unread items offers no Mark all read")
	}
	post("/inbox/read-all", url.Values{})
	if unreadOnScreen(t, f) != 0 {
		t.Fatal("mark all read left items unread")
	}
	if strings.Contains(b.page("/inbox"), `action="/inbox/read-all">`) {
		t.Fatal("Mark all read is offered with nothing unread")
	}
	resp := b.post("/inbox/read", url.Values{"id": {"01J000000000000000000A"}, "next": {"/inbox"}})
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("marking an unknown item answered %d, want 404", resp.StatusCode)
	}
}

func TestSettingsMuteAKindOfInboxItem(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	page := b.page("/settings")
	section := between(t, page, `<section class="setgroup" id="inbox">`, "</section>")
	for _, kind := range core.InboxKinds() {
		if !strings.Contains(section, `name="kind" value="`+string(kind)+`"`) {
			t.Errorf("settings offers no switch for %s", kind)
		}
	}
	if strings.Count(section, `aria-checked="true"`) != 4 {
		t.Fatalf("every kind should be on by default:\n%s", section)
	}
	b.post("/settings/inbox", url.Values{"kind": {"comment"}, "receive": {"off"}})
	prefs, err := f.svc.GetInboxPrefs(f.ctx())
	if err != nil || !slices.Equal(prefs.Muted, []core.InboxKind{core.InboxComment}) {
		t.Fatalf("muted = %v, err = %v, want comment", prefs, err)
	}
	section = between(t, b.page("/settings"), `<section class="setgroup" id="inbox">`, "</section>")
	comment := between(t, section, `value="comment">`, "</form>")
	if !strings.Contains(comment, `aria-checked="false"`) || !strings.Contains(comment, `name="receive" value="on"`) {
		t.Fatalf("the comment switch does not read off:\n%s", comment)
	}
	b.post("/settings/inbox", url.Values{"kind": {"comment"}, "receive": {"on"}})
	if prefs, _ := f.svc.GetInboxPrefs(f.ctx()); len(prefs.Muted) != 0 {
		t.Fatalf("turning comments back on left %v muted", prefs.Muted)
	}
}

func TestCommentAndDescriptionBoxesOfferMentions(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cut over")
	page := b.page("/tasks/" + ref)
	for _, id := range []string{`id="comment-body"`, `id="body"`} {
		box := between(t, page, "<textarea "+id, ">")
		if !strings.Contains(box, `data-mention="actor-handles"`) {
			t.Errorf("textarea %s offers no mentions: %s", id, box)
		}
	}
	if !strings.Contains(page, `<datalist id="actor-handles">`) {
		t.Error("the handles the mentions complete from are not on the page")
	}
	if box := between(t, b.page("/tasks"), `<textarea id="new-body"`, ">"); !strings.Contains(box, `data-mention="new-handles"`) {
		t.Errorf("the new task description offers no mentions: %s", box)
	}
}
