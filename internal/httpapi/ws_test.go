// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// fakeLog is an in-memory durable event log with a configurable pruned floor.
type fakeLog struct {
	mu      sync.Mutex
	events  []core.Event
	oldest  int64
	err     error
	readErr error
}

func (l *fakeLog) add(events ...core.Event) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, events...)
}

func (l *fakeLog) ReadSince(_ context.Context, tenantID string, sinceSeq int64, limit int) ([]core.Event, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.readErr != nil {
		return nil, l.readErr
	}
	var out []core.Event
	for _, e := range l.events {
		if e.TenantID != tenantID || e.Seq <= sinceSeq {
			continue
		}
		out = append(out, e)
		if len(out) == limit {
			break
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Seq < out[j].Seq })
	return out, nil
}

func (l *fakeLog) OldestSeq(_ context.Context, tenantID string) (int64, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.err != nil {
		return 0, l.err
	}
	if l.oldest > 0 {
		return l.oldest, nil
	}
	var lowest int64
	for _, e := range l.events {
		if e.TenantID != tenantID {
			continue
		}
		if lowest == 0 || e.Seq < lowest {
			lowest = e.Seq
		}
	}
	return lowest, nil
}

type wsFixture struct {
	srv    *httptest.Server
	hub    *Hub
	log    *fakeLog
	stream *EventStream
}

func newFixture(t *testing.T, resolve func(*http.Request) *core.Actor) *wsFixture {
	t.Helper()
	log := &fakeLog{}
	hub := NewHub()
	stream := NewEventStream(hub, log)
	stream.pingInterval = 25 * time.Millisecond

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor := resolve(r)
		if actor != nil {
			r = r.WithContext(core.WithActor(r.Context(), actor))
		}
		stream.ServeHTTP(w, r)
	}))
	t.Cleanup(srv.Close)
	return &wsFixture{srv: srv, hub: hub, log: log, stream: stream}
}

func fixedActor(a *core.Actor) func(*http.Request) *core.Actor {
	return func(*http.Request) *core.Actor { return a }
}

func (f *wsFixture) dial(t *testing.T, ctx context.Context) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + wire.RouteEvents
	c, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = c.CloseNow() })
	return c
}

func send(t *testing.T, ctx context.Context, c *websocket.Conn, m ClientMessage) {
	t.Helper()
	b, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := c.Write(ctx, websocket.MessageText, b); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func sendRaw(t *testing.T, ctx context.Context, c *websocket.Conn, payload string) {
	t.Helper()
	if err := c.Write(ctx, websocket.MessageText, []byte(payload)); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func read(t *testing.T, ctx context.Context, c *websocket.Conn) ServerMessage {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, b, err := c.Read(ctx)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var m ServerMessage
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", b, err)
	}
	return m
}

func subscribeOK(t *testing.T, ctx context.Context, c *websocket.Conn, id string, f core.EventFilter) {
	t.Helper()
	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: id, Filter: &f})
	if m := read(t, ctx, c); m.Type != MsgSubscribed || m.ID != id {
		t.Fatalf("subscribe ack = %+v, want subscribed for %q", m, id)
	}
}

func waitUntil(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.After(3 * time.Second)
	tick := time.NewTicker(2 * time.Millisecond)
	defer tick.Stop()
	for {
		if cond() {
			return
		}
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for %s", what)
		case <-tick.C:
		}
	}
}

func TestSubscribeThenReceiveMatchingEvent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})

	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })
	f.hub.Broadcast(core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "task-1"})

	m := read(t, ctx, c)
	if m.Type != MsgEvent || m.ID != "s1" {
		t.Fatalf("got %+v, want an event on s1", m)
	}
	if m.Event == nil || m.Event.SubjectID != "task-1" || m.Event.Seq != 1 {
		t.Fatalf("event payload = %+v", m.Event)
	}
}

