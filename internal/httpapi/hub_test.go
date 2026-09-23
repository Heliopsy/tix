// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func testActor(tenant string, scopes ...core.Scope) *core.Actor {
	if len(scopes) == 0 {
		scopes = []core.Scope{core.ScopeEventSubscribe}
	}
	return &core.Actor{ID: "actor-" + tenant, TenantID: tenant, Kind: core.ActorAgent, Handle: "a", Scopes: scopes}
}

func liveSub(c *wsConn, id string, f core.EventFilter) *subscription {
	s, err := c.addSub(id, f)
	if err != nil {
		panic(err)
	}
	s.activate(c)
	return s
}

func TestHubRegisterUnregisterLen(t *testing.T) {
	h := NewHub()
	c := newConn(testActor("t1"), 4)
	if h.Len() != 0 {
		t.Fatalf("fresh hub has %d connections", h.Len())
	}
	h.Register(c)
	if h.Len() != 1 {
		t.Fatalf("after register Len = %d, want 1", h.Len())
	}
	h.Unregister(c)
	if h.Len() != 0 {
		t.Fatalf("after unregister Len = %d, want 0", h.Len())
	}
}

func TestHubBroadcastEntitlement(t *testing.T) {
	tests := []struct {
		name  string
		actor *core.Actor
		event core.Event
		want  bool
	}{
		{
			name:  "same tenant delivered",
			actor: testActor("t1"),
			event: core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated},
			want:  true,
		},
		{
			name:  "other tenant withheld",
			actor: testActor("t2"),
			event: core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated},
			want:  false,
		},
		{
			name:  "missing scope withheld",
			actor: testActor("t1", core.ScopeTaskRead),
			event: core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated},
			want:  false,
		},
		{
			name:  "project restricted actor gets its project",
			actor: &core.Actor{ID: "p", TenantID: "t1", ProjectID: "proj-1", Scopes: []core.Scope{core.ScopeEventSubscribe}},
			event: core.Event{Seq: 1, TenantID: "t1", ProjectID: "proj-1", Type: core.EventTaskCreated},
			want:  true,
		},
		{
			name:  "project restricted actor denied another project",
			actor: &core.Actor{ID: "p", TenantID: "t1", ProjectID: "proj-1", Scopes: []core.Scope{core.ScopeEventSubscribe}},
			event: core.Event{Seq: 1, TenantID: "t1", ProjectID: "proj-2", Type: core.EventTaskCreated},
			want:  false,
		},
		{
			name:  "admin role without explicit scope delivered",
			actor: &core.Actor{ID: "r", TenantID: "t1", Role: core.RoleAdmin},
			event: core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated},
			want:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHub()
			c := newConn(tc.actor, 4)
			liveSub(c, "s1", core.EventFilter{})
			h.Register(c)
			h.Broadcast(tc.event)

			got := len(c.send) == 1
			if got != tc.want {
				t.Fatalf("delivered = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestHubBroadcastAppliesSubscriptionFilter(t *testing.T) {
	h := NewHub()
	c := newConn(testActor("t1"), 8)
	liveSub(c, "typed", core.EventFilter{Types: []core.EventType{core.EventTaskTransitioned}})
	liveSub(c, "wild", core.EventFilter{Types: []core.EventType{"task.*"}})
	h.Register(c)

	h.Broadcast(core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated})

	if len(c.send) != 1 {
		t.Fatalf("queued %d messages, want 1", len(c.send))
	}
	m := <-c.send
	if m.ID != "wild" || m.Type != MsgEvent {
		t.Fatalf("got %+v, want an event on subscription wild", m)
	}
}

func TestHubBroadcastSkipsAlreadySentSequence(t *testing.T) {
	c := newConn(testActor("t1"), 8)
	s := liveSub(c, "s1", core.EventFilter{})
	c.route(core.Event{Seq: 7, TenantID: "t1", Type: core.EventTaskCreated})
	c.route(core.Event{Seq: 7, TenantID: "t1", Type: core.EventTaskCreated})

	if len(c.send) != 1 {
		t.Fatalf("queued %d messages, want 1", len(c.send))
	}
	if s.at() != 7 {
		t.Fatalf("subscription cursor = %d, want 7", s.at())
	}
}

