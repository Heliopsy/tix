// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// inboxOf lists an actor's whole inbox as "kind ref" lines, newest first.
func inboxOf(t *testing.T, l *Local, ctx context.Context) []string {
	t.Helper()
	items, _, err := l.ListInbox(ctx, core.InboxFilter{Page: core.Page{Limit: 100}})
	if err != nil {
		t.Fatalf("listing inbox: %v", err)
	}
	out := []string{}
	for _, it := range items {
		out = append(out, string(it.Kind)+" "+it.TaskRef)
	}
	return out
}

type inboxFixture struct {
	l                  *Local
	scope              core.TenantScope
	ada, bob, carol    *core.Actor
	adaC, bobC, carolC context.Context
}

func newInboxFixture(t *testing.T) inboxFixture {
	t.Helper()
	l, ctx, scope, me, _ := newTaskFixture(t)
	bob := seedTaskActor(t, l, scope, "bob", core.RoleMember, core.ScopeAll)
	carol := seedTaskActor(t, l, scope, "carol", core.RoleMember, core.ScopeAll)
	return inboxFixture{l: l, scope: scope, ada: me, bob: bob, carol: carol,
		adaC: ctx, bobC: taskContext(bob), carolC: taskContext(carol)}
}

func TestMentionsParseHandlesNotAddresses(t *testing.T) {
	cases := []struct {
		text string
		want []string
	}{
		{"@bob can you look", []string{"bob"}},
		{"thanks @Bob.", []string{"bob"}},
		{"mail me@example.com", nil},
		{"(@carol) and @bob, @bob again", []string{"carol", "bob"}},
		{"@@bob", nil},
		{"no mention", nil},
	}
	for _, tc := range cases {
		if got := mentions(tc.text); !slices.Equal(got, tc.want) {
			t.Errorf("mentions(%q) = %v, want %v", tc.text, got, tc.want)
		}
	}
	if got := newMentions("@bob @carol", "@bob"); !slices.Equal(got, []string{"carol"}) {
		t.Errorf("newMentions = %v, want only carol", got)
	}
}

func TestChangesThatConcernSomebodyReachTheirInbox(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "cut over",
		AssigneeActorID: "bob", Body: "@carol owns the DNS"})
	ref := core.TaskRef{ID: task.ID}

	if got, want := inboxOf(t, f.l, f.bobC), []string{"assigned " + task.Ref}; !slices.Equal(got, want) {
		t.Fatalf("bob after assignment = %v, want %v", got, want)
	}
	if got, want := inboxOf(t, f.l, f.carolC), []string{"mention " + task.Ref}; !slices.Equal(got, want) {
		t.Fatalf("carol after description mention = %v, want %v", got, want)
	}

	if err := f.l.StarTask(f.carolC, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.AddComment(f.adaC, ref, "status update"); err != nil {
		t.Fatal(err)
	}
	for _, who := range []context.Context{f.bobC, f.carolC} {
		if got := inboxOf(t, f.l, who); got[0] != "comment "+task.Ref {
			t.Fatalf("watcher's newest item = %q, want a comment", got[0])
		}
	}

	if _, err := f.l.TransitionTask(f.adaC, ref, core.TransitionInput{To: "doing"}); err != nil {
		t.Fatal(err)
	}
	items, _, err := f.l.ListInbox(f.bobC, core.InboxFilter{})
	if err != nil {
		t.Fatal(err)
	}
	if it := items[0]; it.Kind != core.InboxStatus || it.FromStatus != "todo" || it.ToStatus != "doing" ||
		it.ActorHandle != "alice" || it.TaskTitle != "cut over" {
		t.Fatalf("bob's status item = %+v", it)
	}
	if got := inboxOf(t, f.l, f.carolC); len(got) != 2 {
		t.Fatalf("a status change reached a watcher who is not the assignee: %v", got)
	}
	if got := inboxOf(t, f.l, f.adaC); len(got) != 0 {
		t.Fatalf("the actor received items for her own changes: %v", got)
	}
}

func TestOneChangeGivesOneItemWithTheStrongestReason(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
	ref := core.TaskRef{ID: task.ID}
	if err := f.l.StarTask(f.bobC, ref); err != nil {
		t.Fatal(err)
	}
	before := len(inboxOf(t, f.l, f.bobC))
	events, _ := countRows(t, f.l, f.scope)
	if _, err := f.l.AddComment(f.adaC, ref, "@bob @BOB please, @nobody"); err != nil {
		t.Fatal(err)
	}
	got := inboxOf(t, f.l, f.bobC)
	if len(got) != before+1 || got[0] != "mention "+task.Ref {
		t.Fatalf("bob = %v, want exactly one new mention", got)
	}
	after, _ := countRows(t, f.l, f.scope)
	if after != events+2 {
		t.Fatalf("events %d -> %d, want comment.added and one inbox.updated", events, after)
	}
	last := lastEvent(t, f.l, f.scope)
	if last.Type != core.EventInboxUpdated || !strings.Contains(strings.Join(anyStrings(last.Payload["recipients"]), ","), f.bob.ID) {
		t.Fatalf("last event = %s %v, want inbox.updated naming bob", last.Type, last.Payload)
	}
}