func TestFilteredEventsAreNotDelivered(t *testing.T) {
	tests := []struct {
		name    string
		filter  core.EventFilter
		skipped core.Event
		wanted  core.Event
	}{
		{
			name:    "project filter",
			filter:  core.EventFilter{ProjectIDs: []string{"proj-1"}},
			skipped: core.Event{Seq: 1, TenantID: "t1", ProjectID: "proj-2", Type: core.EventTaskCreated, SubjectID: "no"},
			wanted:  core.Event{Seq: 2, TenantID: "t1", ProjectID: "proj-1", Type: core.EventTaskCreated, SubjectID: "yes"},
		},
		{
			name:    "type filter",
			filter:  core.EventFilter{Types: []core.EventType{core.EventTaskTransitioned}},
			skipped: core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "no"},
			wanted:  core.Event{Seq: 2, TenantID: "t1", Type: core.EventTaskTransitioned, SubjectID: "yes"},
		},
		{
			name:    "wildcard prefix",
			filter:  core.EventFilter{Types: []core.EventType{"task.*"}},
			skipped: core.Event{Seq: 1, TenantID: "t1", Type: core.EventCommentAdded, SubjectID: "no"},
			wanted:  core.Event{Seq: 2, TenantID: "t1", Type: core.EventTaskClaimed, SubjectID: "yes"},
		},
		{
			name:    "match all wildcard",
			filter:  core.EventFilter{Types: []core.EventType{"*"}},
			skipped: core.Event{Seq: 1, TenantID: "other", Type: core.EventTaskCreated, SubjectID: "no"},
			wanted:  core.Event{Seq: 2, TenantID: "t1", Type: core.EventCommentAdded, SubjectID: "yes"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			f := newFixture(t, fixedActor(testActor("t1")))
			c := f.dial(t, ctx)
			subscribeOK(t, ctx, c, "s1", tc.filter)
			waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })

			f.hub.Broadcast(tc.skipped)
			f.hub.Broadcast(tc.wanted)

			m := read(t, ctx, c)
			if m.Event == nil || m.Event.SubjectID != "yes" {
				t.Fatalf("delivered %+v, want only the matching event", m.Event)
			}
		})
	}
}

func TestSinceSeqReplaysMissedEventsInOrder(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	for i := 1; i <= 5; i++ {
		f.log.add(core.Event{Seq: int64(i), TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "task", SubjectType: "task"})
	}
	f.log.add(core.Event{Seq: 6, TenantID: "other", Type: core.EventTaskCreated})

	c := f.dial(t, ctx)
	since := int64(2)
	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: "s1", SinceSeq: &since})
	if m := read(t, ctx, c); m.Type != MsgSubscribed {
		t.Fatalf("ack = %+v", m)
	}

	var got []int64
	for i := 0; i < 3; i++ {
		m := read(t, ctx, c)
		if m.Type != MsgEvent {
			t.Fatalf("replay message = %+v", m)
		}
		got = append(got, m.Event.Seq)
	}
	if len(got) != 3 || got[0] != 3 || got[1] != 4 || got[2] != 5 {
		t.Fatalf("replayed %v, want [3 4 5]", got)
	}

	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })
	f.hub.Broadcast(core.Event{Seq: 4, TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "dup"})
	f.hub.Broadcast(core.Event{Seq: 7, TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "live"})

	m := read(t, ctx, c)
	if m.Event == nil || m.Event.Seq != 7 {
		t.Fatalf("after replay got seq %+v, want the live event at 7 with no duplicate", m.Event)
	}
}

func TestSinceSeqBelowRetentionReportsUnavailableCursor(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	f.log.oldest = 50
	f.log.add(core.Event{Seq: 50, TenantID: "t1", Type: core.EventTaskCreated})

	c := f.dial(t, ctx)
	since := int64(3)
	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: "s1", SinceSeq: &since})

	m := read(t, ctx, c)
	if m.Type != MsgError || m.Error == nil {
		t.Fatalf("got %+v, want an error message", m)
	}
	if !strings.Contains(m.Error.Message, "no longer available") {
		t.Fatalf("error message = %q", m.Error.Message)
	}

	subscribeOK(t, ctx, c, "s2", core.EventFilter{})
}

