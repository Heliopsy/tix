package client

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Message types exchanged over the event stream.
const (
	MessageSubscribe   = "subscribe"
	MessageUnsubscribe = "unsubscribe"
	MessagePing        = "ping"
	MessageSubscribed  = "subscribed"
	MessageEvent       = "event"
	MessagePong        = "pong"
	MessageError       = "error"
)

// eventBuffer bounds how many delivered events are held for a slow caller.
const eventBuffer = 64

// maxEventMessage bounds one inbound stream message.
const maxEventMessage = 8 << 20

type subscribeMessage struct {
	Type     string           `json:"type"`
	ID       string           `json:"id"`
	Filter   core.EventFilter `json:"filter"`
	SinceSeq int64            `json:"since_seq,omitempty"`
}

type serverMessage struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	SinceSeq int64           `json:"since_seq,omitempty"`
	Event    *core.Event     `json:"event,omitempty"`
	Error    *wire.ErrorBody `json:"error,omitempty"`
}

// Subscribe streams matching events until ctx is cancelled.
func (c *Client) Subscribe(ctx context.Context, f core.EventFilter) (<-chan core.Event, error) {
	conn, err := c.dialEvents(ctx)
	if err != nil {
		return nil, err
	}
	conn.SetReadLimit(maxEventMessage)

	sub := subscribeMessage{Type: MessageSubscribe, ID: subscriptionID(), Filter: f, SinceSeq: f.SinceSeq}
	raw, err := json.Marshal(sub)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "encode")
		return nil, core.Internal("encoding subscribe message: %v", err).Wrap(err)
	}
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		_ = conn.Close(websocket.StatusInternalError, "write")
		return nil, transportError(http.MethodGet, c.eventsURL(), err)
	}

	first, err := readServerMessage(ctx, conn)
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "read")
		return nil, err
	}
	if first.Type == MessageError {
		_ = conn.Close(websocket.StatusNormalClosure, "")
		return nil, envelopeError(first.Error)
	}

	out := make(chan core.Event, eventBuffer)
	c.track(func() { _ = conn.CloseNow() })
	go pump(ctx, conn, first, out)
	return out, nil
}

func pump(ctx context.Context, conn *websocket.Conn, first *serverMessage, out chan<- core.Event) {
	defer close(out)
	defer func() { _ = conn.Close(websocket.StatusNormalClosure, "") }()

	if first.Type == MessageEvent && first.Event != nil && !deliver(ctx, out, *first.Event) {
		return
	}
	for {
		msg, err := readServerMessage(ctx, conn)
		if err != nil {
			return
		}
		if msg.Type == MessageEvent && msg.Event != nil && !deliver(ctx, out, *msg.Event) {
			return
		}
	}
}

func deliver(ctx context.Context, out chan<- core.Event, ev core.Event) bool {
	select {
	case out <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

func readServerMessage(ctx context.Context, conn *websocket.Conn) (*serverMessage, error) {
	_, raw, err := conn.Read(ctx)
	if err != nil {
		return nil, transportError(http.MethodGet, "event stream", err)
	}
	var msg serverMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		return nil, core.Internal("decoding stream message: %v", err).Wrap(err)
	}
	return &msg, nil
}

func envelopeError(body *wire.ErrorBody) error {
	if body == nil || body.Code == "" {
		return core.Internal("event stream refused the subscription")
	}
	return &core.Error{Kind: body.Code, Message: body.Message, Details: body.Details}
}

func (c *Client) dialEvents(ctx context.Context) (*websocket.Conn, error) {
	header := http.Header{}
	if c.token != "" {
		header.Set(wire.HeaderAuth, "Bearer "+c.token)
	}
	if c.cookie != "" {
		// #nosec G124 -- see transport.go: an outgoing cookie header carries no
		// response directives.
		header.Set("Cookie", (&http.Cookie{Name: wire.SessionCookieName, Value: c.cookie}).String())
	}
	header.Set("User-Agent", c.userAgent)

	conn, resp, err := websocket.Dial(ctx, c.eventsURL(), &websocket.DialOptions{
		HTTPClient: c.http,
		HTTPHeader: header,
	})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		if resp != nil && resp.StatusCode >= 300 {
			return nil, statusError(resp.StatusCode, nil)
		}
		return nil, transportError(http.MethodGet, c.eventsURL(), err)
	}
	return conn, nil
}

// subscriptionID names one subscription on a connection.
func subscriptionID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "sub"
	}
	return "sub-" + hex.EncodeToString(b[:])
}

func (c *Client) eventsURL() string {
	u, err := url.Parse(c.url(wire.RouteEvents, nil))
	if err != nil {
		return c.url(wire.RouteEvents, nil)
	}
	switch u.Scheme {
	case "https":
		u.Scheme = "wss"
	default:
		u.Scheme = "ws"
	}
	return u.String()
}
