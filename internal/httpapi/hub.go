// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"context"
	"net"
	"net/http"
	"sync"

	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
)

// defaultSendQueue bounds the messages one connection may have awaiting a write.
const defaultSendQueue = 256

// maxPendingEvents bounds the live events a subscription may buffer while it replays history.
const maxPendingEvents = 1024

// Hub fans committed events out to the connections entitled to receive them.
type Hub struct {
	// hookMu is held across a tenant count transition and the hook that
	// transition triggers, so two connections changing the same tenant can
	// never deliver their hooks in the opposite order to their counts. The
	// hooks themselves run outside mu: they are caller-supplied, they may
	// block, and a hook that read the hub back under mu would deadlock.
	hookMu sync.Mutex

	mu      sync.RWMutex
	conns   map[*wsConn]struct{}
	tenants map[string]int

	// live is the process registry an administrator lists and ends
	// connections through. The hub already knows on upgrade and on close
	// exactly what the registry wants to be told, so it is fed from those
	// two points rather than from a second tracking path.
	live *connections.Registry

	onFirst func(tenantID string)
	onLast  func(tenantID string)
}

// NewHub builds an empty hub feeding the process-wide connection registry.
func NewHub() *Hub {
	return &Hub{
		conns:   make(map[*wsConn]struct{}),
		tenants: make(map[string]int),
		live:    connections.Default,
	}
}

// SetRegistry sends this hub's connections to another registry. A process has
// one registry, so this exists for a test that wants its own and for a caller
// assembling a server explicitly.
func (h *Hub) SetRegistry(r *connections.Registry) {
	if r == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.live = r
}

// SetTenantHooks registers callbacks fired when a tenant gains its first
// connection and loses its last. Events are read per tenant, so a caller uses
// these to run exactly one reader per tenant that anyone is listening to.
// A hook must not register or unregister a connection itself.
func (h *Hub) SetTenantHooks(onFirst, onLast func(tenantID string)) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.onFirst, h.onLast = onFirst, onLast
}

// Register adds a connection to the fan-out set.
func (h *Hub) Register(c *wsConn) {
	h.hookMu.Lock()
	defer h.hookMu.Unlock()

	h.mu.Lock()
	h.conns[c] = struct{}{}
	tenant, first, hook := h.trackLocked(c, 1)
	registry := h.live
	h.mu.Unlock()
	c.handle = registerLive(registry, c)
	if first && hook != nil {
		hook(tenant)
	}
}

// registerLive records the connection in the registry, if it speaks for
// anybody. An unauthenticated connection is not something an administrator can
// be shown or asked to end.
func registerLive(r *connections.Registry, c *wsConn) *connections.Handle {
	if r == nil || c == nil || c.actor == nil || c.actor.TenantID == "" {
		return nil
	}
	return r.Register(connections.Entry{
		Surface:     core.ConnectionEvents,
		TenantID:    c.actor.TenantID,
		ActorID:     c.actor.ID,
		ActorHandle: c.actor.Handle,
		Remote:      c.remote,
	}, func(reason string) error {
		c.fail(reason)
		return nil
	})
}

// Unregister removes a connection and discards its subscriptions.
func (h *Hub) Unregister(c *wsConn) {
	h.hookMu.Lock()
	h.mu.Lock()
	delete(h.conns, c)
	tenant, last, hook := h.trackLocked(c, -1)
	h.mu.Unlock()
	if last && hook != nil {
		hook(tenant)
	}
	h.hookMu.Unlock()

	c.handle.Unregister()
	c.dropSubs()
}

// trackLocked adjusts the per-tenant count and reports whether the count
// crossed zero. The caller must hold h.mu.
func (h *Hub) trackLocked(c *wsConn, delta int) (tenant string, crossed bool, hook func(string)) {
	if c == nil || c.actor == nil || c.actor.TenantID == "" {
		return "", false, nil
	}
	tenant = c.actor.TenantID
	before := h.tenants[tenant]
	after := before + delta
	if after <= 0 {
		delete(h.tenants, tenant)
		return tenant, before > 0, h.onLast
	}
	h.tenants[tenant] = after
	return tenant, before == 0, h.onFirst
}

// tenantCount reports how many connections the tenant currently has.
func (h *Hub) tenantCount(tenantID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.tenants[tenantID]
}

// remoteOf names where a request came from, believing a forwarded address only
// when the middleware already decided the peer was a trusted proxy.
func remoteOf(r *http.Request) string {
	if addr := ClientIPFrom(r.Context()); addr.IsValid() {
		return addr.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// Len reports how many connections are registered.
func (h *Hub) Len() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.conns)
}

// Broadcast delivers an event to every entitled subscription without ever blocking on one connection.
func (h *Hub) Broadcast(e core.Event) {
	for _, c := range h.snapshot() {
		c.route(e)
	}
}

// Pump broadcasts every event from src until it closes or ctx is cancelled.
func (h *Hub) Pump(ctx context.Context, src <-chan core.Event) {
	for {
		select {
		case e, ok := <-src:
			if !ok {
				return
			}
			h.Broadcast(e)
		case <-ctx.Done():
			return
		}
	}
}

