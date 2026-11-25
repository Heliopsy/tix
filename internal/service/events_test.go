// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// pollEvent is one event a poll test appends.
type pollEvent struct {
	typ     core.EventType
	subject string
	project string
}

// appendPollEvents commits events in one transaction and returns their sequence numbers.
func appendPollEvents(t *testing.T, l *Local, scope core.TenantScope, events ...pollEvent) []int64 {
	t.Helper()
	ctx := context.Background()
	seqs := make([]int64, 0, len(events))
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		for _, pe := range events {
			subject := pe.subject
			if subject == "" {
				subject = "task"
			}
			e := core.Event{Type: pe.typ, SubjectType: subject, SubjectID: "s1", ProjectID: pe.project}
			if err := tx.AppendEvent(ctx, &e); err != nil {
				return err
			}
			seqs = append(seqs, e.Seq)
		}
		return nil
	}); err != nil {
		t.Fatalf("appending events: %v", err)
	}
	return seqs
}

func pollSeqs(page *core.EventPage) []int64 {
	out := make([]int64, 0, len(page.Events))
	for _, e := range page.Events {
		out = append(out, e.Seq)
	}
	return out
}

func sameSeqs(a, b []int64) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPollEventsAnswersWhatTheCursorAndFilterSelect(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	base := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated, project: "pC"})[0]
	seqs := appendPollEvents(t, l, scope,
		pollEvent{typ: core.EventTaskCreated, project: "pA"},
		pollEvent{typ: core.EventCommentAdded, project: "pA"},
		pollEvent{typ: core.EventTaskUpdated, project: "pB"},
		pollEvent{typ: "user.created", subject: "user"},
		pollEvent{typ: core.EventTaskCreated, project: "pB"},
	)
	agent := &core.Actor{ID: "agent", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeEventSubscribe}}
	scoped := &core.Actor{ID: "scoped", TenantID: scope.TenantID, Kind: core.ActorAgent, ProjectID: "pB",
		Scopes: []core.Scope{core.ScopeEventSubscribe}}
	last := seqs[len(seqs)-1]

	tests := []struct {
		name     string
		actor    *core.Actor
		in       core.EventPollInput
		want     []int64
		wantNext int64
	}{
		{"exclusive cursor", admin, core.EventPollInput{SinceSeq: seqs[1]}, seqs[2:], last},
		{"nothing after the cursor", admin, core.EventPollInput{SinceSeq: last}, nil, last},
		{"no cursor starts at the newest", admin, core.EventPollInput{}, nil, last},
		{"type filter", admin, core.EventPollInput{SinceSeq: base, Types: []core.EventType{"task.*"}},
			[]int64{seqs[0], seqs[2], seqs[4]}, last},
		{"project filter", admin, core.EventPollInput{SinceSeq: base, ProjectIDs: []string{"pA"}},
			seqs[:2], last},
		{"limit stops at the last event delivered", admin, core.EventPollInput{SinceSeq: base, Limit: 2},
			seqs[:2], seqs[1]},
		{"cursor passes events the filter leaves out", admin,
			core.EventPollInput{SinceSeq: base, Types: []core.EventType{"comment.added"}},
			[]int64{seqs[1]}, last},
		{"administrative event needs its scope", agent, core.EventPollInput{SinceSeq: seqs[2]},
			[]int64{seqs[4]}, last},
		{"wildcard credential sees administrative events", admin, core.EventPollInput{SinceSeq: seqs[2]},
			seqs[3:], last},
		{"project-scoped token is narrowed", scoped, core.EventPollInput{SinceSeq: base},
			[]int64{seqs[2], seqs[4]}, last},
		{"project-scoped token naming its project", scoped,
			core.EventPollInput{SinceSeq: base, ProjectIDs: []string{"pB"}}, []int64{seqs[2], seqs[4]}, last},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			page, err := l.PollEvents(core.WithActor(context.Background(), tt.actor), tt.in)
			if err != nil {
				t.Fatalf("PollEvents: %v", err)
			}
			if got := pollSeqs(page); !sameSeqs(got, tt.want) {
				t.Errorf("events = %v, want %v", got, tt.want)
			}
			if page.NextSeq != tt.wantNext {
				t.Errorf("next_seq = %d, want %d", page.NextSeq, tt.wantNext)
			}
			if page.Events == nil {
				t.Error("events is nil; it must encode as [] rather than null")
			}
		})
	}
}

