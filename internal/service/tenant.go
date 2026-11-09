// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Event types for records the core vocabulary does not name, because that
// vocabulary covers tasks, projects, workflows and fields only.
const (
	eventTenantCreated   core.EventType = "tenant.created"
	eventTenantUpdated   core.EventType = "tenant.updated"
	eventTenantDeleted   core.EventType = "tenant.deleted"
	eventDomainAdded     core.EventType = "domain.added"
	eventDomainRemoved   core.EventType = "domain.removed"
	eventDomainUpdated   core.EventType = "domain.updated"
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
	auditDomainUpdate  = "domain.update"
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

// reachTenant resolves a reference to a live tenant: the actor's own, or, for
// a global admin, any. Every other tenant reads as missing, so no caller can
// confirm another tenant exists.
func reachTenant(ctx context.Context, tx store.Tx, actor *core.Actor, ref string, global bool) (*core.Tenant, error) {
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
	if (t.ID != actor.TenantID && !global) || t.DeletedAt != nil {
		return nil, core.NotFound("tenant %q", ref)
	}
	return t, nil
}

// CreateTenant registers a new tenant together with its first admins.
//
// The tenant row and its audit entry are written in the caller's tenant, as
// before; the builtin workflow and the admins go in a second transaction
// scoped to the new tenant, because a transaction is scoped to one tenant and
// on PostgreSQL row-level security holds it there. The second transaction is
// all or nothing, and when it fails the tenant row is removed again, so a
// failure never leaves a tenant nobody can sign in to.
func (l *Local) CreateTenant(ctx context.Context, in core.CreateTenantInput) (*core.CreatedTenant, error) {
	actor, err := l.authorizeGlobal(ctx)
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	key := strings.ToLower(strings.TrimSpace(in.Key))
	accounts, err := l.existingAccounts(ctx, in.Admins)
	if err != nil {
		return nil, err
	}
	admins, err := l.prepareAdmins(in.Admins, accounts)
	if err != nil {
		return nil, err
	}

	var tenant *core.Tenant
	err = l.write(ctx, actor, func(m *mutation) error {
		created, err := createTenantRow(ctx, m, l.ids.New(), key, in.Name, adminEmails(admins))
		tenant = created
		return err
	})
	if err != nil {
		return nil, err
	}

	by, err := l.actingAs(ctx, actor)
	if err != nil {
		return nil, err
	}
	out, err := l.seedTenant(withGlobalStanding(ctx, by), tenant, admins)
	if err != nil {
		if undoErr := l.unwindTenant(ctx, actor, tenant); undoErr != nil {
			return nil, core.Internal("setting up tenant %q failed and removing it failed too; "+
				"remove it from a shell holding the database: %v; %v", key, err, undoErr)
		}
		return nil, err
	}
	return out, nil
}

// pendingAdmin is one admin of a tenant being created, resolved and hashed
// before any transaction opens. Password is set only when it was generated.
// Account is set when the address already has one, which is then made an
// admin of the new tenant with its password and second factor untouched.
type pendingAdmin struct {
	email, handle, displayName string
	hash, generated            string
	account                    *core.User
}

// existingAccounts finds the admins whose addresses already have an account.
// Only a global admin creates a tenant, and a global admin can already set any
// account's password, so making an existing account an admin of the new
// tenant asks nothing of its owner.
func (l *Local) existingAccounts(ctx context.Context, in []core.TenantAdminInput) (map[string]*core.User, error) {
	out := map[string]*core.User{}
	if len(in) == 0 {
		return out, nil
	}
	err := l.store.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		for _, a := range in {
			email := core.NormalEmail(a.Email)
			switch found, _, err := u.GetUserByEmail(ctx, email); {
			case err == nil:
				out[email] = found
			case !core.IsKind(err, core.KindNotFound):
				return err
			}
		}
		return nil
	})
	return out, err
}

