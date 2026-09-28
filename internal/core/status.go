// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"time"
)

// StatusService reports what the installation holds and what is running in it.
type StatusService interface {
	Status(ctx context.Context) (*StatusReport, error)
}

// Server liveness constants.
const (
	// ServerHeartbeatInterval is how often a running server refreshes its row.
	ServerHeartbeatInterval = 30 * time.Second

	// ServerStaleAfter is how long a server may go unseen before a reader
	// calls it gone. It is three heartbeats: one missed beat is a busy machine
	// or a slow write, three is a process that has stopped.
	//
	// The judgement is the reader's and needs no sweeper to have run, exactly
	// as a lease expiry is authoritative the moment it passes. That matters
	// because the case this table exists for is the one where nothing tidied
	// up: a graceful shutdown deletes its own row, so every row a reader has to
	// judge belongs to a process that crashed, was killed, or is unreachable.
	ServerStaleAfter = 3 * ServerHeartbeatInterval

	// ServerForgetAfter is how long an unseen row is kept before a registering
	// server deletes it. This is hygiene, not correctness: a row that outlives
	// it still reads as stale, and a purge that never runs changes nothing a
	// reader sees.
	ServerForgetAfter = 7 * 24 * time.Hour
)

// ServerSurface names a surface a server process serves.
type ServerSurface string

// The surfaces a server can serve.
const (
	ServerSurfaceAPI ServerSurface = "api"
	ServerSurfaceWS  ServerSurface = "ws"
	ServerSurfaceWeb ServerSurface = "web"
	ServerSurfaceSSH ServerSurface = "ssh"
)

// ServerSurfaces lists every surface a server can serve, in the order a reader
// should see them.
var ServerSurfaces = []ServerSurface{
	ServerSurfaceAPI, ServerSurfaceWS, ServerSurfaceWeb, ServerSurfaceSSH,
}

// Valid reports whether s is a surface this build serves.
func (s ServerSurface) Valid() bool {
	for _, known := range ServerSurfaces {
		if s == known {
			return true
		}
	}
	return false
}

// Server is one server process registered against this installation.
//
// Every field here is installation state. Nothing on it names a tenant, a
// tenant key, a hostname that resolves to one, or an actor, which is what
// allows the row to be shown at all: a server serves every tenant, so a row
// that disclosed one would disclose it to all the others.
//
// Connections are deliberately absent. A connection is held in one process's
// memory and a stored count would be true only inside that process and only
// until the next socket closed, so the count reaches a reader from the process
// that has it or not at all.
type Server struct {
	ID       string          `json:"id" yaml:"id"`
	Address  string          `json:"address" yaml:"address"`
	Version  string          `json:"version" yaml:"version"`
	Surfaces []ServerSurface `json:"surfaces" yaml:"surfaces"`
	// StartedAt is when the process registered, which is its uptime.
	StartedAt time.Time `json:"started_at" yaml:"started_at"`
	// LastSeenAt is the last heartbeat. Attached is derived from it.
	LastSeenAt time.Time `json:"last_seen_at" yaml:"last_seen_at"`
}

// Attached reports whether the server was heartbeating as of now.
//
// It takes the instant rather than reading a clock, both because this package
// is stdlib only and because a judgement a caller can pin is a judgement a test
// can make without waiting.
func (s Server) Attached(now time.Time) bool {
	return now.Sub(s.LastSeenAt) <= ServerStaleAfter
}

// UptimeAt reports how long the server has been running as of now.
func (s Server) UptimeAt(now time.Time) Duration {
	if s.StartedAt.IsZero() || now.Before(s.StartedAt) {
		return 0
	}
	return Duration(now.Sub(s.StartedAt))
}

// StatusAt renders the row as a reader receives it, judged against one instant.
func (s Server) StatusAt(now time.Time) ServerStatus {
	return ServerStatus{
		ID:         s.ID,
		Address:    s.Address,
		Version:    s.Version,
		Surfaces:   s.Surfaces,
		StartedAt:  s.StartedAt,
		LastSeenAt: s.LastSeenAt,
		Uptime:     s.UptimeAt(now),
		Attached:   s.Attached(now),
	}
}

// ServerStatus is one server as a reader receives it: the stored row spelled
// out, plus the two things only a reader can supply.
//
// The row's fields are repeated rather than embedded because an embedded
// struct inlines by default in one encoder and nests in the other, and the JSON
// shape here is a contract that should not depend on which of the two somebody
// reads it through.
type ServerStatus struct {
	ID         string          `json:"id" yaml:"id"`
	Address    string          `json:"address" yaml:"address"`
	Version    string          `json:"version" yaml:"version"`
	Surfaces   []ServerSurface `json:"surfaces" yaml:"surfaces"`
	StartedAt  time.Time       `json:"started_at" yaml:"started_at"`
	LastSeenAt time.Time       `json:"last_seen_at" yaml:"last_seen_at"`

	// Uptime is how long the process had been running when the report was
	// taken, so a reader need not subtract two instants to learn it.
	Uptime Duration `json:"uptime" yaml:"uptime"`

	// Attached is the reader's own staleness judgement, carried in the
	// document so that no consumer has to know the threshold. Two consumers
	// deriving it separately is how two dashboards come to disagree.
	Attached bool `json:"attached" yaml:"attached"`

	// Connections is how many connections this server is holding, and is set
	// only for the server that produced the report. A nil is "not known",
	// which is not the same claim as zero, so it is omitted rather than
	// rendered.
	Connections *int `json:"connections,omitempty" yaml:"connections,omitempty"`
}

// Installation describes the store the report was taken against.
type Installation struct {
	Version       string `json:"version" yaml:"version"`
	SchemaVersion int    `json:"schema_version" yaml:"schema_version"`
	// Store is the resolved target, with any password already redacted.
	Store  string `json:"store,omitempty" yaml:"store,omitempty"`
	Engine string `json:"engine,omitempty" yaml:"engine,omitempty"`
	// ObservedAt is the instant every staleness judgement in this report was
	// made against.
	ObservedAt time.Time `json:"observed_at" yaml:"observed_at"`
}

// Work counts what the caller's own tenant holds, plus the number of tenants
// the caller can see.
//
// Tenants is deliberately what this reader could already list rather than what
// the installation holds. An installation-wide count would tell the
// administrator of one tenant that six others exist, which nothing else in the
// product tells them.
type Work struct {
	Tenants  int `json:"tenants" yaml:"tenants"`
	Projects int `json:"projects" yaml:"projects"`
	Tasks    int `json:"tasks" yaml:"tasks"`
	Claimed  int `json:"claimed" yaml:"claimed"`
	// LeasesExpiredUnswept are claims past their expiry that the sweeper has
	// not yet cleared. They read as unclaimed already; the figure says how much
	// the sweeper is behind.
	LeasesExpiredUnswept int `json:"leases_expired_unswept" yaml:"leases_expired_unswept"`
	WebhooksPending      int `json:"webhooks_pending" yaml:"webhooks_pending"`
	WebhooksFailed       int `json:"webhooks_failed" yaml:"webhooks_failed"`
}

// StatusReport is the whole answer to "what exists and what is running".
type StatusReport struct {
	Installation Installation `json:"installation" yaml:"installation"`
	// Servers is never nil, so a consumer iterating it need not nil-check.
	Servers []ServerStatus `json:"servers" yaml:"servers"`
	Work    Work           `json:"work" yaml:"work"`
}

// AttachedCount reports how many of the listed servers are heartbeating.
func (r StatusReport) AttachedCount() int {
	n := 0
	for _, s := range r.Servers {
		if s.Attached {
			n++
		}
	}
	return n
}
