package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Client message types accepted on the event stream.
const (
	MsgSubscribe   = "subscribe"
	MsgUnsubscribe = "unsubscribe"
	MsgPing        = "ping"
)

// Server message types sent on the event stream.
const (
	MsgSubscribed = "subscribed"
	MsgEvent      = "event"
	MsgPong       = "pong"
	MsgError      = "error"
)

// Event stream timings and limits.
const (
	defaultPingInterval = 30 * time.Second
	defaultPingTimeout  = 10 * time.Second
	defaultWriteTimeout = 10 * time.Second
	replayBatch         = 256
	maxClientMessage    = 64 << 10
)

// knownEventTypes is the closed set a subscription filter may name. It is
// derived from the contract package, never copied: a copy would refuse
// subscriptions to an event type the outbox already emits.
var knownEventTypes = core.EventTypes()

// ClientMessage is one message sent by a client on the event stream.
type ClientMessage struct {
	Type     string            `json:"type"`
	ID       string            `json:"id,omitempty"`
	Filter   *core.EventFilter `json:"filter,omitempty"`
	SinceSeq *int64            `json:"since_seq,omitempty"`
}

// ServerMessage is one message sent by the server on the event stream.
type ServerMessage struct {
	Type     string          `json:"type"`
	ID       string          `json:"id,omitempty"`
	SinceSeq int64           `json:"since_seq,omitempty"`
	Event    *core.Event     `json:"event,omitempty"`
	Error    *wire.ErrorBody `json:"error,omitempty"`
}

// EventLog replays durable events so a resuming subscriber loses nothing.
type EventLog interface {
	ReadSince(ctx context.Context, tenantID string, sinceSeq int64, limit int) ([]core.Event, error)
	OldestSeq(ctx context.Context, tenantID string) (int64, error)
}

// EventStream upgrades an authenticated request and serves the WebSocket event protocol.
type EventStream struct {
	hub          *Hub
	log          EventLog
	pingInterval time.Duration
	pingTimeout  time.Duration
	writeTimeout time.Duration
	queue        int
}

// NewEventStream builds a stream fanning out over hub and replaying from log.
func NewEventStream(hub *Hub, log EventLog) *EventStream {
	return &EventStream{
		hub:          hub,
		log:          log,
		pingInterval: defaultPingInterval,
		pingTimeout:  defaultPingTimeout,
		writeTimeout: defaultWriteTimeout,
		queue:        defaultSendQueue,
	}
}

// closeCredentialGone is the reason a stream whose credential stopped being
// valid is closed with. It names no account and no reason beyond the fact.
// #nosec G101 -- a close reason shown to a client, not a credential; the
// scanner matches on the identifier containing "Credential".
const closeCredentialGone = "the credential behind this stream is no longer valid; reconnect"

// ServeHTTP authenticates at upgrade time and then runs the connection.
func (s *EventStream) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	actor, ok := core.ActorFrom(r.Context())
	if !ok {
		WriteError(w, core.Unauthenticated("the event stream requires an authenticated caller"))
		return
	}
	if !actor.HasScope(core.ScopeEventSubscribe) {
		WriteError(w, core.Forbidden("scope %q is required to subscribe to events", core.ScopeEventSubscribe))
		return
	}

	ws, err := websocket.Accept(w, r, &websocket.AcceptOptions{CompressionMode: websocket.CompressionDisabled})
	if err != nil {
		return
	}
	ws.SetReadLimit(maxClientMessage)

	c := newConn(actor, s.queue)
	c.remote = remoteOf(r)
	s.hub.Register(c)
	defer s.hub.Unregister(c)
	revalidate, _ := RevalidateFrom(r.Context())
	s.run(r.Context(), ws, c, revalidate)
}

// stillValid reports whether the credential the stream was opened with still
// resolves to the same authority.
//
// A revoked token, an ended session, an expired session and a role change all
// happen after the upgrade, and none of them reach a connection that only ever
// authenticated once. The actor is compared rather than replaced, because the
// hub reads it from another goroutine: a stream whose authority moved in any
// direction is closed, and the client reconnects under what it holds now.
func stillValid(ctx context.Context, c *wsConn, revalidate Revalidate) bool {
	if revalidate == nil {
		return true
	}
	fresh, err := revalidate(ctx)
	if err != nil || fresh == nil {
		return false
	}
	return sameAuthority(c.actor, fresh)
}