func TestUnsubscribeStopsDelivery(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})
	subscribeOK(t, ctx, c, "s2", core.EventFilter{})
	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })

	send(t, ctx, c, ClientMessage{Type: MsgUnsubscribe, ID: "s1"})
	waitUntil(t, "unsubscribe", func() bool {
		for _, conn := range f.hub.snapshot() {
			return conn.subByID("s1") == nil
		}
		return false
	})

	f.hub.Broadcast(core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated})
	m := read(t, ctx, c)
	if m.ID != "s2" {
		t.Fatalf("delivered on %q, want only s2", m.ID)
	}

	send(t, ctx, c, ClientMessage{Type: MsgUnsubscribe, ID: "s1"})
	if m := read(t, ctx, c); m.Type != MsgError {
		t.Fatalf("unsubscribing twice = %+v, want an error", m)
	}
}

func TestPingIsAnsweredWithPong(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)

	send(t, ctx, c, ClientMessage{Type: MsgPing, ID: "p1"})
	m := read(t, ctx, c)
	if m.Type != MsgPong || m.ID != "p1" {
		t.Fatalf("got %+v, want pong p1", m)
	}
}

func TestBadRequestsKeepTheConnectionOpen(t *testing.T) {
	tests := []struct {
		name    string
		payload string
		want    core.Kind
	}{
		{name: "malformed json", payload: `{"type":`, want: core.KindInvalid},
		{name: "unknown message type", payload: `{"type":"explode"}`, want: core.KindInvalid},
		{name: "missing subscription id", payload: `{"type":"subscribe"}`, want: core.KindInvalid},
		{name: "negative since_seq", payload: `{"type":"subscribe","id":"s1","since_seq":-2}`, want: core.KindInvalid},
		{name: "unknown event type", payload: `{"type":"subscribe","id":"s1","filter":{"types":["task.exploded"]}}`, want: core.KindInvalid},
		{name: "unknown wildcard prefix", payload: `{"type":"subscribe","id":"s1","filter":{"types":["nope.*"]}}`, want: core.KindInvalid},
		{name: "duplicate subscription", payload: `{"type":"subscribe","id":"dup"}`, want: core.KindConflict},
	}

	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "dup", core.EventFilter{})

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			sendRaw(t, ctx, c, tc.payload)
			m := read(t, ctx, c)
			if m.Type != MsgError || m.Error == nil {
				t.Fatalf("got %+v, want an error message", m)
			}
			if m.Error.Code != tc.want {
				t.Fatalf("error code = %q, want %q", m.Error.Code, tc.want)
			}
		})
	}

	send(t, ctx, c, ClientMessage{Type: MsgPing, ID: "alive"})
	if m := read(t, ctx, c); m.Type != MsgPong {
		t.Fatalf("connection did not survive bad requests: %+v", m)
	}
}

func TestSubscriberNeverSeesAnotherTenantsEvent(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, func(r *http.Request) *core.Actor {
		return testActor(r.URL.Query().Get("tenant"))
	})

	url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + wire.RouteEvents
	a, aResp, err := websocket.Dial(ctx, url+"?tenant=t1", nil)
	if aResp != nil && aResp.Body != nil {
		_ = aResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial t1: %v", err)
	}
	defer func() { _ = a.CloseNow() }()
	b, bResp, err := websocket.Dial(ctx, url+"?tenant=t2", nil)
	if bResp != nil && bResp.Body != nil {
		_ = bResp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dial t2: %v", err)
	}
	defer func() { _ = b.CloseNow() }()

	subscribeOK(t, ctx, a, "s1", core.EventFilter{})
	subscribeOK(t, ctx, b, "s1", core.EventFilter{})
	waitUntil(t, "both registrations", func() bool { return f.hub.Len() == 2 })

	f.hub.Broadcast(core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "secret"})
	f.hub.Broadcast(core.Event{Seq: 2, TenantID: "t2", Type: core.EventTaskCreated, SubjectID: "mine"})

	m := read(t, ctx, b)
	if m.Event == nil || m.Event.SubjectID != "mine" || m.Event.TenantID != "t2" {
		t.Fatalf("tenant t2 received %+v", m.Event)
	}
	send(t, ctx, a, ClientMessage{Type: MsgPing, ID: "drain"})
	if m := read(t, ctx, a); m.Type != MsgEvent || m.Event.SubjectID != "secret" {
		t.Fatalf("tenant t1 received %+v", m)
	}
}