func TestHubSlowConsumerIsDroppedWithoutBlockingOthers(t *testing.T) {
	h := NewHub()
	slow := newConn(testActor("t1"), 2)
	fast := newConn(testActor("t1"), 64)
	liveSub(slow, "s1", core.EventFilter{})
	liveSub(fast, "s1", core.EventFilter{})
	h.Register(slow)
	h.Register(fast)

	go func() {
		for range fast.send {
		}
	}()

	for i := 1; i <= 32; i++ {
		h.Broadcast(core.Event{Seq: int64(i), TenantID: "t1", Type: core.EventTaskCreated})
	}

	select {
	case <-slow.closed:
	default:
		t.Fatal("slow connection was not closed")
	}
	if slow.failure() == "" {
		t.Fatal("slow connection carries no failure reason")
	}
	select {
	case <-fast.closed:
		t.Fatal("fast connection was closed by a slow peer")
	default:
	}
}

func TestSubscriptionBuffersUntilActivated(t *testing.T) {
	c := newConn(testActor("t1"), 8)
	s, err := c.addSub("s1", core.EventFilter{})
	if err != nil {
		t.Fatalf("addSub: %v", err)
	}
	c.route(core.Event{Seq: 3, TenantID: "t1", Type: core.EventTaskCreated})
	if len(c.send) != 0 {
		t.Fatalf("queued %d messages before activation, want 0", len(c.send))
	}

	s.note(3)
	if !s.activate(c) {
		t.Fatal("activate reported a full queue")
	}
	if len(c.send) != 0 {
		t.Fatalf("replayed sequence was delivered twice: %d queued", len(c.send))
	}

	c.route(core.Event{Seq: 4, TenantID: "t1", Type: core.EventTaskCreated})
	if len(c.send) != 1 {
		t.Fatalf("queued %d messages after activation, want 1", len(c.send))
	}
}

func TestSubscriptionPendingOverflowFailsConnection(t *testing.T) {
	c := newConn(testActor("t1"), 4)
	if _, err := c.addSub("s1", core.EventFilter{}); err != nil {
		t.Fatalf("addSub: %v", err)
	}
	for i := 1; i <= maxPendingEvents+1; i++ {
		c.route(core.Event{Seq: int64(i), TenantID: "t1", Type: core.EventTaskCreated})
	}
	select {
	case <-c.closed:
	default:
		t.Fatal("pending overflow did not close the connection")
	}
}

func TestConnAddSubRejectsDuplicateAndRemoveReportsUnknown(t *testing.T) {
	c := newConn(testActor("t1"), 4)
	if _, err := c.addSub("s1", core.EventFilter{}); err != nil {
		t.Fatalf("addSub: %v", err)
	}
	if _, err := c.addSub("s1", core.EventFilter{}); err == nil {
		t.Fatal("duplicate subscription id accepted")
	}
	if !c.removeSub("s1") {
		t.Fatal("removeSub reported an unknown subscription")
	}
	if c.removeSub("s1") {
		t.Fatal("removeSub accepted an already removed subscription")
	}
}

// pumpDeadline bounds every wait in the Pump tests. A Pump that never reads,
// or never returns, has to fail the test rather than stall the suite: the
// whole point of these two is that the goroutine stops.
const pumpDeadline = 5 * time.Second

// pumping starts a Pump over a fresh source and returns the source, a
// connection subscribed to everything, and the channel closed when Pump
// returns.
func pumping(ctx context.Context, t *testing.T) (chan core.Event, *wsConn, chan struct{}) {
	t.Helper()
	h := NewHub()
	c := newConn(testActor("t1"), 8)
	liveSub(c, "s1", core.EventFilter{})
	h.Register(c)

	src := make(chan core.Event)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Pump(ctx, src)
	}()
	return src, c, done
}

// pumpOne sends one event and waits for it to reach the connection, which is
// what proves Pump is reading its source and broadcasting at all.
func pumpOne(t *testing.T, src chan<- core.Event, c *wsConn) {
	t.Helper()
	select {
	case src <- core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated}:
	case <-time.After(pumpDeadline):
		t.Fatal("Pump never read from its source")
	}
	select {
	case m := <-c.send:
		if m.Type != MsgEvent {
			t.Fatalf("pumped message type = %q, want %q", m.Type, MsgEvent)
		}
	case <-time.After(pumpDeadline):
		t.Fatal("Pump read an event but did not broadcast it")
	}
}