// sameAuthority reports whether two resolutions of one credential grant the same.
func sameAuthority(a, b *core.Actor) bool {
	if a == nil || b == nil {
		return false
	}
	return a.ID == b.ID &&
		a.TenantID == b.TenantID &&
		a.Role == b.Role &&
		a.TokenID == b.TokenID &&
		a.ProjectID == b.ProjectID &&
		slices.Equal(a.Scopes, b.Scopes)
}

// run pairs the read loop with a single writer and tears both down together.
func (s *EventStream) run(ctx context.Context, ws *websocket.Conn, c *wsConn, revalidate Revalidate) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		defer cancel()
		s.writeLoop(ctx, ws, c, revalidate)
	}()

	s.readLoop(ctx, ws, c)
	cancel()
	wg.Wait()
	_ = ws.CloseNow()
}

// readLoop handles client messages until the peer or the context goes away.
func (s *EventStream) readLoop(ctx context.Context, ws *websocket.Conn, c *wsConn) {
	for {
		typ, raw, err := ws.Read(ctx)
		if err != nil {
			return
		}
		if typ != websocket.MessageText {
			c.enqueue(errorMessage("", core.Invalid("event stream messages must be text")))
			continue
		}
		var m ClientMessage
		if err := json.Unmarshal(raw, &m); err != nil {
			c.enqueue(errorMessage("", core.Invalid("message is not valid json")))
			continue
		}
		s.handle(ctx, c, m)
	}
}

// handle dispatches one parsed client message.
func (s *EventStream) handle(ctx context.Context, c *wsConn, m ClientMessage) {
	switch m.Type {
	case MsgSubscribe:
		s.subscribe(ctx, c, m)
	case MsgUnsubscribe:
		if !c.removeSub(m.ID) {
			c.enqueue(errorMessage(m.ID, core.NotFound("no subscription %q on this connection", m.ID)))
		}
	case MsgPing:
		c.enqueue(ServerMessage{Type: MsgPong, ID: m.ID})
	default:
		c.enqueue(errorMessage(m.ID, core.Invalid("unknown message type %q", m.Type)))
	}
}

// subscribe validates a filter, acknowledges it, replays history, then goes live.
func (s *EventStream) subscribe(ctx context.Context, c *wsConn, m ClientMessage) {
	filter, err := s.resolveFilter(ctx, c.actor, m)
	if err != nil {
		c.enqueue(errorMessage(m.ID, err))
		return
	}
	sub, err := c.addSub(m.ID, filter)
	if err != nil {
		c.enqueue(errorMessage(m.ID, err))
		return
	}
	if !c.enqueue(ServerMessage{Type: MsgSubscribed, ID: sub.id, SinceSeq: filter.SinceSeq}) {
		return
	}
	if filter.SinceSeq > 0 && !s.replay(ctx, c, sub) {
		return
	}
	sub.activate(c)
}

// resolveFilter rejects an unusable subscribe request and narrows the filter to the credential.
func (s *EventStream) resolveFilter(ctx context.Context, actor *core.Actor, m ClientMessage) (core.EventFilter, error) {
	var filter core.EventFilter
	if m.Filter != nil {
		filter = *m.Filter
	}
	if m.SinceSeq != nil {
		filter.SinceSeq = *m.SinceSeq
	}
	if strings.TrimSpace(m.ID) == "" {
		return filter, core.Invalid("subscribe requires a subscription id")
	}
	if filter.SinceSeq < 0 {
		return filter, core.Invalid("since_seq must not be negative")
	}
	if err := validateEventTypes(filter.Types); err != nil {
		return filter, err
	}
	if actor.ScopedToProject() {
		if len(filter.ProjectIDs) > 0 && !containsProject(filter.ProjectIDs, actor.ProjectID) {
			return filter, core.Forbidden("this credential may only subscribe to project %q", actor.ProjectID)
		}
		filter.ProjectIDs = []string{actor.ProjectID}
	}
	if filter.SinceSeq > 0 {
		oldest, err := s.log.OldestSeq(ctx, actor.TenantID)
		if err != nil {
			return filter, err
		}
		if oldest > 0 && filter.SinceSeq+1 < oldest {
			return filter, core.Invalid("cursor %d is no longer available; the oldest retained event is %d", filter.SinceSeq, oldest)
		}
	}
	return filter, nil
}

