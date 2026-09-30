// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func seedAuditSubjects(t *testing.T, l *Local, scope core.TenantScope, n int) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		for i := 0; i < n; i++ {
			e := core.AuditEntry{
				Action:      "task.update",
				SubjectType: "task",
				SubjectID:   fmt.Sprintf("t%02d", i),
				Source:      core.SourceCLI,
			}
			if err := tx.AppendAudit(ctx, &e); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding audit entries: %v", err)
	}
}

// Keyset paging must walk the whole log without skipping or repeating a row.
func TestListAuditPagesByCursorExactlyOnce(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seedAuditSubjects(t, l, scope, 10)

	for _, sort := range []string{"seq", "occurred_at"} {
		t.Run(sort, func(t *testing.T) {
			seen := map[int64]int{}
			cursor := ""
			for pages := 0; ; pages++ {
				if pages > 10 {
					t.Fatal("paging did not terminate")
				}
				entries, next, err := l.ListAudit(ctx, core.AuditFilter{
					Page: core.Page{Limit: 3, Cursor: cursor, Sort: sort},
				})
				if err != nil {
					t.Fatalf("listing audit: %v", err)
				}
				for _, e := range entries {
					seen[e.Seq]++
				}
				if next == "" {
					break
				}
				cursor = next
			}
			if len(seen) != 10 {
				t.Fatalf("paged over %d distinct entries, want 10", len(seen))
			}
			for seq, n := range seen {
				if n != 1 {
					t.Errorf("entry %d returned %d times", seq, n)
				}
			}
		})
	}
}

// seedAuditWords writes n entries whose after image carries word for the
// first matching of them and something else for the rest, newest last.
func seedAuditWords(t *testing.T, l *Local, scope core.TenantScope, n, matching int, word string) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		for i := range n {
			title := "other"
			if i < matching {
				title = word
			}
			e := core.AuditEntry{
				Action:      "task.create",
				SubjectType: "task",
				SubjectID:   fmt.Sprintf("t%03d", i),
				Source:      core.SourceWeb,
				After:       []byte(fmt.Sprintf(`{"title":%q}`, title)),
			}
			if err := tx.AppendAudit(ctx, &e); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding audit entries: %v", err)
	}
}

// A free-text filter the store answers is a filter whose page fits exactly,
// and that is the whole point of it living here.
//
// It used to be applied by the caller, over pages the store had already
// returned. A caller cannot report a cursor for a page it assembled itself:
// the only cursor it holds names the end of the last store page it read, so
// every match it trimmed off the end of a screenful was stepped over by the
// next request rather than shown on it. Walking the listing to its end is the
// property that catches that, because a skipped entry appears on no page.
func TestAuditTextFilterWalksEveryMatch(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	const word = "zebra"
	seedAuditWords(t, l, scope, 120, 55, word)

	seen := map[string]int{}
	cursor := ""
	for pages := 0; ; pages++ {
		if pages > 20 {
			t.Fatal("paging did not terminate")
		}
		entries, next, err := l.ListAudit(ctx, core.AuditFilter{
			Text: word, Page: core.Page{Limit: 10, Cursor: cursor, Sort: "seq"},
		})
		if err != nil {
			t.Fatalf("listing audit: %v", err)
		}
		for _, e := range entries {
			if !strings.Contains(string(e.After), word) {
				t.Errorf("entry %s does not match %q but was returned", e.SubjectID, word)
			}
			seen[e.SubjectID]++
		}
		if next == "" {
			break
		}
		cursor = next
	}
	if len(seen) != 55 {
		t.Errorf("walking the filtered listing to its end found %d matches, want 55", len(seen))
	}
	for id, n := range seen {
		if n != 1 {
			t.Errorf("entry %s was returned %d times", id, n)
		}
	}
}

// The box searches what an entry records, not one column of it. A term that
// occurs only in the action, or only in the snapshot, has to find its entry
// either way, because that is what the screen offering the box promises.
func TestAuditTextFilterReadsActionAndSnapshot(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seedAuditWords(t, l, scope, 4, 2, "zebra")

	for _, tc := range []struct {
		name, text string
		want       int
	}{
		{"snapshot", "zebra", 2},
		{"action", "task.create", 4},
		{"source", "web", 4},
		{"no match", "narwhal", 0},
		{"wildcards are literal", "%", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			entries, _, err := l.ListAudit(ctx, core.AuditFilter{
				Text: tc.text, Page: core.Page{Limit: 50},
			})
			if err != nil {
				t.Fatalf("listing audit: %v", err)
			}
			if len(entries) != tc.want {
				t.Errorf("searching for %q returned %d entries, want %d", tc.text, len(entries), tc.want)
			}
		})
	}
}

