package service

import (
	"context"
	"strings"

	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// Event types for records the core vocabulary does not name, because that
// vocabulary covers tasks, projects, workflows and fields only.
const (
	eventTenantCreated   core.EventType = "tenant.created"
	eventTenantUpdated   core.EventType = "tenant.updated"
	eventTenantDeleted   core.EventType = "tenant.deleted"
	eventDomainAdded     core.EventType = "domain.added"
	eventDomainRemoved   core.EventType = "domain.removed"
	eventMemberAdded     core.EventType = "member.added"
	eventMemberRemoved   core.EventType = "member.removed"
	eventProjectDeleted  core.EventType = "project.deleted"
	eventWorkflowDeleted core.EventType = "workflow.deleted"
	eventFieldDeleted    core.EventType = "field.deleted"
)

// Audit actions recorded by this package.
const (
	auditTenantCreate  = "tenant.create"
	auditTenantUpdate  = "tenant.update"
	auditTenantDelete  = "tenant.delete"
	auditDomainAdd     = "domain.add"
	auditDomainRemove  = "domain.remove"
	auditMemberAdd     = "member.add"
	auditMemberRemove  = "member.remove"
	auditProjectCreate = "project.create"
	auditProjectUpdate = "project.update"
	auditProjectDelete = "project.delete"
	auditFieldPut      = "field.put"
	auditFieldDelete   = "field.delete"
	auditWorkflowPut   = "workflow.put"
	auditWorkflowDel   = "workflow.delete"
	auditTaskMigrate   = "task.migrate"
)

// asUnscoped reaches the cross-tenant face of a transaction. Tenant rows carry
// no tenant column, so administering them needs that face of the same
// transaction rather than a second one, which would split the audit entry from
// the row it describes.
func asUnscoped(tx store.Tx) (store.UnscopedTx, error) {
	u, ok := tx.(store.UnscopedTx)
	if !ok {
		return nil, core.Internal("this store cannot administer tenants inside a scoped transaction")
	}
	return u, nil
}

// lookupTenant resolves a tenant by identifier or by key.
func lookupTenant(ctx context.Context, u store.UnscopedTx, ref string) (*core.Tenant, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("tenant reference is required")
	}
	t, err := u.GetTenantByID(ctx, ref)
	if err == nil {
		return t, nil
	}
	if !core.IsKind(err, core.KindNotFound) {
		return nil, err
	}
	return u.GetTenantByKey(ctx, strings.ToLower(ref))
}

// ownTenant resolves a reference and refuses anything outside the actor's own
// tenant as missing, so no caller can confirm another tenant exists.
func ownTenant(ctx context.Context, tx store.Tx, actor *core.Actor, ref string) (*core.Tenant, error) {
	if strings.TrimSpace(ref) == "" {
		ref = actor.TenantID
	}
	u, err := asUnscoped(tx)
	if err != nil {
		return nil, err
	}
	t, err := lookupTenant(ctx, u, ref)
	if err != nil {
		return nil, err
	}
	if t.ID != actor.TenantID || t.DeletedAt != nil {
		return nil, core.NotFound("tenant %q", ref)
	}
	return t, nil
}

// CreateTenant registers a new tenant.
func (l *Local) CreateTenant(ctx context.Context, in core.CreateTenantInput) (*core.Tenant, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	key := strings.ToLower(strings.TrimSpace(in.Key))

	var out *core.Tenant
	err = l.write(ctx, actor, func(m *mutation) error {
		created, err := createTenantRow(ctx, m, "", key, in.Name)
		out = created
		return err
	})
	if err != nil {
		return nil, err
	}

	if err := l.seedTenantWorkflow(ctx, out.ID); err != nil {
		return nil, err
	}
	return out, nil
}

// seedTenantWorkflow gives a new tenant the builtin workflow, without which it
// could hold no projects. It runs in its own transaction because the caller's
// transaction is scoped to the caller's tenant, not the new one. It is
// idempotent, so a retry after a partial failure repairs the tenant.
func (l *Local) seedTenantWorkflow(ctx context.Context, tenantID string) error {
	system := core.SystemActor(tenantID)
	return l.write(ctx, system, func(m *mutation) error {
		_, err := ensureBuiltinWorkflow(ctx, m)
		return err
	})
}