// replay streams the durable log from the cursor before live delivery begins.
func (s *EventStream) replay(ctx context.Context, c *wsConn, sub *subscription) bool {
	cursor := sub.filter.SinceSeq
	for {
		events, err := s.log.ReadSince(ctx, c.actor.TenantID, cursor, replayBatch)
		if err != nil {
			c.enqueue(errorMessage(sub.id, err))
			return false
		}
		for i := range events {
			cursor = events[i].Seq
			if !c.entitled(events[i]) || !sub.filter.Matches(events[i]) {
				continue
			}
			sub.note(events[i].Seq)
			if !c.enqueue(eventMessage(sub.id, events[i])) {
				return false
			}
		}
		if len(events) < replayBatch {
			return true
		}
	}
}

// writeLoop is the only writer on the socket, pinging while it waits for work.
//
// The credential is re-checked on the same tick as the ping rather than on
// every event, so the cost is one credential lookup per connection per ping
// interval whatever the event rate. That leaves a staleness window of one ping
// interval, 30 seconds by default: a revocation lands within that, not at the
// end of the session lifetime.
func (s *EventStream) writeLoop(ctx context.Context, ws *websocket.Conn, c *wsConn, revalidate Revalidate) {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	for {
		select {
		case m := <-c.send:
			if err := s.write(ctx, ws, m); err != nil {
				return
			}
		case <-c.closed:
			reason := c.failure()
			_ = s.write(ctx, ws, errorMessage("", core.Precondition("%s", reason)))
			_ = ws.Close(websocket.StatusPolicyViolation, truncateReason(reason))
			return
		case <-ticker.C:
			if !stillValid(ctx, c, revalidate) {
				_ = s.write(ctx, ws, errorMessage("", core.Unauthenticated("%s", closeCredentialGone)))
				_ = ws.Close(websocket.StatusPolicyViolation, truncateReason(closeCredentialGone))
				return
			}
			pingCtx, cancel := context.WithTimeout(ctx, s.pingTimeout)
			err := ws.Ping(pingCtx)
			cancel()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

// write emits one server message under a write deadline.
func (s *EventStream) write(ctx context.Context, ws *websocket.Conn, m ServerMessage) error {
	b, err := json.Marshal(m)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, s.writeTimeout)
	defer cancel()
	return ws.Write(ctx, websocket.MessageText, b)
}

// eventMessage wraps an event for the subscription that matched it.
func eventMessage(subID string, e core.Event) ServerMessage {
	return ServerMessage{Type: MsgEvent, ID: subID, Event: &e}
}

// errorMessage renders err with the same taxonomy the REST envelope uses.
func errorMessage(subID string, err error) ServerMessage {
	kind := core.KindOf(err)
	body := wire.ErrorBody{Code: kind, Message: err.Error()}

	var domain *core.Error
	if errors.As(err, &domain) {
		body.Message = domain.Message
		body.Details = domain.Details
	}
	if kind == core.KindInternal {
		body = wire.ErrorBody{Code: core.KindInternal, Message: "internal error"}
	}
	return ServerMessage{Type: MsgError, ID: subID, Error: &body}
}

// validateEventTypes rejects a filter naming a type outside the closed set.
func validateEventTypes(types []core.EventType) error {
	for _, t := range types {
		if !knownEventType(t) {
			return core.Invalid("unknown event type %q", t)
		}
	}
	return nil
}

// knownEventType reports whether a literal or wildcard pattern can ever match.
func knownEventType(pattern core.EventType) bool {
	if pattern == "*" {
		return true
	}
	prefix, wildcard := strings.CutSuffix(string(pattern), "*")
	for _, known := range knownEventTypes {
		if wildcard && strings.HasPrefix(string(known), prefix) {
			return true
		}
		if !wildcard && known == pattern {
			return true
		}
	}
	return false
}

// containsProject reports whether the list names the project.
func containsProject(ids []string, id string) bool {
	for _, got := range ids {
		if got == id {
			return true
		}
	}
	return false
}

// truncateReason keeps a close reason inside the protocol's 123 byte limit.
func truncateReason(s string) string {
	if len(s) <= 120 {
		return s
	}
	return s[:120]
}
