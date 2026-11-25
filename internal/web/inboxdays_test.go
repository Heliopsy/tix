// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// inboxList is the inbox's list region.
func inboxList(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `<section id="inbox-list"`, "</section>")
}

func TestTheInboxGroupsItemsUnderTodayAndFiltersByKind(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cut over")
	carolMentionsAlice(t, f, ref)

	// The service's clock stands at the start of 2026, so the item is
	// older than yesterday; TestInboxDaysAreCountedInTheReadersZone covers
	// the boundaries.
	list := inboxList(t, b.page("/inbox"))
	if !strings.Contains(list, `<h2 class="inbox-day" id="inbox-day-earlier">Earlier</h2>`) {
		t.Fatalf("an old item is not under Earlier:\n%s", list)
	}
	if strings.Contains(list, "inbox-day-yesterday") || strings.Contains(list, "inbox-day-today") {
		t.Fatalf("an empty day heading was drawn:\n%s", list)
	}

	bar := between(t, b.page("/inbox"), `<nav class="inbox-kinds"`, "</nav>")
	for _, want := range []string{
		`href="/inbox?kinds=mention" class="chip inbox-kind" data-kind="mention" aria-pressed="false"`,
		`href="/inbox?kinds=comment" class="chip inbox-kind" data-kind="comment"`,
	} {
		if !strings.Contains(bar, want) {
			t.Errorf("the kind chips lack %q:\n%s", want, bar)
		}
	}

	if list := inboxList(t, b.page("/inbox?kinds=assigned,comment")); strings.Contains(list, "carol mentioned you") ||
		!strings.Contains(list, "Nothing of these kinds.") {
		t.Fatalf("filtering to other kinds still lists the mention:\n%s", list)
	}
	page := b.page("/inbox?kinds=mention&show=unread")
	if list := inboxList(t, page); !strings.Contains(list, "carol mentioned you") {
		t.Fatalf("filtering to mentions lost the mention:\n%s", list)
	}
	bar = between(t, page, `<div class="inbox-bar"`, "</div>")
	for _, want := range []string{
		`href="/inbox?kinds=mention" class="segment"`,
		`href="/inbox?kinds=mention&amp;show=unread" class="segment is-on"`,
		`href="/inbox?show=unread" class="chip inbox-kind is-on" data-kind="mention" aria-pressed="true"`,
	} {
		if !strings.Contains(bar, want) {
			t.Errorf("the filtered bar lacks %q:\n%s", want, bar)
		}
	}
}

func TestStopThisKindFromAnInboxItemMutesIt(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "cut over")
	carolMentionsAlice(t, f, ref)

	list := inboxList(t, b.page("/inbox"))
	form := between(t, list, `<form method="post" class="inbox-stop" action="/settings/inbox">`, "</form>")
	for _, want := range []string{`name="kind" value="mention"`, `name="receive" value="off"`, `name="stop" value="1"`,
		"Stop mentions"} {
		if !strings.Contains(form, want) {
			t.Fatalf("the row's stop form lacks %q:\n%s", want, form)
		}
	}

	resp := b.post("/settings/inbox", url.Values{"kind": {"mention"}, "receive": {"off"}, "stop": {"1"}, "next": {"/inbox"}})
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("stopping answered %d", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); !strings.HasPrefix(loc, "/inbox?flash=") ||
		!strings.Contains(loc, url.QueryEscape("Mentions no longer reach your inbox")) {
		t.Fatalf("stopping went to %q", loc)
	}
	prefs, err := f.svc.GetInboxPrefs(f.ctx())
	if err != nil || !prefs.Mutes(core.InboxMention) {
		t.Fatalf("mentions are not muted: %+v, %v", prefs, err)
	}
	if list := inboxList(t, b.page("/inbox")); strings.Contains(list, "inbox-stop") || !strings.Contains(list, "carol mentioned you") {
		t.Fatalf("after stopping, the item went or still offers Stop:\n%s", list)
	}
}

func TestTheInboxSummaryCountsLateWorkAndTheTenant(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	late := time.Now().Add(-72 * time.Hour)
	if _, err := f.svc.CreateTask(f.ctx(), core.CreateTaskInput{ProjectRef: "infra", Title: "late one",
		AssigneeActorID: "alice", DueAt: &late}); err != nil {
		t.Fatal(err)
	}
	b.createTask("infra", "open one")

	card := between(t, b.page("/inbox"), `<section class="card inbox-summary"`, "</section>")
	for _, want := range []string{
		`<a class="inbox-stat is-late" href="/mine" data-stat="overdue"><span class="inbox-stat-n">1</span>`,
		`data-stat="today"><span class="inbox-stat-n">0</span>`,
		`data-stat="open"><span class="inbox-stat-n">2</span>`,
		`href="/stats?days=7" data-stat="done"><span class="inbox-stat-n">0</span>`,
	} {
		if !strings.Contains(card, want) {
			t.Errorf("the summary lacks %q:\n%s", want, card)
		}
	}
}