func TestProjectRestrictedCredentialOnlySeesItsProject(t *testing.T) {
	ctx := context.Background()
	actor := &core.Actor{ID: "p", TenantID: "t1", ProjectID: "proj-1", Kind: core.ActorAgent, Scopes: []core.Scope{core.ScopeEventSubscribe}}
	f := newFixture(t, fixedActor(actor))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})
	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })

	f.hub.Broadcast(core.Event{Seq: 1, TenantID: "t1", ProjectID: "proj-2", Type: core.EventTaskCreated, SubjectID: "no"})
	f.hub.Broadcast(core.Event{Seq: 2, TenantID: "t1", ProjectID: "proj-1", Type: core.EventTaskCreated, SubjectID: "yes"})

	m := read(t, ctx, c)
	if m.Event == nil || m.Event.SubjectID != "yes" {
		t.Fatalf("project restricted subscriber received %+v", m.Event)
	}

	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: "s2", Filter: &core.EventFilter{ProjectIDs: []string{"proj-9"}}})
	if m := read(t, ctx, c); m.Type != MsgError || m.Error.Code != core.KindForbidden {
		t.Fatalf("subscribing outside the permitted project = %+v", m)
	}
}

func TestUpgradeRejectedWithoutActorOrScope(t *testing.T) {
	tests := []struct {
		name  string
		actor *core.Actor
		want  int
	}{
		{name: "unauthenticated", actor: nil, want: http.StatusUnauthorized},
		{name: "missing scope", actor: testActor("t1", core.ScopeTaskRead), want: http.StatusForbidden},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			f := newFixture(t, fixedActor(tc.actor))
			url := "ws" + strings.TrimPrefix(f.srv.URL, "http") + wire.RouteEvents
			c, resp, err := websocket.Dial(ctx, url, nil)
			if resp != nil && resp.Body != nil {
				_ = resp.Body.Close()
			}
			if err == nil {
				_ = c.CloseNow()
				t.Fatal("upgrade succeeded without entitlement")
			}
			if resp == nil || resp.StatusCode != tc.want {
				t.Fatalf("status = %v, want %d", resp, tc.want)
			}
			if f.hub.Len() != 0 {
				t.Fatalf("hub registered %d connections after a refused upgrade", f.hub.Len())
			}
		})
	}
}

func TestSlowConsumerDoesNotBlockOtherConnections(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	f.stream.queue = 1

	stalled := f.dial(t, ctx)
	subscribeOK(t, ctx, stalled, "s1", core.EventFilter{})
	waitUntil(t, "stalled registration", func() bool { return f.hub.Len() == 1 })

	f.stream.queue = defaultSendQueue
	healthy := f.dial(t, ctx)
	subscribeOK(t, ctx, healthy, "s1", core.EventFilter{})
	waitUntil(t, "healthy registration", func() bool { return f.hub.Len() == 2 })

	for i := 1; i <= 64; i++ {
		f.hub.Broadcast(core.Event{Seq: int64(i), TenantID: "t1", Type: core.EventTaskCreated, SubjectID: "e"})
	}

	m := read(t, ctx, healthy)
	if m.Type != MsgEvent {
		t.Fatalf("healthy connection got %+v while a peer stalled", m)
	}
}