// prepareAdmins normalises the admins and hashes the passwords of new ones,
// generating one where none was given. Hashing happens here, outside any
// transaction, because it is deliberately slow. An existing account keeps its
// password, so supplying one for it is refused rather than ignored.
func (l *Local) prepareAdmins(in []core.TenantAdminInput, accounts map[string]*core.User) ([]pendingAdmin, error) {
	out := make([]pendingAdmin, 0, len(in))
	for _, a := range in {
		p := pendingAdmin{
			email:       core.NormalEmail(a.Email),
			handle:      a.HandleOrDefault(),
			displayName: strings.TrimSpace(a.DisplayName),
		}
		if acct := accounts[p.email]; acct != nil {
			if a.Password != "" {
				return nil, core.Invalid("%s already has an account, which keeps its own password; leave the "+
					"password out", p.email).WithDetail(core.DetailField, "admins")
			}
			p.account = acct
			out = append(out, p)
			continue
		}
		password := a.Password
		if password == "" {
			generated, err := auth.NewInitialPassword()
			if err != nil {
				return nil, err
			}
			password, p.generated = generated, generated
		}
		hash, err := l.hasher.Hash(password)
		if err != nil {
			return nil, err
		}
		p.hash = hash
		out = append(out, p)
	}
	return out, nil
}

// adminEmails lists the admins' addresses, for the record of the creation.
func adminEmails(admins []pendingAdmin) []string {
	out := make([]string, 0, len(admins))
	for _, a := range admins {
		out = append(out, a.email)
	}
	return out
}

// emailFree reports a conflict when an account was created for the address
// after the admins were resolved, which only a concurrent request can do.
func emailFree(ctx context.Context, u store.UnscopedTx, email string) error {
	switch _, _, err := u.GetUserByEmail(ctx, email); {
	case err == nil:
		return core.Conflict("an account for %s was created while this tenant was being set up; "+
			"try again", email).WithDetail(core.DetailField, "admins")
	case !core.IsKind(err, core.KindNotFound):
		return err
	}
	return nil
}