func TestPollEventsRefusesWhatTheWebSocketRefuses(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	seqs := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated}, pollEvent{typ: core.EventTaskCreated})
	mute := &core.Actor{ID: "mute", TenantID: scope.TenantID, Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead}}
	scoped := &core.Actor{ID: "scoped", TenantID: scope.TenantID, Kind: core.ActorAgent, ProjectID: "pB",
		Scopes: []core.Scope{core.ScopeEventSubscribe}}

	tests := []struct {
		name  string
		actor *core.Actor
		in    core.EventPollInput
		kind  core.Kind
	}{
		{"missing scope", mute, core.EventPollInput{}, core.KindForbidden},
		{"another project", scoped, core.EventPollInput{ProjectIDs: []string{"pA"}}, core.KindForbidden},
		{"unknown type", admin, core.EventPollInput{Types: []core.EventType{"task.invented"}}, core.KindInvalid},
		{"negative cursor", admin, core.EventPollInput{SinceSeq: -1}, core.KindInvalid},
		{"limit over the maximum", admin, core.EventPollInput{Limit: MaxPollLimit + 1}, core.KindInvalid},
		{"negative limit", admin, core.EventPollInput{Limit: -1}, core.KindInvalid},
		{"wait over the cap", admin, core.EventPollInput{Wait: core.Duration(MaxPollWait + time.Second)}, core.KindInvalid},
		{"negative wait", admin, core.EventPollInput{Wait: core.Duration(-time.Second)}, core.KindInvalid},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := l.PollEvents(core.WithActor(context.Background(), tt.actor), tt.in)
			if !core.IsKind(err, tt.kind) {
				t.Fatalf("PollEvents = %v, want %s", err, tt.kind)
			}
		})
	}
	if _, err := l.PollEvents(core.WithActor(context.Background(), admin),
		core.EventPollInput{SinceSeq: seqs[0], Limit: MaxPollLimit, Wait: core.Duration(MaxPollWait)}); err != nil {
		t.Fatalf("the largest limit and wait were refused: %v", err)
	}
}

func TestPollEventsReportsAnExpiredCursorLikeTheWebSocket(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	seedEvents(t, l, scope, 3, clk.Now())
	clk.Advance(60 * 24 * time.Hour)
	if _, err := l.Prune(ctx, core.PruneInput{}); err != nil {
		t.Fatalf("prune: %v", err)
	}
	seedEvents(t, l, scope, 2, clk.Now())
	oldest := oldestEventSeq(t, l, scope)

	_, err := l.PollEvents(ctx, core.EventPollInput{SinceSeq: 1})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("polling from a pruned cursor = %v, want invalid", err)
	}
	want := CheckCursorRetained(1, oldest).Error()
	if err.Error() != want || !strings.Contains(want, "no longer available") {
		t.Errorf("error = %q, want the WebSocket's %q", err, want)
	}
	if _, err := l.PollEvents(ctx, core.EventPollInput{SinceSeq: oldest - 1}); err != nil {
		t.Errorf("polling from just before the oldest retained event = %v, want it accepted", err)
	}
}

// pollAsync runs a poll in the background and waits until it is blocked on
// the clock, so the caller can move time or commit knowing the poll is waiting.
func pollAsync(t *testing.T, l *Local, clk *clock.Fake, actor *core.Actor, in core.EventPollInput) <-chan *core.EventPage {
	t.Helper()
	out := make(chan *core.EventPage, 1)
	errs := make(chan error, 1)
	go func() {
		page, err := l.PollEvents(core.WithActor(context.Background(), actor), in)
		if err != nil {
			errs <- err
			return
		}
		out <- page
	}()
	deadline := time.Now().Add(10 * time.Second)
	for clk.Waiters() == 0 {
		select {
		case err := <-errs:
			t.Fatalf("PollEvents: %v", err)
		case page := <-out:
			t.Fatalf("the poll answered %v without waiting", pollSeqs(page))
		default:
		}
		if time.Now().After(deadline) {
			t.Fatal("the poll never started waiting")
		}
		<-time.After(time.Millisecond)
	}
	return out
}

func receivePage(t *testing.T, pages <-chan *core.EventPage) *core.EventPage {
	t.Helper()
	select {
	case page := <-pages:
		return page
	case <-time.After(10 * time.Second):
		t.Fatal("the poll did not answer")
		return nil
	}
}

func TestPollEventsWaitReturnsOnTheNextMatchingCommit(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	start := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated})[0]

	pages := pollAsync(t, l, clk, actor, core.EventPollInput{
		SinceSeq: start, Types: []core.EventType{"comment.added"}, Wait: core.Duration(MaxPollWait),
	})
	skipped := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskUpdated})[0]
	select {
	case page := <-pages:
		t.Fatalf("a commit the filter leaves out ended the wait with %v", pollSeqs(page))
	case <-time.After(50 * time.Millisecond):
	}
	want := appendPollEvents(t, l, scope, pollEvent{typ: core.EventCommentAdded})[0]

	page := receivePage(t, pages)
	if got := pollSeqs(page); !sameSeqs(got, []int64{want}) {
		t.Fatalf("events = %v, want [%d] (skipped %d)", got, want, skipped)
	}
	if page.NextSeq != want {
		t.Errorf("next_seq = %d, want %d", page.NextSeq, want)
	}
}

