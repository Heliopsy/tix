package httpapi

import (
	"context"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
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

func TestHubPumpStopsOnContextCancel(t *testing.T) {
	h := NewHub()
	c := newConn(testActor("t1"), 8)
	liveSub(c, "s1", core.EventFilter{})
	h.Register(c)

	src := make(chan core.Event)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Pump(ctx, src)
	}()

	src <- core.Event{Seq: 1, TenantID: "t1", Type: core.EventTaskCreated}
	if m := <-c.send; m.Type != MsgEvent {
		t.Fatalf("pumped message type = %q", m.Type)
	}
	cancel()
	<-done
}

func TestHubPumpStopsWhenSourceCloses(t *testing.T) {
	h := NewHub()
	src := make(chan core.Event)
	done := make(chan struct{})
	go func() {
		defer close(done)
		h.Pump(context.Background(), src)
	}()
	close(src)
	<-done
}

func TestConnFailIsIdempotent(t *testing.T) {
	c := newConn(testActor("t1"), 1)
	c.fail("first")
	c.fail("second")
	if c.failure() != "first" {
		t.Fatalf("failure reason = %q, want %q", c.failure(), "first")
	}
}
