// Package connections tracks the live connections one server process holds.
//
// Nothing here is persisted. A connection exists only while the process that
// accepted it runs, so a restart empties the registry, which is correct: the
// connections are gone too.
package connections

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"sort"
	"sync"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
)

// CloseFunc ends one live connection, telling its holder why. It is supplied
// by whichever surface accepted the connection, because only that surface
// knows how to say goodbye on its own protocol.
type CloseFunc func(reason string) error

// Entry describes a connection at the moment it is registered. Since is filled
// from the registry's clock when it is left zero.
type Entry struct {
	Surface     core.ConnectionSurface
	TenantID    string
	ActorID     string
	ActorHandle string
	Remote      string
	Fingerprint string
}

// Registry holds what one process is serving right now.
type Registry struct {
	serverID string
	clock    clock.Clock
	ids      id.Generator

	mu   sync.RWMutex
	live map[string]record
}

// record is one registered connection and the way to end it.
type record struct {
	conn  core.Connection
	close CloseFunc
}

// Option configures a Registry.
type Option func(*Registry)

// WithServerID names the process in every answer it gives.
func WithServerID(name string) Option {
	return func(r *Registry) {
		if name != "" {
			r.serverID = name
		}
	}
}

// WithClock sets the time source a connection's start is read from.
func WithClock(c clock.Clock) Option {
	return func(r *Registry) {
		if c != nil {
			r.clock = c
			r.ids = id.NewGenerator(c)
		}
	}
}

// New builds an empty registry.
func New(opts ...Option) *Registry {
	r := &Registry{serverID: defaultServerID(), clock: clock.New(), live: map[string]record{}}
	r.ids = id.NewGenerator(r.clock)
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Default is the registry the surfaces of this process share.
//
// A connection is held by one process, so one process has one registry. The
// feeders and the service find it here rather than having it threaded through
// wiring they do not own.
var Default = New()

// defaultServerID names this process well enough to tell it apart from another
// on the same host, since a hostname alone does not.
func defaultServerID() string {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "server"
	}
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return host
	}
	return host + "-" + hex.EncodeToString(b[:])
}

// ServerID names the process answering.
func (r *Registry) ServerID() string { return r.serverID }

// Register records a live connection and returns the handle its feeder holds
// for as long as it lasts. close is called at most once, by End.
func (r *Registry) Register(e Entry, onClose CloseFunc) *Handle {
	conn := core.Connection{
		ID:          r.ids.New(),
		Surface:     e.Surface,
		TenantID:    e.TenantID,
		ActorID:     e.ActorID,
		ActorHandle: e.ActorHandle,
		Remote:      e.Remote,
		Since:       r.clock.Now(),
		Fingerprint: e.Fingerprint,
	}
	r.mu.Lock()
	r.live[conn.ID] = record{conn: conn, close: onClose}
	r.mu.Unlock()
	return &Handle{reg: r, id: conn.ID}
}

// Unregister forgets a connection without closing it, for a socket that has
// already gone.
func (r *Registry) Unregister(connID string) {
	r.mu.Lock()
	delete(r.live, connID)
	r.mu.Unlock()
}

// List returns the connections of one tenant, oldest first.
func (r *Registry) List(tenantID string) []core.Connection {
	out := []core.Connection{}
	if tenantID == "" {
		return out
	}
	r.mu.RLock()
	for _, rec := range r.live {
		if rec.conn.TenantID == tenantID {
			out = append(out, rec.conn)
		}
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].Since.Equal(out[j].Since) {
			return out[i].ID < out[j].ID
		}
		return out[i].Since.Before(out[j].Since)
	})
	return out
}

// BySurface returns every live connection on one surface, across tenants.
//
// This is the one unscoped read here, and it exists for a listener shutting
// down: a listener owns its own sessions and must reach all of them to say why
// they are ending, whoever they belong to. It is deliberately not reachable
// from any request path. Nothing tenant-facing may call it, and the tenant
// surfaces above take a tenant and refuse an empty one.
func (r *Registry) BySurface(surface core.ConnectionSurface) []core.Connection {
	out := []core.Connection{}
	r.mu.RLock()
	for _, rec := range r.live {
		if rec.conn.Surface == surface {
			out = append(out, rec.conn)
		}
	}
	r.mu.RUnlock()
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Counts reports one tenant's connections by surface beside the process total.
// The total carries no breakdown, by design.
func (r *Registry) Counts(tenantID string) core.ConnectionCounts {
	var out core.ConnectionCounts
	r.mu.RLock()
	defer r.mu.RUnlock()
	out.Process = len(r.live)
	if tenantID == "" {
		return out
	}
	for _, rec := range r.live {
		if rec.conn.TenantID != tenantID {
			continue
		}
		out.Tenant++
		switch rec.conn.Surface {
		case core.ConnectionEvents:
			out.Events++
		case core.ConnectionSSH:
			out.SSH++
		}
	}
	return out
}

// Lookup returns a live connection of one tenant. A connection of any other
// tenant is reported as absent, never as refused: refusing would confirm that
// the identifier names something.
func (r *Registry) Lookup(tenantID, connID string) (core.Connection, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rec, ok := r.live[connID]
	if !ok || tenantID == "" || rec.conn.TenantID != tenantID {
		return core.Connection{}, false
	}
	return rec.conn, true
}

// End closes one live connection of a tenant, telling its holder why. A
// connection that has already gone is not an error: the caller wanted it gone
// and it is.
func (r *Registry) End(tenantID, connID, reason string) error {
	r.mu.Lock()
	rec, ok := r.live[connID]
	if !ok || tenantID == "" || rec.conn.TenantID != tenantID {
		r.mu.Unlock()
		return nil
	}
	delete(r.live, connID)
	r.mu.Unlock()
	if rec.close == nil {
		return nil
	}
	return rec.close(reason)
}

// Len reports how many connections the process is holding.
func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.live)
}

// Handle is a feeder's grip on one registered connection.
type Handle struct {
	reg  *Registry
	id   string
	once sync.Once
}

// ID returns the identifier the connection is addressed by.
func (h *Handle) ID() string {
	if h == nil {
		return ""
	}
	return h.id
}

// Unregister takes the connection out of the registry. It is idempotent, so a
// feeder may call it from a deferred close and from an error path both.
func (h *Handle) Unregister() {
	if h == nil || h.reg == nil {
		return
	}
	h.once.Do(func() { h.reg.Unregister(h.id) })
}
