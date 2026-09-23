// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

func eventServer(t *testing.T, handle func(ctx context.Context, conn *websocket.Conn, sub subscribeMessage)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

		ctx := r.Context()
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return
		}
		var sub subscribeMessage
		if err := json.Unmarshal(raw, &sub); err != nil {
			return
		}
		handle(ctx, conn, sub)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func writeMessage(ctx context.Context, conn *websocket.Conn, v any) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, raw)
}

func TestSubscribeDeliversEvents(t *testing.T) {
	seen := make(chan subscribeMessage, 1)
	srv := eventServer(t, func(ctx context.Context, conn *websocket.Conn, sub subscribeMessage) {
		seen <- sub
		if err := writeMessage(ctx, conn, serverMessage{Type: MessageSubscribed, ID: "s1"}); err != nil {
			return
		}
		for i := 1; i <= 2; i++ {
			ev := core.Event{Seq: int64(i), ID: "e", Type: core.EventTaskCreated}
			if err := writeMessage(ctx, conn, serverMessage{Type: MessageEvent, ID: "s1", Event: &ev}); err != nil {
				return
			}
		}
		<-ctx.Done()
	})

	c, err := New(srv.URL, "secret-token")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	filter := core.EventFilter{ProjectIDs: []string{"p1"}, Types: []core.EventType{core.EventTaskCreated}, SinceSeq: 7}
	events, err := c.Subscribe(ctx, filter)
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	sub := <-seen
	if sub.Type != MessageSubscribe || sub.SinceSeq != 7 || sub.ID == "" {
		t.Fatalf("subscribe message = %+v", sub)
	}
	if len(sub.Filter.ProjectIDs) != 1 || sub.Filter.ProjectIDs[0] != "p1" {
		t.Fatalf("filter = %+v", sub.Filter)
	}
	if len(sub.Filter.Types) != 1 || sub.Filter.Types[0] != core.EventTaskCreated {
		t.Fatalf("filter types = %+v", sub.Filter.Types)
	}

	for want := int64(1); want <= 2; want++ {
		select {
		case ev := <-events:
			if ev.Seq != want {
				t.Fatalf("seq = %d, want %d", ev.Seq, want)
			}
		case <-time.After(eventWait):
			t.Fatal("timed out waiting for event")
		}
	}

	cancel()
	// The assertion is that cancellation eventually closes the channel, not
	// that it happens within some latency budget: a loaded runner under -race
	// takes its time propagating the cancel through the websocket read. The
	// drain is bounded for the same reason, so a channel that never closes
	// fails here rather than hanging the package.
	deadline := time.After(closeWait)
	for {
		select {
		case _, open := <-events:
			if !open {
				return
			}
		case <-deadline:
			t.Fatal("channel not closed after cancellation")
		}
	}
}

// closeWait bounds how long a cancelled subscription may take to close its
// channel, and eventWait how long a message may take to arrive. Both are
// generous on purpose: they exist to fail a hang, not to police latency.
const (
	closeWait = 30 * time.Second
	eventWait = 30 * time.Second
)

func TestSubscribeSurfacesServerError(t *testing.T) {
	srv := eventServer(t, func(ctx context.Context, conn *websocket.Conn, _ subscribeMessage) {
		body := wire.ErrorBody{
			Code:    core.KindInvalid,
			Message: "unknown event type",
			Details: map[string]any{"type": "task.exploded"},
		}
		_ = writeMessage(ctx, conn, serverMessage{Type: MessageError, Error: &body})
	})

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	_, err = c.Subscribe(context.Background(), core.EventFilter{})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
	var domain *core.Error
	if !errors.As(err, &domain) || domain.Details["type"] != "task.exploded" {
		t.Fatalf("error = %v", err)
	}
}

func TestSubscribeErrorWithoutBody(t *testing.T) {
	srv := eventServer(t, func(ctx context.Context, conn *websocket.Conn, _ subscribeMessage) {
		_ = writeMessage(ctx, conn, serverMessage{Type: MessageError})
	})

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Subscribe(context.Background(), core.EventFilter{}); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestSubscribeRefusedAtUpgrade(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, core.Unauthenticated("token required"))
	}))
	defer srv.Close()

	c, err := New(srv.URL, "t", WithSessionCookie("sess"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Subscribe(context.Background(), core.EventFilter{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestSubscribeTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()
	srv.Close()

	if _, err := c.Subscribe(context.Background(), core.EventFilter{}); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestSubscribeMalformedMessage(t *testing.T) {
	srv := eventServer(t, func(ctx context.Context, conn *websocket.Conn, _ subscribeMessage) {
		_ = conn.Write(ctx, websocket.MessageText, []byte("{not json"))
	})

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if _, err := c.Subscribe(context.Background(), core.EventFilter{}); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestSubscribeIgnoresNonEventMessages(t *testing.T) {
	srv := eventServer(t, func(ctx context.Context, conn *websocket.Conn, _ subscribeMessage) {
		_ = writeMessage(ctx, conn, serverMessage{Type: MessageSubscribed, ID: "s1"})
		_ = writeMessage(ctx, conn, serverMessage{Type: MessagePong})
		ev := core.Event{Seq: 9, Type: core.EventTaskUpdated}
		_ = writeMessage(ctx, conn, serverMessage{Type: MessageEvent, ID: "s1", Event: &ev})
		<-ctx.Done()
	})

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	events, err := c.Subscribe(ctx, core.EventFilter{})
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	select {
	case ev := <-events:
		if ev.Seq != 9 {
			t.Fatalf("seq = %d", ev.Seq)
		}
	case <-time.After(eventWait):
		t.Fatal("timed out")
	}

	if err := c.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	select {
	case <-events:
	case <-time.After(eventWait):
		t.Fatal("close did not end the stream")
	}
}

func TestEventsURLSchemes(t *testing.T) {
	tests := []struct{ in, want string }{
		{"http://example.com", "ws://example.com" + wire.RouteEvents},
		{"https://example.com/tix", "wss://example.com/tix" + wire.RouteEvents},
	}
	for _, tc := range tests {
		c, err := New(tc.in, "t")
		if err != nil {
			t.Fatalf("New(%q): %v", tc.in, err)
		}
		if got := c.eventsURL(); got != tc.want {
			t.Errorf("eventsURL = %q, want %q", got, tc.want)
		}
	}
}