func TestListAuditFiltersBySubject(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seedAuditSubjects(t, l, scope, 5)

	entries, _, err := l.ListAudit(ctx, core.AuditFilter{SubjectType: "task", SubjectID: "t03"})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	if len(entries) != 1 || entries[0].SubjectID != "t03" {
		t.Fatalf("filtered listing = %+v, want the single t03 entry", entries)
	}
}

func TestListAuditEmptyResultIsNotAnError(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	entries, next, err := l.ListAudit(ctx, core.AuditFilter{SubjectID: "nothing"})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	if len(entries) != 0 || next != "" {
		t.Errorf("empty listing = %d entries, cursor %q", len(entries), next)
	}
}

// Reading the audit log needs its own scope; being able to read the records it
// describes is not enough.
func TestListAuditDeniedWithoutAuditScope(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	reader := &core.Actor{ID: "r1", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	ctx := core.WithActor(context.Background(), reader)

	entries, _, err := l.ListAudit(ctx, core.AuditFilter{})
	if !core.IsKind(err, core.KindForbidden) {
		t.Errorf("audit listing without the scope = %v, want forbidden", err)
	}
	if len(entries) != 0 {
		t.Errorf("a refused listing returned %d entries", len(entries))
	}
}

func TestListAuditRejectsBadPaging(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seedAuditSubjects(t, l, scope, 4)

	if _, _, err := l.ListAudit(ctx, core.AuditFilter{Page: core.Page{Sort: "banana"}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("listing sorted by an unknown column = %v, want invalid", err)
	}
	if _, _, err := l.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: -1}}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("listing with a negative limit = %v, want invalid", err)
	}

	_, next, err := l.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 2}})
	if err != nil || next == "" {
		t.Fatalf("first page = %v, cursor %q", err, next)
	}
	_, _, err = l.ListAudit(ctx, core.AuditFilter{
		Page: core.Page{Limit: 2, Cursor: next, Sort: "occurred_at"},
	})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("reusing a cursor under another ordering = %v, want invalid", err)
	}
}

func TestSubscribeDeliversCommittedEvents(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx, cancel := context.WithCancel(core.WithActor(context.Background(), actor))
	defer cancel()

	stream, err := l.Subscribe(ctx, core.EventFilter{Types: []core.EventType{"task.*"}})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	seedEvents(t, l, scope, 1, clk.Now())

	select {
	case e, ok := <-stream:
		if !ok {
			t.Fatal("stream closed before delivering an event")
		}
		if e.Type != core.EventTaskCreated {
			t.Errorf("delivered %q, want %q", e.Type, core.EventTaskCreated)
		}
		// The cursor advances after the event is handed over, so that it always
		// reflects what the subscriber has actually taken. Checking it the
		// instant the receive returns races that update.
		waitForCursor(t, scope.TenantID, e.Seq)
	case <-time.After(10 * time.Second):
		t.Fatal("no event delivered")
	}
}

func TestSubscribeRejectsBadCursorAndMissingScope(t *testing.T) {
	l, _, scope, actor := newLocal(t)

	ctx := core.WithActor(context.Background(), actor)
	if _, err := l.Subscribe(ctx, core.EventFilter{SinceSeq: -1}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("subscribe with a negative cursor = %v, want invalid", err)
	}

	mute := &core.Actor{ID: "m1", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead}}
	if _, err := l.Subscribe(core.WithActor(context.Background(), mute), core.EventFilter{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("subscribe without the scope = %v, want forbidden", err)
	}
}

func TestGetRetentionReturnsDefaultsFavouringAudit(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	p, err := l.GetRetention(ctx)
	if err != nil {
		t.Fatalf("get retention: %v", err)
	}
	def := core.DefaultRetention(scope.TenantID)
	if *p != def {
		t.Errorf("default policy = %+v, want %+v", *p, def)
	}
	if p.AuditEntries <= p.Events {
		t.Error("the default audit window must be substantially longer than the event window")
	}
	if p.WebhookDeliveries > p.AuditEntries {
		t.Error("the default delivery window must not outlive the audit window")
	}
}

func TestPutRetentionChangesOneWindowOnly(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	before, err := l.GetRetention(ctx)
	if err != nil {
		t.Fatalf("get retention: %v", err)
	}
	got, err := l.PutRetention(ctx, core.RetentionPolicy{Events: core.Duration(time.Hour)})
	if err != nil {
		t.Fatalf("put retention: %v", err)
	}
	if got.Events != core.Duration(time.Hour) {
		t.Errorf("event window = %s, want 1h", got.Events)
	}
	if got.AuditEntries != before.AuditEntries || got.WebhookDeliveries != before.WebhookDeliveries {
		t.Errorf("changing the event window disturbed the others: %+v", *got)
	}

	stored, err := l.GetRetention(ctx)
	if err != nil {
		t.Fatalf("re-reading retention: %v", err)
	}
	if *stored != *got {
		t.Errorf("stored policy = %+v, want %+v", *stored, *got)
	}
}