// snapshot copies the connection set so fan-out never holds the hub lock.
func (h *Hub) snapshot() []*wsConn {
	h.mu.RLock()
	defer h.mu.RUnlock()
	out := make([]*wsConn, 0, len(h.conns))
	for c := range h.conns {
		out = append(out, c)
	}
	return out
}

// wsConn is one authenticated event-stream connection and the subscriptions held on it.
type wsConn struct {
	actor  *core.Actor
	remote string
	send   chan ServerMessage

	// handle is this connection's place in the process registry, held from
	// the hub's Register to its Unregister.
	handle *connections.Handle

	mu   sync.Mutex
	subs map[string]*subscription

	closeOnce sync.Once
	closed    chan struct{}
	reason    string
}

// newConn builds a connection with a bounded outbound queue.
func newConn(a *core.Actor, queue int) *wsConn {
	if queue <= 0 {
		queue = defaultSendQueue
	}
	return &wsConn{
		actor:  a,
		send:   make(chan ServerMessage, queue),
		subs:   make(map[string]*subscription),
		closed: make(chan struct{}),
	}
}

// entitled reports whether the connection's credential may see the event at all.
func (c *wsConn) entitled(e core.Event) bool {
	if c.actor == nil || c.actor.TenantID == "" || c.actor.TenantID != e.TenantID {
		return false
	}
	if !c.actor.HasScope(core.ScopeEventSubscribe) {
		return false
	}
	return !c.actor.ScopedToProject() || c.actor.ProjectID == e.ProjectID
}

// route offers an event to every matching subscription on the connection.
func (c *wsConn) route(e core.Event) {
	if !c.entitled(e) {
		return
	}
	for _, s := range c.snapshotSubs() {
		if !s.filter.Matches(e) {
			continue
		}
		if !s.send(c, e) {
			return
		}
	}
}

// addSub registers a subscription that buffers events until it is activated.
func (c *wsConn) addSub(id string, f core.EventFilter) (*subscription, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.subs[id]; ok {
		return nil, core.Conflict("subscription %q already exists on this connection", id)
	}
	s := &subscription{id: id, filter: f, maxSeq: f.SinceSeq}
	c.subs[id] = s
	return s, nil
}

// removeSub drops a subscription and reports whether it existed.
func (c *wsConn) removeSub(id string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.subs[id]; !ok {
		return false
	}
	delete(c.subs, id)
	return true
}

// subByID returns the named subscription, or nil.
func (c *wsConn) subByID(id string) *subscription {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.subs[id]
}

// dropSubs releases every subscription held on the connection.
func (c *wsConn) dropSubs() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.subs = make(map[string]*subscription)
}

// snapshotSubs copies the subscription set so delivery never holds the connection lock.
func (c *wsConn) snapshotSubs() []*subscription {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]*subscription, 0, len(c.subs))
	for _, s := range c.subs {
		out = append(out, s)
	}
	return out
}

// enqueue queues a message, failing the connection rather than blocking a broadcast.
func (c *wsConn) enqueue(m ServerMessage) bool {
	select {
	case <-c.closed:
		return false
	default:
	}
	select {
	case c.send <- m:
		return true
	default:
		c.fail("slow consumer: outbound queue full, resume from your last seq")
		return false
	}
}

// fail marks the connection for closure with the first reason recorded.
func (c *wsConn) fail(reason string) {
	c.closeOnce.Do(func() {
		c.reason = reason
		close(c.closed)
	})
}

// failure returns the recorded closure reason, empty while the connection is healthy.
func (c *wsConn) failure() string {
	select {
	case <-c.closed:
		return c.reason
	default:
		return ""
	}
}

// subscription is one filter on a connection plus the cursor of what it has sent.
type subscription struct {
	id     string
	filter core.EventFilter

	mu      sync.Mutex
	live    bool
	maxSeq  int64
	pending []core.Event
}

// send delivers an event, buffering it while the subscription is still replaying history.
func (s *subscription) send(c *wsConn, e core.Event) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.live {
		if len(s.pending) >= maxPendingEvents {
			c.fail("slow consumer: replay backlog exceeded, resume from your last seq")
			return false
		}
		s.pending = append(s.pending, e)
		return true
	}
	if e.Seq <= s.maxSeq {
		return true
	}
	s.maxSeq = e.Seq
	return c.enqueue(eventMessage(s.id, e))
}

// note records that an event was already delivered during replay.
func (s *subscription) note(seq int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if seq > s.maxSeq {
		s.maxSeq = seq
	}
}

// at returns the highest sequence number the subscription has delivered.
func (s *subscription) at() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.maxSeq
}

// activate flushes what arrived during replay and switches to live delivery.
func (s *subscription) activate(c *wsConn) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	buffered := s.pending
	s.pending = nil
	s.live = true
	for _, e := range buffered {
		if e.Seq <= s.maxSeq {
			continue
		}
		s.maxSeq = e.Seq
		if !c.enqueue(eventMessage(s.id, e)) {
			return false
		}
	}
	return true
}
