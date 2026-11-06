// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Event types and audit actions for global admin grants.
const (
	eventGlobalAdminGranted core.EventType = "global_admin.granted"
	eventGlobalAdminRevoked core.EventType = "global_admin.revoked"

	auditGlobalAdminGrant  = "global_admin.grant"
	auditGlobalAdminRevoke = "global_admin.revoke"
)

// authorizeGlobal resolves the actor and refuses anyone who is not a global
// admin right now. The flag is read from the store on every call, so a revoke
// reaches a live session or token at once.
func (l *Local) authorizeGlobal(ctx context.Context) (*core.Actor, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	standing := authz.GlobalStanding{DirectStore: core.DirectStoreFrom(ctx)}
	if !standing.DirectStore || actor.TokenID != "" {
		if standing.GlobalAdmin, err = l.isGlobalAdmin(ctx, actor); err != nil {
			return nil, err
		}
	}
	if err := l.policy.CanGlobal(actor, standing); err != nil {
		return nil, err
	}
	return actor, nil
}

// globalAllowed reports whether the caller may act as a global admin, for the
// operations that widen for one rather than refuse everybody else.
func (l *Local) globalAllowed(ctx context.Context) (bool, error) {
	_, err := l.authorizeGlobal(ctx)
	switch {
	case err == nil:
		return true, nil
	case core.IsKind(err, core.KindForbidden):
		return false, nil
	default:
		return false, err
	}
}

// isGlobalAdmin reports whether the actor is a user who is a global admin and
// is not disabled. An actor with no user row, an agent or the local actor, is
// not one.
func (l *Local) isGlobalAdmin(ctx context.Context, actor *core.Actor) (bool, error) {
	var out bool
	err := l.read(ctx, actor, func(tx store.Tx) error {
		u, err := tx.GetUser(ctx, actor.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		out = u.GlobalAdmin && u.DisabledAt == nil
		return nil
	})
	return out, err
}

// GrantGlobalAdmin makes a user a global admin. Granting it to somebody who
// already holds it changes nothing and records nothing.
func (l *Local) GrantGlobalAdmin(ctx context.Context, in core.GlobalAdminInput) (*core.User, error) {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return nil, err
	}
	if err := core.ValidateEmail(in.Email); err != nil {
		return nil, err
	}
	email := core.NormalEmail(in.Email)

	var out *core.User
	err = l.write(ctx, actor, func(m *mutation) error {
		u, err := userByEmail(ctx, m.tx, email)
		if err != nil {
			return err
		}
		out = u
		if u.GlobalAdmin {
			return nil
		}
		return setGlobalAdmin(ctx, m, u, true, auditGlobalAdminGrant, eventGlobalAdminGranted)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// RevokeGlobalAdmin takes global admin away. The last enabled global admin
// cannot be revoked, so the installation is never left with nobody who can
// create a tenant short of a shell on the server.
func (l *Local) RevokeGlobalAdmin(ctx context.Context, email string) error {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return err
	}
	if err := core.ValidateEmail(email); err != nil {
		return err
	}
	email = core.NormalEmail(email)

	return l.write(ctx, actor, func(m *mutation) error {
		u, err := userByEmail(ctx, m.tx, email)
		if err != nil {
			return err
		}
		if !u.GlobalAdmin {
			return core.NotFound("global admin %q", email)
		}
		admins, err := globalAdmins(ctx, m.tx)
		if err != nil {
			return err
		}
		others := 0
		for _, a := range admins {
			if a.ID != u.ID && a.DisabledAt == nil {
				others++
			}
		}
		if others == 0 {
			return core.Precondition("%s is the last global admin; grant another before revoking this one", email).
				WithDetail("reason", "last_global_admin")
		}
		return setGlobalAdmin(ctx, m, u, false, auditGlobalAdminRevoke, eventGlobalAdminRevoked)
	})
}

// ListGlobalAdmins returns every global admin, disabled ones included so the
// listing says who holds the flag rather than who can use it today.
func (l *Local) ListGlobalAdmins(ctx context.Context) ([]core.User, error) {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return nil, err
	}
	var out []core.User
	err = l.read(ctx, actor, func(tx store.Tx) error {
		found, err := globalAdmins(ctx, tx)
		out = found
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// userByEmail resolves a user anywhere in the installation by address.
func userByEmail(ctx context.Context, tx store.Tx, email string) (*core.User, error) {
	u, err := asUnscoped(tx)
	if err != nil {
		return nil, err
	}
	found, _, err := u.GetUserByEmail(ctx, email)
	if core.IsKind(err, core.KindNotFound) {
		return nil, core.NotFound("user %q", email)
	}
	return found, err
}

// setGlobalAdmin writes the flag and records the change.
func setGlobalAdmin(ctx context.Context, m *mutation, u *core.User, on bool, action string, typ core.EventType) error {
	before := *u
	if err := m.tx.SetUserGlobalAdmin(ctx, u.ID, on); err != nil {
		return err
	}
	u.GlobalAdmin = on
	return m.Record(action, typ, "user", u.ID, "", before, u, map[string]any{"email": u.Email})
}

// globalAdmins walks the global users table a page at a time and keeps the
// global admins. The table carries no index on the flag; the walk is bounded by
// the number of users and runs only on an administrative call.
func globalAdmins(ctx context.Context, tx store.Tx) ([]core.User, error) {
	out := []core.User{}
	page := core.Page{Sort: core.SortCreatedAt, Limit: core.MaxPageLimit}
	page, err := page.Normalize()
	if err != nil {
		return nil, err
	}
	for {
		found, err := tx.ListUsers(ctx, page)
		if err != nil {
			return nil, err
		}
		for _, u := range found {
			if u.GlobalAdmin {
				out = append(out, u)
			}
		}
		if len(found) < page.Limit {
			return out, nil
		}
		page.Cursor = userCursor(page, found[len(found)-1]).Encode()
	}
}
