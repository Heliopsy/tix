// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Event types for the identity records the core vocabulary does not name.
const (
	eventUserCreated core.EventType = "user.created"
	eventUserUpdated core.EventType = "user.updated"
	eventUserDeleted core.EventType = "user.deleted"
)

// Audit actions recorded for users.
const (
	auditUserCreate = "user.create"
	auditUserUpdate = "user.update"
	auditUserDelete = "user.delete"
)

// defaultUserRole is granted when a caller names no role, because the weakest
// role is the only safe default for an account someone else created.
const defaultUserRole = core.RoleViewer

// userHandle derives the actor handle for a user, falling back to the local
// part of the email so that every user is addressable without extra input.
//
// It deliberately does not fall back to a generated name. An address with no
// local part is malformed input, and inventing a handle for it would accept
// the mistake silently; a handle is also a name people type, which a name
// nobody chose is not.
func userHandle(handle, email string) string {
	handle = strings.ToLower(strings.TrimSpace(handle))
	if handle != "" {
		return handle
	}
	local, _, _ := strings.Cut(email, "@")
	return local
}

// ownUser resolves a user of the caller's tenant by its actor identifier and
// returns the account behind it. A user of another tenant is reported missing,
// so no caller can confirm it exists.
//
// The account returned carries its own identifier, which writes to the users
// row need; asMember gives the identifier callers of this tenant see.
func ownUser(ctx context.Context, tx store.Tx, id string) (*core.User, *core.Actor, error) {
	if strings.TrimSpace(id) == "" {
		return nil, nil, core.Invalid("user identifier is required")
	}
	a, err := tx.GetActor(ctx, id)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil, core.NotFound("user %q", id)
		}
		return nil, nil, err
	}
	u, err := accountOf(ctx, tx, a)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil, core.NotFound("user %q", id)
		}
		return nil, nil, err
	}
	return u, a, nil
}

// accountOf returns the account an actor belongs to, through the actor's link
// and never by assuming the two share an identifier. An actor with no account,
// an agent or the local actor, reads as not found.
func accountOf(ctx context.Context, tx store.Tx, a *core.Actor) (*core.User, error) {
	if a.UserID == "" {
		return nil, core.NotFound("user for actor %q", a.ID)
	}
	return tx.GetUser(ctx, a.UserID)
}

// accountOfActor is accountOf for an actor known only by identifier.
func accountOfActor(ctx context.Context, tx store.Tx, actorID string) (*core.User, error) {
	a, err := tx.GetActor(ctx, actorID)
	if err != nil {
		return nil, err
	}
	return accountOf(ctx, tx, a)
}

// asMember presents an account as a user of this tenant, whose identifier is
// the tenant's actor. For every account created before actors carried a link
// the two are equal; for a person in several tenants they are not.
func asMember(u *core.User, a *core.Actor) *core.User {
	out := *u
	out.ID = a.ID
	return &out
}

// userRole returns an actor's role in this tenant, or the empty role when the
// actor holds no membership.
func userRole(ctx context.Context, tx store.Tx, actorID string) (core.Role, error) {
	mem, err := tx.GetMember(ctx, actorID)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return "", nil
		}
		return "", err
	}
	return mem.Role, nil
}

// setUserRole replaces an actor's membership role in this tenant.
func setUserRole(ctx context.Context, tx store.Tx, actorID string, role core.Role) error {
	if !role.Valid() {
		return core.Invalid("role %q must be %q, %q or %q", role, core.RoleViewer, core.RoleMember, core.RoleAdmin)
	}
	switch _, err := tx.GetMember(ctx, actorID); {
	case err == nil:
		if err := tx.RemoveMember(ctx, actorID); err != nil {
			return err
		}
	case !core.IsKind(err, core.KindNotFound):
		return err
	}
	return tx.AddMember(ctx, &core.Membership{ActorID: actorID, Role: role})
}

// userCursor addresses the position just after this user in the ordering.
func userCursor(page core.Page, u core.User) core.Cursor {
	value := sqlb.TimeText(u.CreatedAt)
	if page.Sort == "email" {
		value = u.Email
	}
	return core.Cursor{
		SortValue: value,
		ID:        u.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}
}

