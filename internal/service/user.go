package service

import (
	"context"
	"strings"

	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
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
func userHandle(handle, email string) string {
	handle = strings.ToLower(strings.TrimSpace(handle))
	if handle != "" {
		return handle
	}
	local, _, _ := strings.Cut(email, "@")
	return local
}

// ownUser resolves a user that has an actor in the caller's tenant. A user of
// another tenant is reported missing, so no caller can confirm it exists.
//
// A user and its actor share one identifier; that is what binds a global user
// row to exactly one tenant.
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
	u, err := tx.GetUser(ctx, id)
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, nil, core.NotFound("user %q", id)
		}
		return nil, nil, err
	}
	return u, a, nil
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

// nextUserCursor returns the cursor for the page after these users.
func nextUserCursor(page core.Page, items []core.User) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	last := items[len(items)-1]
	value := sqlb.TimeText(last.CreatedAt)
	if page.Sort == "email" {
		value = last.Email
	}
	return core.Cursor{
		SortValue: value,
		ID:        last.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}.Encode()
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
		u := &core.User{ID: id, Email: email, DisplayName: in.DisplayName}
		if err := m.tx.CreateUser(ctx, u, hash); err != nil {
			return err
		}
		a := &core.Actor{ID: id, Kind: core.ActorUser, Handle: handle, DisplayName: in.DisplayName}
		if err := m.tx.CreateActor(ctx, a); err != nil {
			return err
		}
		if err := setUserRole(ctx, m.tx, id, role); err != nil {
			return err
		}
		out = u
		return m.Record(auditUserCreate, eventUserCreated, "user", id, "", nil, u,
			map[string]any{"email": email, "handle": handle, "role": string(role)})
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
		u, _, err := ownUser(ctx, tx, id)
		out = u
		return err
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

	var fetched []core.User
	out := []core.User{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListUsers(ctx, page)
		if err != nil {
			return err
		}
		fetched = found
		for _, u := range found {
			switch _, err := tx.GetActor(ctx, u.ID); {
			case err == nil:
				out = append(out, u)
			case !core.IsKind(err, core.KindNotFound):
				return err
			}
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	return out, nextUserCursor(page, fetched), nil
}

// UpdateUser changes a user's display name, password, role or disabled state.
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
			if err := setUserRole(ctx, m.tx, u.ID, *in.Role); err != nil {
				return err
			}
		}
		out = u
		payload := map[string]any{"email": u.Email, "password_changed": hash != ""}
		if in.Role != nil {
			payload["role"] = string(*in.Role)
		}
		return m.Record(auditUserUpdate, eventUserUpdated, "user", u.ID, "", before, u, payload)
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
		return m.Record(auditUserDelete, eventUserDeleted, "user", u.ID, "", u, nil,
			map[string]any{
				"email": u.Email, "handle": a.Handle,
				"sessions_ended": sessions, "tokens_revoked": tokens,
			})
	})
}