// wantStopped fails if Pump has not returned within the deadline.
func wantStopped(t *testing.T, done <-chan struct{}, why string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(pumpDeadline):
		t.Fatalf("Pump did not return %s", why)
	}
}

func TestHubPumpStopsOnContextCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	src, c, done := pumping(ctx, t)

	pumpOne(t, src, c)

	select {
	case <-done:
		t.Fatal("Pump returned while its context was still live")
	default:
	}

	cancel()
	wantStopped(t, done, "when its context was cancelled")
}

func TestHubPumpStopsWhenSourceCloses(t *testing.T) {
	src, c, done := pumping(context.Background(), t)

	pumpOne(t, src, c)

	select {
	case <-done:
		t.Fatal("Pump returned while its source was still open")
	default:
	}

	close(src)
	wantStopped(t, done, "when its source closed")
}

func TestConnFailIsIdempotent(t *testing.T) {
	c := newConn(testActor("t1"), 1)
	c.fail("first")
	c.fail("second")
	if c.failure() != "first" {
		t.Fatalf("failure reason = %q, want %q", c.failure(), "first")
	}
}

// hookCall is one tenant hook firing and the tenant count it observed.
type hookCall struct {
	kind   string
	tenant string
	count  int
}

// hookRecorder collects tenant hook calls with the state each one saw.
type hookRecorder struct {
	hub *Hub

	mu    sync.Mutex
	calls []hookCall

	gate func(kind string)
}

func newHookRecorder(h *Hub) *hookRecorder {
	r := &hookRecorder{hub: h}
	h.SetTenantHooks(r.hook("first"), r.hook("last"))
	return r
}

func (r *hookRecorder) hook(kind string) func(string) {
	return func(tenant string) {
		if r.gate != nil {
			r.gate(kind)
		}
		count := r.hub.tenantCount(tenant)
		r.mu.Lock()
		defer r.mu.Unlock()
		r.calls = append(r.calls, hookCall{kind: kind, tenant: tenant, count: count})
	}
}

func (r *hookRecorder) recorded() []hookCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]hookCall(nil), r.calls...)
}

func (r *hookRecorder) kinds() []string {
	out := []string{}
	for _, c := range r.recorded() {
		out = append(out, c.kind+":"+c.tenant)
	}
	return out
}

func TestHubTenantHooksFireOnceAtEachEdge(t *testing.T) {
	h := NewHub()
	rec := newHookRecorder(h)

	first := newConn(testActor("t1"), 4)
	second := newConn(testActor("t1"), 4)

	h.Register(first)
	h.Register(second)
	h.Unregister(first)
	h.Unregister(second)

	want := []string{"first:t1", "last:t1"}
	if got := rec.kinds(); !equalStrings(got, want) {
		t.Errorf("hook calls = %v, want %v", got, want)
	}
	for _, c := range rec.recorded() {
		if c.kind == "first" && c.count != 1 {
			t.Errorf("onFirst saw count %d, want 1", c.count)
		}
		if c.kind == "last" && c.count != 0 {
			t.Errorf("onLast saw count %d, want 0", c.count)
		}
	}
}

func TestHubTenantHooksAreIndependentPerTenant(t *testing.T) {
	h := NewHub()
	rec := newHookRecorder(h)

	one := newConn(testActor("t1"), 4)
	two := newConn(testActor("t2"), 4)

	h.Register(one)
	h.Register(two)
	h.Unregister(two)
	h.Unregister(one)

	want := []string{"first:t1", "first:t2", "last:t2", "last:t1"}
	if got := rec.kinds(); !equalStrings(got, want) {
		t.Errorf("hook calls = %v, want %v", got, want)
	}
}

func TestHubTenantHooksSkipConnectionsWithoutATenant(t *testing.T) {
	tests := []struct {
		name string
		conn *wsConn
	}{
		{name: "no actor", conn: newConn(nil, 4)},
		{name: "empty tenant", conn: newConn(&core.Actor{ID: "a"}, 4)},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			h := NewHub()
			rec := newHookRecorder(h)
			h.Register(tc.conn)
			h.Unregister(tc.conn)
			if got := rec.kinds(); len(got) != 0 {
				t.Errorf("hook calls = %v, want none", got)
			}
		})
	}
}