func TestAStatusChangeOutranksItsOwnComment(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
	if _, err := f.l.TransitionTask(f.adaC, core.TaskRef{ID: task.ID}, core.TransitionInput{To: "doing", Comment: "starting"}); err != nil {
		t.Fatal(err)
	}
	if got := inboxOf(t, f.l, f.bobC); !slices.Equal(got, []string{"status " + task.Ref, "assigned " + task.Ref}) {
		t.Fatalf("bob = %v, want one status item for the move and its comment", got)
	}
}

func TestReassigningAndReleasingReachTheAssignee(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t"})
	ref := core.TaskRef{ID: task.ID}
	bob := "bob"
	if _, err := f.l.UpdateTask(f.adaC, ref, core.UpdateTaskInput{AssigneeActorID: &bob}); err != nil {
		t.Fatal(err)
	}
	title := "renamed"
	if _, err := f.l.UpdateTask(f.adaC, ref, core.UpdateTaskInput{Title: &title}); err != nil {
		t.Fatal(err)
	}
	if got := inboxOf(t, f.l, f.bobC); !slices.Equal(got, []string{"assigned " + task.Ref}) {
		t.Fatalf("bob after reassignment and a rename = %v, want one assignment", got)
	}
	claim, err := f.l.ClaimTask(f.adaC, ref, core.ClaimInput{})
	if err != nil {
		t.Fatal(err)
	}
	if err := f.l.ReleaseLease(f.adaC, ref, claim.LeaseToken, core.ReleaseInput{Status: "doing"}); err != nil {
		t.Fatal(err)
	}
	if got := firstItem(t, f.l, f.bobC); got.Kind != core.InboxStatus || got.ToStatus != "doing" {
		t.Fatalf("bob after a release into doing = %+v, want a status item", got)
	}
}

func TestSelfAndMutedKindsCreateNothing(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "alice"})
	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@alice note to self"); err != nil {
		t.Fatal(err)
	}
	if got := inboxOf(t, f.l, f.adaC); len(got) != 0 {
		t.Fatalf("own actions reached own inbox: %v", got)
	}

	if _, err := f.l.PutInboxPrefs(f.bobC, core.InboxPrefsInput{Muted: []string{"comment"}}); err != nil {
		t.Fatal(err)
	}
	mine := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "u", AssigneeActorID: "bob"})
	events, _ := countRows(t, f.l, f.scope)
	if _, err := f.l.AddComment(f.adaC, core.TaskRef{ID: mine.ID}, "plain"); err != nil {
		t.Fatal(err)
	}
	if got := inboxOf(t, f.l, f.bobC); len(got) != 1 || got[0] != "assigned "+mine.Ref {
		t.Fatalf("muted comment reached bob: %v", got)
	}
	if after, _ := countRows(t, f.l, f.scope); after != events+1 {
		t.Fatalf("a change notifying nobody wrote an inbox event: %d -> %d", events, after)
	}
}

func TestEditingNotifiesOnlyNewMentions(t *testing.T) {
	f := newInboxFixture(t)
	task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", Body: "@bob look"})
	c, err := f.l.AddComment(f.adaC, core.TaskRef{ID: task.ID}, "@carol hi")
	if err != nil {
		t.Fatal(err)
	}
	body := "@bob look, typo fixed"
	if _, err := f.l.UpdateTask(f.adaC, core.TaskRef{ID: task.ID}, core.UpdateTaskInput{Body: &body}); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.EditComment(f.adaC, c.ID, "@carol hi, and @bob"); err != nil {
		t.Fatal(err)
	}
	if got := inboxOf(t, f.l, f.bobC); len(got) != 2 {
		t.Fatalf("bob = %v, want the description mention and the comment edit's mention", got)
	}
	if got := inboxOf(t, f.l, f.carolC); len(got) != 1 {
		t.Fatalf("carol was mentioned again by an edit that kept her: %v", got)
	}
}

