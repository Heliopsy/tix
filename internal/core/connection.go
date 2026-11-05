// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"context"
	"time"
)

// ConnectionSurface names the access path a live connection arrived on.
type ConnectionSurface string

// The surfaces a live connection can arrive on. These are connections, not
// sessions: Session is a login backed by a row, and one actor may hold one
// session and several connections at once.
const (
	ConnectionEvents ConnectionSurface = "events"
	ConnectionSSH    ConnectionSurface = "ssh"
)

// ConnectionSurfaces lists every surface a connection can arrive on.
var ConnectionSurfaces = []ConnectionSurface{ConnectionEvents, ConnectionSSH}

// Valid reports whether s is a surface this build serves.
func (s ConnectionSurface) Valid() bool {
	for _, known := range ConnectionSurfaces {
		if s == known {
			return true
		}
	}
	return false
}

// Connection is one connection a server process is holding right now. Nothing
// about it is stored: it is true only while the process runs.
type Connection struct {
	// ID identifies the connection within the answering server, for ending it.
	ID string `json:"id" yaml:"id"`
	// Surface is the access path the connection arrived on.
	Surface ConnectionSurface `json:"surface" yaml:"surface"`
	// TenantID identifies the tenant the connection reads.
	TenantID string `json:"tenant_id" yaml:"tenant_id"`
	// ActorID is the actor the connection authenticated as.
	ActorID string `json:"actor_id" yaml:"actor_id"`
	// ActorHandle is that actor's handle.
	ActorHandle string `json:"actor_handle,omitempty" yaml:"actor_handle,omitempty"`
	// Remote is the client address the connection came from, without its port.
	Remote string `json:"remote,omitempty" yaml:"remote,omitempty"`
	// Since is when the connection opened, RFC 3339.
	Since time.Time `json:"since" yaml:"since"`
	// Fingerprint names the key an SSH connection opened with, and is empty
	// on every other surface.
	Fingerprint string `json:"fingerprint,omitempty" yaml:"fingerprint,omitempty"`
}

// ConnectionCounts reports how many connections are live. The per-surface
// figures and Tenant cover the caller's own tenant. Process covers every
// connection the server holds and deliberately carries no breakdown by tenant,
// actor or address: it is capacity information, not a directory.
type ConnectionCounts struct {
	// Events counts the tenant's live event stream connections.
	Events int `json:"events" yaml:"events"`
	// SSH counts the tenant's live SSH connections.
	SSH int `json:"ssh" yaml:"ssh"`
	// Tenant counts all of the tenant's live connections.
	Tenant int `json:"tenant" yaml:"tenant"`
	// Process counts every connection this server holds, across all tenants.
	Process int `json:"process" yaml:"process"`
}

// ConnectionList is one server's answer about what it is holding. ServerID
// names the process that answered, because a deployment running several
// servers against one database gets a partial view from each.
type ConnectionList struct {
	// ServerID names the server process that answered.
	ServerID string `json:"server_id" yaml:"server_id"`
	// Connections are the caller's tenant's live connections on that server.
	Connections []Connection `json:"connections" yaml:"connections"`
	// Counts are the totals that go with the list.
	Counts ConnectionCounts `json:"counts" yaml:"counts"`
}

// ConnectionService covers the live connections one server process holds.
type ConnectionService interface {
	// ListConnections returns the connections of the caller's tenant that this
	// server is holding, with the counts that go with them.
	ListConnections(ctx context.Context) (*ConnectionList, error)
	// EndConnection closes one live connection of the caller's tenant. It is
	// not revocation: the holder may reconnect with credentials that are still
	// valid.
	EndConnection(ctx context.Context, id string) error
}