func TestPollEventsWaitTimesOutEmpty(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	start := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated})[0]

	pages := pollAsync(t, l, clk, actor, core.EventPollInput{SinceSeq: start, Wait: core.Duration(MaxPollWait)})
	clk.Advance(MaxPollWait - time.Second)
	select {
	case page := <-pages:
		t.Fatalf("the poll answered %v before its wait ran out", pollSeqs(page))
	case <-time.After(50 * time.Millisecond):
	}
	clk.Advance(time.Second)

	page := receivePage(t, pages)
	if len(page.Events) != 0 || page.NextSeq != start {
		t.Fatalf("timed out poll = %v next %d, want no events and next %d", pollSeqs(page), page.NextSeq, start)
	}
}

func TestPollEventsWithoutWaitAnswersAtOnce(t *testing.T) {
	l, clk, scope, actor := newLocal(t)
	start := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated})[0]
	page, err := l.PollEvents(core.WithActor(context.Background(), actor), core.EventPollInput{SinceSeq: start})
	if err != nil {
		t.Fatalf("PollEvents: %v", err)
	}
	if len(page.Events) != 0 || page.NextSeq != start || clk.Waiters() != 0 {
		t.Fatalf("poll without wait = %v next %d with %d waiters", pollSeqs(page), page.NextSeq, clk.Waiters())
	}
}

// A poll that stops at its read budget has not caught up, so it answers
// rather than waiting on events that are already committed.
func TestPollEventsStopsAtItsReadBudgetWithoutWaiting(t *testing.T) {
	budget, batch := pollScanBudget, pollBatch
	pollScanBudget, pollBatch = 10, 5
	t.Cleanup(func() { pollScanBudget, pollBatch = budget, batch })
	l, clk, scope, actor := newLocal(t)
	seeded := seedEvents(t, l, scope, pollScanBudget+10, clk.Now())

	page, err := l.PollEvents(core.WithActor(context.Background(), actor), core.EventPollInput{
		SinceSeq: seeded[0].Seq, Types: []core.EventType{"comment.added"}, Wait: core.Duration(MaxPollWait),
	})
	if err != nil {
		t.Fatalf("PollEvents: %v", err)
	}
	if want := seeded[pollScanBudget].Seq; len(page.Events) != 0 || page.NextSeq != want {
		t.Fatalf("budgeted poll = %v next %d, want no events and next %d", pollSeqs(page), page.NextSeq, want)
	}
}

// A client that polls and then subscribes with the returned cursor receives
// every later event exactly once.
func TestPollThenSubscribeLeavesNoGap(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx, cancel := context.WithCancel(core.WithActor(context.Background(), actor))
	defer cancel()
	first := appendPollEvents(t, l, scope,
		pollEvent{typ: core.EventTaskCreated}, pollEvent{typ: core.EventTaskCreated}, pollEvent{typ: core.EventTaskUpdated})

	page, err := l.PollEvents(ctx, core.EventPollInput{SinceSeq: first[0], Limit: 1})
	if err != nil {
		t.Fatalf("PollEvents: %v", err)
	}
	later := appendPollEvents(t, l, scope, pollEvent{typ: core.EventCommentAdded})

	stream, err := l.Subscribe(ctx, core.EventFilter{SinceSeq: page.NextSeq})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	if page.NextSeq != first[1] {
		t.Fatalf("next_seq = %d, want %d", page.NextSeq, first[1])
	}
	want := append([]int64{first[2]}, later...)
	for _, seq := range want {
		select {
		case e := <-stream:
			if e.Seq != seq {
				t.Fatalf("subscription delivered %d, want %d after polling up to %d", e.Seq, seq, page.NextSeq)
			}
		case <-time.After(10 * time.Second):
			t.Fatalf("event %d never arrived", seq)
		}
	}
}

// event:subscribe is confinable, so the direct subscription narrows a pinned
// token exactly as the poll and the WebSocket do.
func TestSubscribeNarrowsAPinnedToken(t *testing.T) {
	l, _, scope, _ := newLocal(t)
	pinned := &core.Actor{ID: "pinned", TenantID: scope.TenantID, Kind: core.ActorAgent, ProjectID: "pB",
		Scopes: []core.Scope{core.ScopeEventSubscribe}}
	ctx, cancel := context.WithCancel(core.WithActor(context.Background(), pinned))
	defer cancel()
	start := appendPollEvents(t, l, scope, pollEvent{typ: core.EventTaskCreated, project: "pB"})[0]

	if _, err := l.Subscribe(ctx, core.EventFilter{ProjectIDs: []string{"pA"}}); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a pinned token subscribing to another project = %v, want forbidden", err)
	}
	stream, err := l.Subscribe(ctx, core.EventFilter{SinceSeq: start})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	seqs := appendPollEvents(t, l, scope,
		pollEvent{typ: core.EventTaskCreated, project: "pA"}, pollEvent{typ: core.EventTaskCreated, project: "pB"})
	select {
	case e := <-stream:
		if e.Seq != seqs[1] {
			t.Fatalf("pinned subscription delivered %d (project %q), want %d", e.Seq, e.ProjectID, seqs[1])
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the pinned subscription delivered nothing")
	}
}