// createTenantRow inserts a tenant and records it, rejecting a taken key.
func createTenantRow(ctx context.Context, m *mutation, tenantID, key, name string) (*core.Tenant, error) {
	u, err := asUnscoped(m.tx)
	if err != nil {
		return nil, err
	}
	switch _, err := u.GetTenantByKey(ctx, key); {
	case err == nil:
		return nil, core.Conflict("tenant key %q is already in use", key)
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}

	t := &core.Tenant{ID: tenantID, Key: key, Name: name}
	if err := u.CreateTenant(ctx, t); err != nil {
		return nil, err
	}
	if err := m.Record(auditTenantCreate, eventTenantCreated, "tenant", t.ID, "", nil, t,
		map[string]any{"key": t.Key}); err != nil {
		return nil, err
	}
	return t, nil
}

// GetTenant returns a tenant by identifier or key. An empty reference means the
// actor's own tenant.
func (l *Local) GetTenant(ctx context.Context, ref string) (*core.Tenant, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Tenant
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		t, err := ownTenant(ctx, tx, actor, ref)
		out = t
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTenants returns the tenants the actor may see, which is its own. An actor
// is bound to one tenant, so a wider listing would disclose another's data.
func (l *Local) ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, string, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	if _, err := page.Normalize(); err != nil {
		return nil, "", err
	}
	out := []core.Tenant{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		t, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if t.DeletedAt == nil {
			out = append(out, *t)
		}
		return nil
	}); err != nil {
		return nil, "", err
	}
	return out, "", nil
}

// UpdateTenant changes a tenant's mutable fields.
func (l *Local) UpdateTenant(ctx context.Context, ref string, in core.UpdateTenantInput) (*core.Tenant, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return nil, core.Invalid("tenant name is required")
	}

	var out *core.Tenant
	err = l.write(ctx, actor, func(m *mutation) error {
		t, err := ownTenant(ctx, m.tx, actor, ref)
		if err != nil {
			return err
		}
		before := *t
		if in.Name != nil {
			t.Name = *in.Name
		}
		u, err := asUnscoped(m.tx)
		if err != nil {
			return err
		}
		if err := u.UpdateTenant(ctx, t); err != nil {
			return err
		}
		out = t
		return m.Record(auditTenantUpdate, eventTenantUpdated, "tenant", t.ID, "", before, t,
			map[string]any{"key": t.Key})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTenant soft deletes a tenant, so its data stops being reachable and its
// domains stop resolving without any row being destroyed.
func (l *Local) DeleteTenant(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		t, err := ownTenant(ctx, m.tx, actor, ref)
		if err != nil {
			return err
		}
		before := *t
		now := m.now
		t.DeletedAt = &now
		u, err := asUnscoped(m.tx)
		if err != nil {
			return err
		}
		if err := u.UpdateTenant(ctx, t); err != nil {
			return err
		}
		return m.Record(auditTenantDelete, eventTenantDeleted, "tenant", t.ID, "", before, t,
			map[string]any{"key": t.Key})
	})
}

// AddMember grants an actor a role in the current tenant.
func (l *Local) AddMember(ctx context.Context, actorID string, role core.Role) (*core.Membership, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(actorID) == "" {
		return nil, core.Invalid("actor identifier is required")
	}
	if !role.Valid() {
		return nil, core.Invalid("role %q must be %q, %q or %q", role, core.RoleViewer, core.RoleMember, core.RoleAdmin)
	}

	var out *core.Membership
	err = l.write(ctx, actor, func(m *mutation) error {
		if _, err := m.tx.GetActor(ctx, actorID); err != nil {
			return err
		}
		switch _, err := m.tx.GetMember(ctx, actorID); {
		case err == nil:
			return core.Conflict("actor %q is already a member of this tenant", actorID)
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
		mem := &core.Membership{ActorID: actorID, Role: role}
		if err := m.tx.AddMember(ctx, mem); err != nil {
			return err
		}
		out = mem
		return m.Record(auditMemberAdd, eventMemberAdded, "membership", actorID, "", nil, mem,
			map[string]any{"role": string(role)})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListMembers returns every membership in the current tenant.
func (l *Local) ListMembers(ctx context.Context) ([]core.Membership, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.Membership{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListMembers(ctx)
		out = found
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// RemoveMember revokes an actor's membership in the current tenant.
func (l *Local) RemoveMember(ctx context.Context, actorID string) error {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		before, err := m.tx.GetMember(ctx, actorID)
		if err != nil {
			return err
		}
		if err := m.tx.RemoveMember(ctx, actorID); err != nil {
			return err
		}
		return m.Record(auditMemberRemove, eventMemberRemoved, "membership", actorID, "", before, nil,
			map[string]any{"role": string(before.Role)})
	})
}
