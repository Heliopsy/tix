// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
)

// Event emitted when an administrator ends a live connection.
const eventConnectionEnded core.EventType = "connection.ended"

// Audit action recorded when an administrator ends a live connection.
const auditConnectionEnd = "connection.end"

// endReason is what the holder of an ended connection is told. It names no
// administrator, because the holder is not owed the caller's identity; the
// audit entry carries that.
const endReason = "this connection was ended by an administrator; reconnect if you still hold valid credentials"

// ListConnections returns the connections of the caller's tenant that this
// server holds, with the counts that go with them.
func (l *Local) ListConnections(ctx context.Context) (*core.ConnectionList, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	return &core.ConnectionList{
		ServerID:    l.conns.ServerID(),
		Connections: l.conns.List(actor.TenantID),
		Counts:      l.conns.Counts(actor.TenantID),
	}, nil
}

// EndConnection closes one live connection of the caller's tenant.
//
// It writes no domain rows, because the thing being changed is a socket, but
// it still writes the audit entry and the event, and they commit before the
// socket is closed: a record of a cut that did not happen is a smaller problem
// than a cut with no record.
//
// Ending is not revocation. The holder may reconnect at once with credentials
// that are still valid; revoke the credential to stop that.
func (l *Local) EndConnection(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return core.Invalid("connection identifier is required")
	}
	// A connection of another tenant is absent, not refused. Refusing would
	// confirm that the identifier names something.
	conn, ok := l.conns.Lookup(actor.TenantID, id)
	if !ok {
		return core.NotFound("no live connection %q on this server", id)
	}

	if err := l.write(ctx, actor, func(m *mutation) error {
		return m.Record(auditConnectionEnd, eventConnectionEnded, "connection", conn.ID, "", nil,
			map[string]any{
				"id":       conn.ID,
				"surface":  conn.Surface,
				"actor_id": conn.ActorID,
				"since":    conn.Since,
			},
			map[string]any{"connection_id": conn.ID, "surface": conn.Surface, "ended_actor_id": conn.ActorID})
	}); err != nil {
		return err
	}
	return l.conns.End(actor.TenantID, conn.ID, endReason)
}