// A hub with no hooks set, and one whose hooks were cleared, still has to
// track its connections: the hooks are a notification, not the bookkeeping.
func TestHubWithoutTenantHooksIsSafe(t *testing.T) {
	h := NewHub()
	c := newConn(testActor("t1"), 4)

	for _, stage := range []string{"never set", "cleared"} {
		if stage == "cleared" {
			h.SetTenantHooks(nil, nil)
		}
		h.Register(c)
		if got := h.tenantCount("t1"); got != 1 {
			t.Errorf("%s: tenant count after register = %d, want 1", stage, got)
		}
		if got := h.Len(); got != 1 {
			t.Errorf("%s: Len after register = %d, want 1", stage, got)
		}
		h.Unregister(c)
		if got := h.tenantCount("t1"); got != 0 {
			t.Errorf("%s: tenant count after unregister = %d, want 0", stage, got)
		}
		if got := h.Len(); got != 0 {
			t.Errorf("%s: Len after unregister = %d, want 0", stage, got)
		}
	}
}

func TestHubSetTenantHooksReplacesThePreviousPair(t *testing.T) {
	h := NewHub()
	rec := newHookRecorder(h)

	replaced := make(chan string, 2)
	h.SetTenantHooks(func(tenant string) { replaced <- "first:" + tenant }, func(tenant string) { replaced <- "last:" + tenant })

	c := newConn(testActor("t1"), 4)
	h.Register(c)
	h.Unregister(c)

	if got := rec.kinds(); len(got) != 0 {
		t.Errorf("the replaced hooks still fired: %v", got)
	}
	close(replaced)
	var got []string
	for s := range replaced {
		got = append(got, s)
	}
	if want := []string{"first:t1", "last:t1"}; !equalStrings(got, want) {
		t.Errorf("hook calls = %v, want %v", got, want)
	}
}

// Unregistering a connection the hub never held must not report the tenant's
// last connection as gone.
func TestHubUnregisterOfAnUnknownConnectionFiresNothing(t *testing.T) {
	h := NewHub()
	rec := newHookRecorder(h)
	h.Unregister(newConn(testActor("t1"), 4))
	if got := rec.kinds(); len(got) != 0 {
		t.Errorf("hook calls = %v, want none", got)
	}
}

// A reconnect races a disconnect on one tenant. If the hooks may run out of
// order with the counts that triggered them, the surviving connection is left
// with no reader and the count never returns to zero to start one, so the
// stream is silently dead forever.
//
// The interleaving is driven, not waited for: onLast is held inside the hook
// while the reconnect is attempted, so the defect reproduces on every run.
func TestHubTenantHookOrderMatchesCountOrder(t *testing.T) {
	h := NewHub()
	rec := newHookRecorder(h)

	entered := make(chan struct{})
	release := make(chan struct{})
	reconnected := make(chan struct{})
	rec.gate = func(kind string) {
		if kind != "last" {
			return
		}
		close(entered)
		<-release
	}

	going := newConn(testActor("t1"), 4)
	coming := newConn(testActor("t1"), 4)

	h.Register(going)

	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Unregister(going)
	}()
	<-entered

	go func() {
		defer close(reconnected)
		h.Register(coming)
	}()

	// The reconnect must not be able to change the tenant count while a hook
	// for an earlier transition is still in flight. Only the broken ordering
	// lets it finish here; the fix holds it until onLast returns.
	select {
	case <-reconnected:
		t.Error("a reconnect changed the tenant count while onLast was still running")
	case <-time.After(200 * time.Millisecond):
	}

	close(release)
	<-done
	<-reconnected

	calls := rec.recorded()
	for _, c := range calls {
		if c.kind == "last" && c.count != 0 {
			t.Errorf("onLast ran with %d live connections on %q: the live connection now has no reader", c.count, c.tenant)
		}
	}
	if len(calls) == 0 || calls[len(calls)-1].kind != "first" {
		t.Errorf("hook calls = %v, want the reconnect's onFirst last", rec.kinds())
	}
	if got := h.tenantCount("t1"); got != 1 {
		t.Errorf("tenant count after the reconnect = %d, want 1", got)
	}
}

func equalStrings(a, b []string) bool {
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
