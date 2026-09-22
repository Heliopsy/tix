package httpapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// revalidatingFixture serves the event stream with a revalidator the test
// drives, standing in for the credential lookup the middleware installs.
type revalidatingFixture struct {
	srv    *httptest.Server
	hub    *Hub
	log    *fakeLog
	stream *EventStream
	calls  atomic.Int64
}

func newRevalidatingFixture(t *testing.T, actor *core.Actor, resolve func() (*core.Actor, error)) *revalidatingFixture {
	t.Helper()
	f := &revalidatingFixture{log: &fakeLog{}, hub: NewHub()}
	f.stream = NewEventStream(f.hub, f.log)
	f.stream.pingInterval = 20 * time.Millisecond
	f.stream.pingTimeout = 200 * time.Millisecond

	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := core.WithActor(r.Context(), actor)
		if resolve != nil {
			ctx = context.WithValue(ctx, ctxKeyRevalidate, Revalidate(func(context.Context) (*core.Actor, error) {
				f.calls.Add(1)
				return resolve()
			}))
		}
		f.stream.ServeHTTP(w, r.WithContext(ctx))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *revalidatingFixture) dial(t *testing.T, ctx context.Context) *websocket.Conn {
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

// drainUntilClosed reads until the socket goes away, returning the last error.
func drainUntilClosed(t *testing.T, ctx context.Context, c *websocket.Conn) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	for {
		if _, _, err := c.Read(ctx); err != nil {
			return err
		}
	}
}

func streamActor() *core.Actor {
	return &core.Actor{ID: "actor-1", TenantID: "tenant-1", Kind: core.ActorUser,
		Handle: "ada", Role: core.RoleAdmin}
}

// TestStreamStopsWhenCredentialIsRevoked pins that an open event stream does
// not outlive the credential it was upgraded with. Before the periodic
// re-check the socket stayed up, kept alive by its own ping, until the client
// hung up.
func TestStreamStopsWhenCredentialIsRevoked(t *testing.T) {
	ctx := context.Background()
	var revoked atomic.Bool
	f := newRevalidatingFixture(t, streamActor(), func() (*core.Actor, error) {
		if revoked.Load() {
			return nil, core.Unauthenticated("invalid credentials")
		}
		return streamActor(), nil
	})

	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})

	revoked.Store(true)
	err := drainUntilClosed(t, ctx, c)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close status = %v (err %v), want policy violation", websocket.CloseStatus(err), err)
	}
	if f.calls.Load() == 0 {
		t.Fatal("the stream never re-checked its credential")
	}
}

// TestStreamStopsWhenRoleChanges pins that a demotion reaches a live stream
// rather than waiting out the session lifetime.
func TestStreamStopsWhenRoleChanges(t *testing.T) {
	ctx := context.Background()
	var demoted atomic.Bool
	f := newRevalidatingFixture(t, streamActor(), func() (*core.Actor, error) {
		a := streamActor()
		if demoted.Load() {
			a.Role = core.RoleViewer
		}
		return a, nil
	})

	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})

	demoted.Store(true)
	err := drainUntilClosed(t, ctx, c)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("close status = %v (err %v), want policy violation", websocket.CloseStatus(err), err)
	}
}

// TestStreamSurvivesWhileCredentialHolds pins that re-checking does not close
// a healthy stream, and that it happens once per ping rather than per event.
func TestStreamSurvivesWhileCredentialHolds(t *testing.T) {
	ctx := context.Background()
	f := newRevalidatingFixture(t, streamActor(), func() (*core.Actor, error) {
		return streamActor(), nil
	})

	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})

	for i := range 20 {
		f.hub.Broadcast(core.Event{Seq: int64(i + 1), TenantID: "tenant-1",
			Type: core.EventTaskCreated, ProjectID: "p1"})
	}
	for range 20 {
		if m := read(t, ctx, c); m.Type != MsgEvent {
			t.Fatalf("message = %+v, want an event", m)
		}
	}
	waitUntil(t, "a revalidation tick", func() bool { return f.calls.Load() >= 1 })
	if got := f.calls.Load(); got > 5 {
		t.Fatalf("revalidated %d times for 20 events; it must ride the ping, not the event", got)
	}

	send(t, ctx, c, ClientMessage{Type: MsgPing, ID: "p"})
	if m := read(t, ctx, c); m.Type != MsgPong {
		t.Fatalf("message = %+v, want a pong on a live stream", m)
	}
}

// TestStreamWithoutRevalidatorIsUnchanged pins that a surface mounting the
// stream without a credential revalidator keeps working.
func TestStreamWithoutRevalidatorIsUnchanged(t *testing.T) {
	ctx := context.Background()
	f := newRevalidatingFixture(t, streamActor(), nil)

	c := f.dial(t, ctx)
	subscribeOK(t, ctx, c, "s1", core.EventFilter{})
	time.Sleep(60 * time.Millisecond)
	send(t, ctx, c, ClientMessage{Type: MsgPing, ID: "p"})
	if m := read(t, ctx, c); m.Type != MsgPong {
		t.Fatalf("message = %+v, want a pong", m)
	}
}
