// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// auditSessionRevoke records a person ending one of their sessions by
// identifier, which is not necessarily the one they presented.
const auditSessionRevoke = "session.revoke"

// sessionOwner returns the caller when it may manage its own sessions.
func (l *Local) sessionOwner(ctx context.Context) (*core.Actor, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.policy.CanManageSessions(actor); err != nil {
		return nil, err
	}
	return actor, nil
}

// ListSessions lists the caller's own unexpired sessions in this tenant,
// newest first, marking the one presented with this request.
func (l *Local) ListSessions(ctx context.Context, page core.Page) ([]core.SessionInfo, string, error) {
	actor, err := l.sessionOwner(ctx)
	if err != nil {
		return nil, "", err
	}
	if page.Sort == "" {
		page.Sort = core.SortCreatedAt
	}
	if page.Sort != core.SortCreatedAt {
		return nil, "", core.Invalid("sessions sort by %s only", core.SortCreatedAt)
	}
	if page.Direction == "" {
		page.Direction = core.Descending
	}
	if page, err = page.Normalize(); err != nil {
		return nil, "", err
	}
	if err := mustCursorMatch(page); err != nil {
		return nil, "", err
	}
	presented := ""
	if token, ok := SessionTokenFrom(ctx); ok {
		presented = auth.HashToken(token)
	}
	out := []core.SessionInfo{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListActorSessions(ctx, actor.ID, l.clock.Now(), page)
		if err != nil {
			return err
		}
		for _, r := range found {
			info := r.SessionInfo
			info.Current = presented != "" && r.TokenHash == presented
			out = append(out, info)
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > 0 && len(out) >= page.Limit {
		last := out[len(out)-1]
		next = core.Cursor{SortValue: sqlb.TimeText(last.CreatedAt), ID: last.ID,
			Sort: page.Sort, Direction: page.Direction}.Encode()
	}
	return out, next, nil
}

// RevokeSession ends one of the caller's own sessions. Another actor's
// session is not found, the same answer as one that does not exist.
func (l *Local) RevokeSession(ctx context.Context, id string) error {
	actor, err := l.sessionOwner(ctx)
	if err != nil {
		return err
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return core.Invalid("session identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		if err := m.tx.DeleteActorSession(ctx, actor.ID, id); err != nil {
			return err
		}
		return m.Record(auditSessionRevoke, eventSessionEnded, "session", id, "",
			map[string]any{"id": id, "actor_id": actor.ID}, nil,
			map[string]any{"session_id": id, "actor_id": actor.ID})
	})
}
