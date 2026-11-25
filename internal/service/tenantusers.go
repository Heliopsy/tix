// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// inTenant is how a global admin reaches into another tenant: it resolves the
// tenant and returns a context whose actor is the caller standing in that
// tenant as its admin. The ordinary tenant-confined methods then run there
// unchanged, every query scoped to that tenant, and their audit entries name
// the global admin who acted. Anybody else is refused before the reference is
// looked at, so the answer says nothing about which tenants exist.
func (l *Local) inTenant(ctx context.Context, ref string) (context.Context, error) {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return nil, err
	}
	var target *core.Tenant
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		t, err := reachTenant(ctx, tx, actor, ref, true)
		target = t
		return err
	}); err != nil {
		return nil, err
	}
	by, err := l.actingAs(ctx, actor)
	if err != nil {
		return nil, err
	}
	stand := *actor
	stand.TenantID = target.ID
	stand.Role = core.RoleAdmin
	stand.Scopes = []core.Scope{core.ScopeAll}
	stand.ProjectID = ""
	return withGlobalStanding(core.WithActor(ctx, &stand), by), nil
}

// ListTenantUsers lists the users of any tenant with their role there,
// leaving out people removed from it unless the filter asks for them.
func (l *Local) ListTenantUsers(ctx context.Context, tenant string, f core.UserFilter) ([]core.TenantUser, string, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, "", err
	}
	users, next, err := l.ListUsers(inner, f)
	if err != nil {
		return nil, "", err
	}
	actor, _ := core.ActorFrom(inner)
	out := make([]core.TenantUser, 0, len(users))
	err = l.read(inner, actor, func(tx store.Tx) error {
		for _, u := range users {
			role, err := userRole(inner, tx, u.ID)
			if err != nil {
				return err
			}
			out = append(out, core.TenantUser{User: u, Role: role})
		}
		return nil
	})
	if err != nil {
		return nil, "", err
	}
	return out, next, nil
}

// CreateTenantUser creates a user in any tenant.
func (l *Local) CreateTenantUser(ctx context.Context, tenant string, in core.CreateUserInput) (*core.User, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return l.CreateUser(inner, in)
}

// UpdateTenantUser changes a user of any tenant.
func (l *Local) UpdateTenantUser(ctx context.Context, tenant, id string, in core.UpdateUserInput) (*core.User, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	return l.UpdateUser(inner, id, in)
}

// DeleteTenantUser deletes a user of any tenant.
func (l *Local) DeleteTenantUser(ctx context.Context, tenant, id string) error {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return err
	}
	return l.DeleteUser(inner, id)
}

// AddTenantMember adds an existing account to any tenant. A global admin can
// already set the account's password, so asking the person to accept adds no
// protection; the addition is recorded in the tenant, naming the global admin.
func (l *Local) AddTenantMember(ctx context.Context, tenant string, in core.AddTenantMemberInput) (*core.User, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	email := core.NormalEmail(in.Email)
	found, err := l.inviteeByEmail(ctx, email)
	if err != nil {
		return nil, err
	}
	if found.user == nil {
		return nil, core.NotFound("no account has the address %s; create the user instead", email).
			WithDetail(core.DetailField, "email")
	}
	role := in.Role
	if role == "" {
		role = defaultUserRole
	}
	actor, _ := core.ActorFrom(inner)
	var out *core.User
	err = l.write(inner, actor, func(m *mutation) error {
		member, err := m.tx.GetActorByUser(inner, found.user.ID)
		switch {
		case core.IsKind(err, core.KindNotFound):
			if member, err = createMemberActor(inner, m, found.user, email, in.Handle); err != nil {
				return err
			}
		case err != nil:
			return err
		}
		current, err := userRole(inner, m.tx, member.ID)
		if err != nil {
			return err
		}
		if current != "" {
			return core.Conflict("%s is already a member of this tenant", email).WithDetail(core.DetailField, "email")
		}
		if member.DisabledAt != nil {
			member.DisabledAt = nil
			if err := m.tx.UpdateActor(inner, member); err != nil {
				return err
			}
		}
		if err := setUserRole(inner, m.tx, member.ID, role); err != nil {
			return err
		}
		out = asMember(found.user, member)
		if err := m.logAccount(inner, found.user.ID, core.AccountMemberAdded,
			map[string]any{"role": string(role)}); err != nil {
			return err
		}
		return m.Record(auditMemberAdd, eventMemberAdded, "user", member.ID, "", nil, out,
			map[string]any{"email": email, "handle": member.Handle, "role": string(role), "global_admin": true})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// createMemberActor writes the actor an existing account acts as in this
// tenant, refusing a handle already in use here.
func createMemberActor(ctx context.Context, m *mutation, account *core.User, email, handle string,
) (*core.Actor, error) {
	handle = userHandle(handle, email)
	if handle == "" {
		return nil, core.Invalid("a user needs a handle").WithDetail(core.DetailField, "handle")
	}
	switch _, err := m.tx.GetActorByHandle(ctx, handle); {
	case err == nil:
		return nil, core.Conflict("handle %q is already in use in this tenant; pass another handle", handle).
			WithDetail(core.DetailField, "handle")
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}
	actor := &core.Actor{UserID: account.ID, Kind: core.ActorUser, Handle: handle, DisplayName: account.DisplayName}
	return actor, m.tx.CreateActor(ctx, actor)
}