func TestReadStateIsOwnAndRecordedOnce(t *testing.T) {
	f := newInboxFixture(t)
	for range 3 {
		mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
	}
	items, _, err := f.l.ListInbox(f.bobC, core.InboxFilter{})
	if err != nil || len(items) != 3 {
		t.Fatalf("items = %d, err = %v", len(items), err)
	}
	if _, err := f.l.ReadInboxItem(f.adaC, items[0].ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("reading somebody else's item: err = %v, want not found", err)
	}

	steps := []struct {
		name  string
		call  func() error
		write bool
		count int
	}{
		{"read", func() error { _, err := f.l.ReadInboxItem(f.bobC, items[0].ID); return err }, true, 2},
		{"read again", func() error { _, err := f.l.ReadInboxItem(f.bobC, items[0].ID); return err }, false, 2},
		{"unread", func() error { _, err := f.l.UnreadInboxItem(f.bobC, items[0].ID); return err }, true, 3},
		{"read all", func() error { _, err := f.l.ReadAllInbox(f.bobC); return err }, true, 0},
		{"read all again", func() error { _, err := f.l.ReadAllInbox(f.bobC); return err }, false, 0},
		{"mute", func() error {
			_, err := f.l.PutInboxPrefs(f.bobC, core.InboxPrefsInput{Muted: []string{"status"}})
			return err
		}, true, 0},
		{"mute the same", func() error {
			_, err := f.l.PutInboxPrefs(f.bobC, core.InboxPrefsInput{Muted: []string{"STATUS"}})
			return err
		}, false, 0},
	}
	for _, s := range steps {
		events, audits := countRows(t, f.l, f.scope)
		if err := s.call(); err != nil {
			t.Fatalf("%s: %v", s.name, err)
		}
		e2, a2 := countRows(t, f.l, f.scope)
		wrote := e2 == events+1 && a2 == audits+1
		silent := e2 == events && a2 == audits
		if s.write && !wrote || !s.write && !silent {
			t.Fatalf("%s: events %d -> %d, audits %d -> %d, want a write = %v", s.name, events, e2, audits, a2, s.write)
		}
		n, err := f.l.CountInbox(f.bobC)
		if err != nil || n.Unread != s.count {
			t.Fatalf("%s: unread = %v, err = %v, want %d", s.name, n, err, s.count)
		}
	}
	prefs, err := f.l.GetInboxPrefs(f.bobC)
	if err != nil || !slices.Equal(prefs.Muted, []core.InboxKind{core.InboxStatus}) {
		t.Fatalf("prefs = %v, err = %v", prefs, err)
	}
	if _, err := f.l.PutInboxPrefs(f.bobC, core.InboxPrefsInput{Muted: []string{"everything"}}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unknown kind: err = %v, want invalid", err)
	}
}

func TestInboxPagesNewestFirstWithoutRepeats(t *testing.T) {
	f := newInboxFixture(t)
	var want []string
	for range 5 {
		task := mustCreateTask(t, f.l, f.adaC, core.CreateTaskInput{ProjectRef: "infra", Title: "t", AssigneeActorID: "bob"})
		want = append([]string{task.Ref}, want...)
	}
	if _, err := f.l.ReadAllInbox(f.bobC); err != nil {
		t.Fatal(err)
	}
	if _, err := f.l.UnreadInboxItem(f.bobC, firstItem(t, f.l, f.bobC).ID); err != nil {
		t.Fatal(err)
	}
	var got []string
	cursor := ""
	for range 10 {
		items, next, err := f.l.ListInbox(f.bobC, core.InboxFilter{Page: core.Page{Limit: 2, Cursor: cursor}})
		if err != nil {
			t.Fatal(err)
		}
		for _, it := range items {
			got = append(got, it.TaskRef)
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if !slices.Equal(got, want) {
		t.Fatalf("paged = %v, want %v", got, want)
	}
	asc, _, err := f.l.ListInbox(f.bobC, core.InboxFilter{Page: core.Page{Direction: core.Ascending}})
	if err != nil || asc[0].TaskRef != want[0] {
		t.Fatalf("a caller asking for ascending got %v first, want the newest %s", asc[0].TaskRef, want[0])
	}
	unread, _, err := f.l.ListInbox(f.bobC, core.InboxFilter{Unread: true})
	if err != nil || len(unread) != 1 || unread[0].TaskRef != want[0] {
		t.Fatalf("unread only = %v, err = %v", unread, err)
	}
}

func TestInboxIsRefusedToAPinnedToken(t *testing.T) {
	f := newInboxFixture(t)
	pinned := *f.bob
	pinned.ProjectID = "anything"
	pinned.TokenID = "tok"
	if _, _, err := f.l.ListInbox(taskContext(&pinned), core.InboxFilter{}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("pinned token listing an inbox: err = %v, want forbidden", err)
	}
}

func firstItem(t *testing.T, l *Local, ctx context.Context) core.InboxItem {
	t.Helper()
	items, _, err := l.ListInbox(ctx, core.InboxFilter{})
	if err != nil || len(items) == 0 {
		t.Fatalf("no items: %v", err)
	}
	return items[0]
}

func anyStrings(v any) []string {
	switch t := v.(type) {
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, x := range t {
			s, _ := x.(string)
			out = append(out, s)
		}
		return out
	}
	return nil
}