// nextUserCursor returns the cursor for the page after n users, the last of
// which is the account last.
//
// A cursor is an opaque token only by convention: it is base64 of the sort key
// and the identifier, so it must never describe a record the caller may not
// read. It is therefore built from the last user being returned, never from
// the unfiltered page underneath them, and a short page ends the listing
// rather than handing back a position in rows the caller cannot see. It holds
// the account's identifier, because the walk it resumes is over accounts.
func nextUserCursor(page core.Page, n int, last core.User) string {
	if n == 0 || n < page.Limit {
		return ""
	}
	return userCursor(page, last).Encode()
}

// CreateUser creates a user, its actor in this tenant, and its membership.
func (l *Local) CreateUser(ctx context.Context, in core.CreateUserInput) (*core.User, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	email := strings.ToLower(strings.TrimSpace(in.Email))
	if email == "" {
		return nil, core.Invalid("email is required")
	}
	handle := userHandle(in.Handle, email)
	if handle == "" {
		return nil, core.Invalid("a user needs a handle")
	}
	role := in.Role
	if role == "" {
		role = defaultUserRole
	}

	hash := ""
	if in.Password != "" {
		if hash, err = l.hasher.Hash(in.Password); err != nil {
			return nil, err
		}
	}

	var out *core.User
	err = l.write(ctx, actor, func(m *mutation) error {
		switch _, err := m.tx.GetActorByHandle(ctx, handle); {
		case err == nil:
			return core.Conflict("handle %q is already in use", handle)
		case !core.IsKind(err, core.KindNotFound):
			return err
		}

		id := l.ids.New()
		u := &core.User{ID: id, Email: email, DisplayName: in.DisplayName,
			RequirePasswordChange: in.RequirePasswordChange}
		if err := m.tx.CreateUser(ctx, u, hash); err != nil {
			return err
		}
		a := &core.Actor{ID: id, UserID: id, Kind: core.ActorUser, Handle: handle, DisplayName: in.DisplayName}
		if err := m.tx.CreateActor(ctx, a); err != nil {
			return err
		}
		if err := setUserRole(ctx, m.tx, id, role); err != nil {
			return err
		}
		out = u
		return m.Record(auditUserCreate, eventUserCreated, "user", id, "", nil, u,
			map[string]any{"email": email, "handle": handle, "role": string(role),
				"require_password_change": u.RequirePasswordChange})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetUser returns a user of this tenant by identifier.
func (l *Local) GetUser(ctx context.Context, id string) (*core.User, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.User
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		u, a, err := ownUser(ctx, tx, id)
		if err != nil {
			return err
		}
		out = asMember(u, a)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListUsers returns the users of this tenant, keyset paginated.
func (l *Local) ListUsers(ctx context.Context, page core.Page) ([]core.User, string, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	if page.Sort == "" {
		page.Sort = core.SortCreatedAt
	}
	page, err = page.Normalize()
	if err != nil {
		return nil, "", err
	}

	out := []core.User{}
	var last core.User
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		// The users table is global: it carries no tenant_id, so the scoped
		// builder cannot constrain it and membership of this tenant is decided
		// by the actor row instead. Paging therefore walks the global ordering
		// and keeps refilling until this tenant's page is full, so that the
		// page the caller sees is a page of its own users rather than a
		// filtered remnant of somebody else's.
		scan := page
		scan.Limit = core.MaxPageLimit
		for {
			found, err := tx.ListUsers(ctx, scan)
			if err != nil {
				return err
			}
			for _, u := range found {
				switch a, err := tx.GetActorByUser(ctx, u.ID); {
				case err == nil:
					enrolled, err := totpEnrolled(ctx, tx, u.ID)
					if err != nil {
						return err
					}
					u.TOTPEnrolled = enrolled
					last = u
					out = append(out, *asMember(&u, a))
					if len(out) == page.Limit {
						return nil
					}
				case !core.IsKind(err, core.KindNotFound):
					return err
				}
			}
			if len(found) < scan.Limit {
				return nil
			}
			scan.Cursor = userCursor(scan, found[len(found)-1]).Encode()
		}
	}); err != nil {
		return nil, "", err
	}
	return out, nextUserCursor(page, len(out), last), nil
}

// UpdateUser changes a user's display name, password, role, disabled state or
// whether their next sign-in must choose a new password.
func (l *Local) UpdateUser(ctx context.Context, id string, in core.UpdateUserInput) (*core.User, error) {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Role != nil && !in.Role.Valid() {
		return nil, core.Invalid("role %q is not recognised", *in.Role)
	}

	hash := ""
	if in.Password != nil {
		if hash, err = l.hasher.Hash(*in.Password); err != nil {
			return nil, err
		}
	}

	var out *core.User
	err = l.write(ctx, actor, func(m *mutation) error {
		u, subject, err := ownUser(ctx, m.tx, id)
		if err != nil {
			return err
		}
		before := *u
		if in.DisplayName != nil {
			u.DisplayName = *in.DisplayName
		}
		if in.RequirePasswordChange != nil {
			u.RequirePasswordChange = *in.RequirePasswordChange
		}
		if in.Disabled != nil {
			switch {
			case *in.Disabled && u.DisabledAt == nil:
				now := m.now
				u.DisabledAt = &now
				// Disabling must take effect now, not when the credential
				// happens to expire.
				if _, err := m.tx.DeleteActorSessions(ctx, subject.ID); err != nil {
					return err
				}
				if _, err := m.tx.RevokeActorTokens(ctx, subject.ID, m.now); err != nil {
					return err
				}
			case !*in.Disabled:
				u.DisabledAt = nil
			}
		}
		if err := m.tx.UpdateUser(ctx, u, hash); err != nil {
			return err
		}
		if in.Role != nil {
			if err := setUserRole(ctx, m.tx, subject.ID, *in.Role); err != nil {
				return err
			}
		}
		out = asMember(u, subject)
		payload := map[string]any{"email": u.Email, "password_changed": hash != ""}
		if in.Role != nil {
			payload["role"] = string(*in.Role)
		}
		if in.RequirePasswordChange != nil {
			payload["require_password_change"] = u.RequirePasswordChange
		}
		return m.Record(auditUserUpdate, eventUserUpdated, "user", subject.ID, "",
			asMember(&before, subject), out, payload)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteUser removes a user and revokes its membership of this tenant, which is
// what stops its actor from acting once the account is gone.
func (l *Local) DeleteUser(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionUserAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		u, a, err := ownUser(ctx, m.tx, id)
		if err != nil {
			return err
		}
		switch _, err := m.tx.GetMember(ctx, a.ID); {
		case err == nil:
			if err := m.tx.RemoveMember(ctx, a.ID); err != nil {
				return err
			}
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
		// Removing the row is not enough: a live session or token would keep
		// working until it expired, so the credentials go in the same
		// transaction as the deletion.
		sessions, err := m.tx.DeleteActorSessions(ctx, a.ID)
		if err != nil {
			return err
		}
		tokens, err := m.tx.RevokeActorTokens(ctx, a.ID, m.now)
		if err != nil {
			return err
		}
		if err := m.tx.DeleteUser(ctx, u.ID); err != nil {
			return err
		}
		return m.Record(auditUserDelete, eventUserDeleted, "user", a.ID, "", asMember(u, a), nil,
			map[string]any{
				"email": u.Email, "handle": a.Handle,
				"sessions_ended": sessions, "tokens_revoked": tokens,
			})
	})
}

// totpEnrolled reports whether a user holds a confirmed second factor. A user
// with no enrolment row is the ordinary case, not an error.
func totpEnrolled(ctx context.Context, tx store.Tx, userID string) (bool, error) {
	enrolment, err := tx.GetUserTOTP(ctx, userID)
	switch {
	case core.IsKind(err, core.KindNotFound):
		return false, nil
	case err != nil:
		return false, err
	default:
		return enrolment.Confirmed(), nil
	}
}
