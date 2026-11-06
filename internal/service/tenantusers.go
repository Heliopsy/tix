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
	stand := *actor
	stand.TenantID = target.ID
	stand.Role = core.RoleAdmin
	stand.Scopes = []core.Scope{core.ScopeAll}
	stand.ProjectID = ""
	return core.WithActor(ctx, &stand), nil
}

// ListTenantUsers lists the users of any tenant with their role there.
func (l *Local) ListTenantUsers(ctx context.Context, tenant string, page core.Page) ([]core.TenantUser, string, error) {
	inner, err := l.inTenant(ctx, tenant)
	if err != nil {
		return nil, "", err
	}
	users, next, err := l.ListUsers(inner, page)
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