func TestPutRetentionIsAudited(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.PutRetention(ctx, core.RetentionPolicy{Events: core.Duration(2 * time.Hour)}); err != nil {
		t.Fatalf("put retention: %v", err)
	}
	entries, _, err := l.ListAudit(ctx, core.AuditFilter{Actions: []string{"retention.update"}})
	if err != nil {
		t.Fatalf("listing audit: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("found %d retention audit entries, want 1", len(entries))
	}
	if len(entries[0].Before) == 0 || len(entries[0].After) == 0 {
		t.Error("the retention audit entry must record the previous and the new windows")
	}
}

func TestPutRetentionRejectsNegativeWindowAndKeepsPolicy(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	before, err := l.GetRetention(ctx)
	if err != nil {
		t.Fatalf("get retention: %v", err)
	}
	if _, err := l.PutRetention(ctx, core.RetentionPolicy{Events: core.Duration(-time.Hour)}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("negative window = %v, want invalid", err)
	}
	after, err := l.GetRetention(ctx)
	if err != nil {
		t.Fatalf("get retention: %v", err)
	}
	if *after != *before {
		t.Errorf("a rejected change altered the policy: %+v", *after)
	}
}

func TestRetentionRequiresAdminScope(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	member := &core.Actor{ID: "m1", TenantID: scope.TenantID, Kind: core.ActorUser, Role: core.RoleMember}
	ctx := core.WithActor(context.Background(), member)

	if _, err := l.GetRetention(ctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("reading retention without admin = %v, want forbidden", err)
	}
	if _, err := l.PutRetention(ctx, core.RetentionPolicy{Events: core.Duration(time.Hour)}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("writing retention without admin = %v, want forbidden", err)
	}
}

// waitForCursor blocks until the tenant's subscriber floor reaches want.
func waitForCursor(t *testing.T, tenantID string, want int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if floor, live := subscribers.floor(tenantID); live && floor == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	floor, live := subscribers.floor(tenantID)
	t.Fatalf("subscriber cursor = %d (live %v), want %d", floor, live, want)
}

// A cursor the retention sweep has overtaken cannot be resumed from: the
// events between it and the oldest surviving row are gone. Handing the
// subscriber the oldest survivor instead would look, from the outside, exactly
// like being caught up, which is the one failure a resume cursor exists to
// prevent. The served path has always refused this; the direct path did not,
// so the same `tix watch --since` was gap-free against a server and silently
// lossy against a database file.
func TestSubscribeRefusesACursorRetentionHasAlreadyRemoved(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	seedEvents(t, l, scope, 3, clk.Now())
	clk.Advance(60 * 24 * time.Hour)
	if _, err := l.Prune(ctx, core.PruneInput{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	seedEvents(t, l, scope, 2, clk.Now())

	_, err := l.Subscribe(ctx, core.EventFilter{SinceSeq: 1})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("subscribing from a pruned cursor = %v, want invalid: a resume that cannot be honoured "+
			"must be refused, not answered with the oldest surviving event", err)
	}
	if !strings.Contains(err.Error(), "no longer available") {
		t.Errorf("error %q does not tell the subscriber its cursor is gone", err)
	}
}

// A cursor still inside the retained window is honoured, so the check above
// refuses only what it must.
func TestSubscribeAcceptsARetainedCursor(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx, cancel := context.WithCancel(core.WithActor(context.Background(), actor))
	defer cancel()

	seedEvents(t, l, scope, 3, clk.Now())
	oldest := oldestEventSeq(t, l, scope)

	if _, err := l.Subscribe(ctx, core.EventFilter{SinceSeq: oldest}); err != nil {
		t.Fatalf("subscribing from the oldest retained event = %v, want it accepted", err)
	}
}

// oldestEventSeq reports the lowest sequence number still stored.
func oldestEventSeq(t *testing.T, l *Local, scope core.TenantScope) int64 {
	t.Helper()
	var seq int64
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		events, err := tx.ReadEvents(context.Background(), 0, 1)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			t.Fatal("no events were seeded")
		}
		seq = events[0].Seq
		return nil
	}); err != nil {
		t.Fatalf("reading the oldest event: %v", err)
	}
	return seq
}