func TestClosedConnectionIsUnregisteredAndLeavesNoGoroutine(t *testing.T) {
	ctx := context.Background()
	base := runtime.NumGoroutine()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})
	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })

	if err := c.Close(websocket.StatusNormalClosure, "done"); err != nil {
		t.Fatalf("close: %v", err)
	}
	waitUntil(t, "unregistration", func() bool { return f.hub.Len() == 0 })
	f.srv.Close()
	waitUntil(t, "goroutines to drain", func() bool { return runtime.NumGoroutine() <= base+2 })
}

func TestReplayFailureIsReportedToTheSubscriber(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	f.log.add(core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated})
	f.log.readErr = errors.New("log unreadable")

	c := f.dial(t, ctx)
	since := int64(1)
	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: "s1", SinceSeq: &since})

	if m := read(t, ctx, c); m.Type != MsgSubscribed {
		t.Fatalf("ack = %+v", m)
	}
	m := read(t, ctx, c)
	if m.Type != MsgError || m.Error.Code != core.KindInternal || m.Error.Message != "internal error" {
		t.Fatalf("got %+v, want a redacted internal error", m)
	}
}

func TestCursorLookupFailureIsReportedToTheSubscriber(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	f.log.err = errors.New("log unreadable")

	c := f.dial(t, ctx)
	since := int64(9)
	send(t, ctx, c, ClientMessage{Type: MsgSubscribe, ID: "s1", SinceSeq: &since})
	if m := read(t, ctx, c); m.Type != MsgError || m.Error.Code != core.KindInternal {
		t.Fatalf("got %+v, want an internal error", m)
	}
}

func TestBinaryMessageIsRejectedWithoutClosing(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)

	if err := c.Write(ctx, websocket.MessageBinary, []byte("{}")); err != nil {
		t.Fatalf("write: %v", err)
	}
	if m := read(t, ctx, c); m.Type != MsgError {
		t.Fatalf("got %+v, want an error message", m)
	}
	send(t, ctx, c, ClientMessage{Type: MsgPing, ID: "alive"})
	if m := read(t, ctx, c); m.Type != MsgPong {
		t.Fatalf("connection did not survive a binary frame: %+v", m)
	}
}

func TestFailedConnectionIsClosedWithPolicyViolation(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t, fixedActor(testActor("t1")))
	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})
	waitUntil(t, "registration", func() bool { return f.hub.Len() == 1 })

	for _, conn := range f.hub.snapshot() {
		conn.fail("slow consumer: outbound queue full, resume from your last seq")
	}

	if m := read(t, ctx, c); m.Type != MsgError || !strings.Contains(m.Error.Message, "slow consumer") {
		t.Fatalf("got %+v, want a slow consumer error", m)
	}
	readCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	_, _, err := c.Read(readCtx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close status = %v (%v), want policy violation", websocket.CloseStatus(err), err)
	}
	waitUntil(t, "unregistration", func() bool { return f.hub.Len() == 0 })
}

func TestTruncateReasonFitsTheCloseFrame(t *testing.T) {
	long := strings.Repeat("x", 300)
	if got := truncateReason(long); len(got) != 120 {
		t.Fatalf("truncated to %d bytes, want 120", len(got))
	}
	if got := truncateReason("short"); got != "short" {
		t.Fatalf("truncateReason mangled a short reason: %q", got)
	}
}

// TestSubscriptionAcceptsEveryCoreEventType walks the contract package's
// vocabulary instead of restating it, so re-hardcoding the closed set here
// fails the moment core declares an event type the copy misses.
func TestSubscriptionAcceptsEveryCoreEventType(t *testing.T) {
	types := core.EventTypes()
	if len(types) == 0 {
		t.Fatal("core.EventTypes() is empty; the subscription filter would accept nothing")
	}
	for _, typ := range types {
		if err := validateEventTypes([]core.EventType{typ}); err != nil {
			t.Errorf("subscribing to %q is refused: %v", typ, err)
		}
	}
	if err := validateEventTypes([]core.EventType{"nope.invented"}); err == nil {
		t.Error("an invented event type was accepted; the filter is not a closed set")
	}
}