// seedTenant gives a new tenant the builtin workflow, without which it could
// hold no projects, and its admins, in one transaction scoped to it.
func (l *Local) seedTenant(ctx context.Context, tenant *core.Tenant, admins []pendingAdmin) (*core.CreatedTenant, error) {
	out := &core.CreatedTenant{Tenant: *tenant}
	err := l.write(ctx, core.SystemActor(tenant.ID), func(m *mutation) error {
		if _, err := ensureBuiltinWorkflow(ctx, m); err != nil {
			return err
		}
		u, err := asUnscoped(m.tx)
		if err != nil {
			return err
		}
		for _, a := range admins {
			if a.account != nil {
				created, err := addAccountAdmin(ctx, m, a)
				if err != nil {
					return err
				}
				out.Admins = append(out.Admins, created)
				continue
			}
			if err := emailFree(ctx, u, a.email); err != nil {
				return err
			}
			created, err := createAdmin(ctx, m, l.ids.New(), a)
			if err != nil {
				return err
			}
			out.Admins = append(out.Admins, created)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, a := range admins {
		if a.generated != "" {
			out.InitialCredentials = append(out.InitialCredentials,
				core.InitialCredential{Email: a.email, Handle: a.handle, Password: a.generated})
		}
	}
	return out, nil
}

// createAdmin writes one admin's user, actor and membership, and records the
// user without any credential: the audit entry and the event name the account
// and say whether its password was generated, never the password.
func createAdmin(ctx context.Context, m *mutation, id string, a pendingAdmin) (core.CreatedAdmin, error) {
	// A generated password has been read by whoever created the tenant, so
	// its owner is asked to replace it at the first sign-in.
	u := &core.User{ID: id, Email: a.email, DisplayName: a.displayName, RequirePasswordChange: a.generated != ""}
	if err := m.tx.CreateUser(ctx, u, a.hash); err != nil {
		return core.CreatedAdmin{}, err
	}
	actor := &core.Actor{ID: id, UserID: id, Kind: core.ActorUser, Handle: a.handle, DisplayName: a.displayName}
	if err := m.tx.CreateActor(ctx, actor); err != nil {
		return core.CreatedAdmin{}, err
	}
	if err := m.tx.AddMember(ctx, &core.Membership{ActorID: id, Role: core.RoleAdmin}); err != nil {
		return core.CreatedAdmin{}, err
	}
	if err := m.logAccount(ctx, id, core.AccountCreated, map[string]any{
		"role": string(core.RoleAdmin), "password_generated": a.generated != "",
	}); err != nil {
		return core.CreatedAdmin{}, err
	}
	credential := "supplied"
	if a.generated != "" {
		credential = "generated"
	}
	if err := m.Record(auditUserCreate, eventUserCreated, "user", id, "", nil, u, map[string]any{
		"email": a.email, "handle": a.handle, "role": string(core.RoleAdmin),
		"credential": credential, "with_tenant": true,
	}); err != nil {
		return core.CreatedAdmin{}, err
	}
	return core.CreatedAdmin{UserID: id, Email: a.email, Handle: a.handle, PasswordGenerated: a.generated != ""}, nil
}

// addAccountAdmin makes an existing account an admin of the new tenant: an
// actor linked to it and an admin membership, recorded as a membership added
// by the global admin who created the tenant.
func addAccountAdmin(ctx context.Context, m *mutation, a pendingAdmin) (core.CreatedAdmin, error) {
	actor, err := createMemberActor(ctx, m, a.account, a.email, a.handle)
	if err != nil {
		return core.CreatedAdmin{}, err
	}
	if err := m.tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleAdmin}); err != nil {
		return core.CreatedAdmin{}, err
	}
	if err := m.logAccount(ctx, a.account.ID, core.AccountMemberAdded,
		map[string]any{"role": string(core.RoleAdmin)}); err != nil {
		return core.CreatedAdmin{}, err
	}
	if err := m.Record(auditMemberAdd, eventMemberAdded, "user", actor.ID, "", nil, asMember(a.account, actor),
		map[string]any{"email": a.email, "handle": actor.Handle, "role": string(core.RoleAdmin),
			"with_tenant": true, "existing_account": true}); err != nil {
		return core.CreatedAdmin{}, err
	}
	return core.CreatedAdmin{UserID: actor.ID, Email: a.email, Handle: actor.Handle}, nil
}

// unwindTenant removes a tenant whose setup failed, and records the removal
// in the caller's tenant beside the creation it undoes.
func (l *Local) unwindTenant(ctx context.Context, actor *core.Actor, tenant *core.Tenant) error {
	return l.write(ctx, actor, func(m *mutation) error {
		u, err := asUnscoped(m.tx)
		if err != nil {
			return err
		}
		if err := u.DeleteTenant(ctx, tenant.ID); err != nil {
			return err
		}
		return m.Record(auditTenantDelete, eventTenantDeleted, "tenant", tenant.ID, "", tenant, nil,
			map[string]any{"key": tenant.Key, "reason": "setup_failed"})
	})
}

// createTenantRow inserts a tenant and records it, rejecting a taken key.
func createTenantRow(ctx context.Context, m *mutation, tenantID, key, name string, admins []string) (*core.Tenant, error) {
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
	payload := map[string]any{"key": t.Key}
	if len(admins) > 0 {
		payload["admins"] = admins
	}
	if err := m.Record(auditTenantCreate, eventTenantCreated, "tenant", t.ID, "", nil, t, payload); err != nil {
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
	global, err := l.globalAllowed(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.Tenant
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		t, err := reachTenant(ctx, tx, actor, ref, global)
		out = t
		return err
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListTenants returns the tenants the actor may see: every live tenant for a
// global admin, and the actor's own for anybody else, since an actor is bound
// to one tenant and a wider listing would disclose another's data.
func (l *Local) ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, string, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	if page, err = page.Normalize(); err != nil {
		return nil, "", err
	}
	global, err := l.globalAllowed(ctx)
	if err != nil {
		return nil, "", err
	}
	if global {
		return l.listEveryTenant(ctx, actor, page)
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

// listEveryTenant pages through every live tenant. Deleted tenants are
// skipped, and a page is refilled until it is full, so a short page means the
// listing has ended.
func (l *Local) listEveryTenant(ctx context.Context, actor *core.Actor, page core.Page) ([]core.Tenant, string, error) {
	if page.Sort == "" {
		page.Sort = core.SortCreatedAt
	}
	out := []core.Tenant{}
	err := l.read(ctx, actor, func(tx store.Tx) error {
		u, err := asUnscoped(tx)
		if err != nil {
			return err
		}
		scan := page
		scan.Limit = core.MaxPageLimit
		for {
			found, err := u.ListTenants(ctx, scan)
			if err != nil {
				return err
			}
			for _, t := range found {
				if t.DeletedAt != nil {
					continue
				}
				out = append(out, t)
				if len(out) == page.Limit {
					return nil
				}
			}
			if len(found) < scan.Limit {
				return nil
			}
			scan.Cursor = tenantCursor(scan, found[len(found)-1]).Encode()
		}
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) == page.Limit {
		next = tenantCursor(page, out[len(out)-1]).Encode()
	}
	return out, next, nil
}

// tenantCursor addresses the position just after this tenant in the ordering.
func tenantCursor(page core.Page, t core.Tenant) core.Cursor {
	value := sqlb.TimeText(t.CreatedAt)
	switch page.Sort {
	case "key":
		value = t.Key
	case "name":
		value = t.Name
	}
	return core.Cursor{SortValue: value, ID: t.ID, Sort: page.Sort, Direction: page.Direction}
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
	// Refused here rather than at render time. A name that does not resolve is
	// almost always a typo, and this is the moment the person who made it is
	// still looking; a page that quietly fell back to the default would teach
	// them nothing. The empty string clears the theme and is always allowed.
	if in.Theme != nil {
		if name := strings.TrimSpace(*in.Theme); name != "" {
			_, custom := core.CustomTheme(name)
			if _, ok := l.themeRegistry().Lookup(name); !ok && !custom {
				return nil, core.NotFound("theme %q", name)
			}
		}
	}

	// Requiring a factor is safe only because an unenrolled user enters
	// enrolment instead of being refused. With no sealing key enrolment is
	// itself refused, so that escape hatch is shut and the requirement locks
	// out everybody who has not already enrolled, the administrator setting it
	// among them. Refused here, where the person who asked is still present,
	// rather than discovered at the next sign-in.
	if err := l.totpRequirementIsReachable(in); err != nil {
		return nil, err
	}
	core.NormalizeTimePrefs(in.TimeFormat, in.Timezone)
	if err := in.Validate(); err != nil {
		return nil, err
	}

	global, err := l.globalAllowed(ctx)
	if err != nil {
		return nil, err
	}
	var out *core.Tenant
	err = l.write(ctx, actor, func(m *mutation) error {
		t, err := reachTenant(ctx, m.tx, actor, ref, global)
		if err != nil {
			return err
		}
		before := *t
		if in.Name != nil {
			t.Name = *in.Name
		}
		if in.Theme != nil {
			t.Theme = strings.ToLower(strings.TrimSpace(*in.Theme))
		}
		if in.LeaseDefault != nil {
			t.LeaseDefault = *in.LeaseDefault
		}
		if in.LeaseMax != nil {
			t.LeaseMax = *in.LeaseMax
		}
		// Checked against what the tenant will hold, not against what this
		// call carried. Setting only --lease-default has to be judged by the
		// maximum already stored, or the two could be walked into
		// contradiction one flag at a time, each call looking fine by itself.
		if t.LeaseDefault > 0 && t.LeaseMax > 0 && t.LeaseDefault > t.LeaseMax {
			return core.Invalid(
				"lease default %s is longer than the lease maximum %s",
				t.LeaseDefault, t.LeaseMax)
		}
		if in.RequireTOTPWeb != nil {
			t.RequireTOTPWeb = *in.RequireTOTPWeb
		}
		if in.RequireTOTPCLI != nil {
			t.RequireTOTPCLI = *in.RequireTOTPCLI
		}
		if in.RequireTOTPSSH != nil {
			t.RequireTOTPSSH = *in.RequireTOTPSSH
		}
		if !before.MFARequired() {
			if err := policyLeavesCallerIn(ctx, m.tx, actor, in); err != nil {
				return err
			}
		}
		applyMFAPolicy(t, in, m.now)
		if in.TrashOff != nil {
			t.TrashOff = *in.TrashOff
		}
		if in.TimeFormat != nil {
			t.TimeFormat = *in.TimeFormat
		}
		if in.Timezone != nil {
			t.Timezone = *in.Timezone
		}
		u, err := asUnscoped(m.tx)
		if err != nil {
			return err
		}
		if err := u.UpdateTenant(ctx, t); err != nil {
			return err
		}
		out = t
		meta := map[string]any{"key": t.Key}
		if before.TrashOff != t.TrashOff {
			meta["trash"] = trashWord(!t.TrashOff)
		}
		if before.MFARequired() != t.MFARequired() {
			meta["mfa_policy"] = string(t.MFAPolicy)
		}
		if before.MFAGrace != t.MFAGrace {
			meta["mfa_grace"] = t.MFAGrace.String()
		}
		return m.Record(auditTenantUpdate, eventTenantUpdated, "tenant", t.ID, "", before, t, meta)
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
	global, err := l.globalAllowed(ctx)
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		t, err := reachTenant(ctx, m.tx, actor, ref, global)
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
		target, err := m.tx.GetActor(ctx, actorID)
		if err != nil {
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
		if err := m.logAccount(ctx, target.UserID, core.AccountMemberAdded,
			map[string]any{"role": string(role)}); err != nil {
			return err
		}
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
		if target, err := m.tx.GetActor(ctx, actorID); err == nil {
			if err := m.logAccount(ctx, target.UserID, core.AccountMemberRemoved, nil); err != nil {
				return err
			}
		} else if !core.IsKind(err, core.KindNotFound) {
			return err
		}
		return m.Record(auditMemberRemove, eventMemberRemoved, "membership", actorID, "", before, nil,
			map[string]any{"role": string(before.Role)})
	})
}

// themeRegistry is the configured registry, or the built-ins when nothing
// supplied one. A Local built without WithThemes still has to resolve names,
// and every test that constructs one is such a caller.
func (l *Local) themeRegistry() *core.ThemeRegistry {
	if l.themes != nil {
		return l.themes
	}
	// Cannot fail: the built-ins are constants core's own tests validate.
	r, _ := core.NewThemeRegistry(nil)
	return r
}

// ResolveTheme returns the palette a tenant presents itself with, for the
// surfaces that render it.
func (l *Local) ResolveTheme(t *core.Tenant) core.Theme {
	return l.themeRegistry().Resolve(t)
}

// totpRequirementIsReachable refuses a requirement nobody could satisfy. Only
// turning one on is checked: clearing one, or leaving it alone, needs no key.
func (l *Local) totpRequirementIsReachable(in core.UpdateTenantInput) error {
	if in.MFAPolicy != nil && *in.MFAPolicy == core.MFAPolicyRequired {
		if _, err := l.sealer(); err != nil {
			return err
		}
	}
	for _, on := range []*bool{in.RequireTOTPWeb, in.RequireTOTPCLI, in.RequireTOTPSSH} {
		if on != nil && *on {
			if _, err := l.sealer(); err != nil {
				return err
			}
			return nil
		}
	}
	return nil
}

// applyMFAPolicy writes the policy and grace an update carries. The time the
// policy turned required is stamped only on the change, so re-sending
// required, or changing only the grace, moves the deadline without restarting
// it; turning the policy off forgets it.
func applyMFAPolicy(t *core.Tenant, in core.UpdateTenantInput, now time.Time) {
	if in.MFAGrace != nil {
		t.MFAGrace = *in.MFAGrace
	}
	if in.MFAPolicy == nil {
		return
	}
	was := t.MFARequired()
	t.MFAPolicy = *in.MFAPolicy
	if t.MFAPolicy == "" {
		t.MFAPolicy = core.MFAPolicyOff
	}
	switch {
	case !t.MFARequired():
		t.MFARequiredAt = nil
	case !was || t.MFARequiredAt == nil:
		at := now
		t.MFARequiredAt = &at
	}
}

// policyLeavesCallerIn refuses turning the second factor policy on for a
// caller who would be the first person it shut out. A caller with no account
// behind it, the machine-local actor, is not refused: the gate never applies
// to it either. The account is global, so it is read in the caller's own
// tenant even when a global admin edits another.
func policyLeavesCallerIn(ctx context.Context, tx store.Tx, actor *core.Actor, in core.UpdateTenantInput) error {
	if in.MFAPolicy == nil || *in.MFAPolicy != core.MFAPolicyRequired {
		return nil
	}
	enrolled, person, err := callerFactor(ctx, tx, actor)
	if err != nil {
		return err
	}
	if person && !enrolled {
		return core.Precondition("you hold no second factor, so requiring one would lock you out; "+
			"enrol first with tix user totp enrol or under Settings in the browser, then turn the policy on").
			WithDetail(core.DetailField, "mfa_policy")
	}
	return nil
}

// callerFactor reads whether the caller is a person and holds a factor.
func callerFactor(ctx context.Context, tx store.Tx, actor *core.Actor) (enrolled, person bool, err error) {
	user, err := accountOfActor(ctx, tx, actor.ID)
	if core.IsKind(err, core.KindNotFound) {
		return false, false, nil
	}
	if err != nil {
		return false, false, err
	}
	enrolled, err = totpEnrolled(ctx, tx, user.ID)
	return enrolled, true, err
}
