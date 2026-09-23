// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Keys of the records a fresh installation is given, so that single-user local
// use never has to name a tenant, a workflow or a project.
const (
	DefaultTenantKey   = "default"
	DefaultProjectKey  = "default"
	BuiltinWorkflowKey = "default"
)

// DefaultLease is how long a claim on a task holds by default.
const DefaultLease = core.Duration(30 * time.Minute)

// BuiltinWorkflow returns the shipped state machine.
func BuiltinWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "To do", Category: core.CategoryTodo},
			{Key: "doing", Label: "Doing", Category: core.CategoryInProgress,
				RevertOnLeaseExpiry: true, RevertTo: "todo"},
			{Key: "blocked", Label: "Blocked", Category: core.CategoryTodo},
			{Key: "done", Label: "Done", Category: core.CategoryDone, Terminal: true},
			{Key: "cancelled", Label: "Cancelled", Category: core.CategoryDone, Terminal: true},
		},
		Transitions: []core.Transition{
			{From: "todo", To: "doing"},
			{From: "todo", To: "blocked"},
			{From: "todo", To: "cancelled"},
			{From: "doing", To: "todo"},
			{From: "doing", To: "blocked"},
			{From: "doing", To: "done"},
			{From: "doing", To: "cancelled"},
			{From: "blocked", To: "todo"},
			{From: "blocked", To: "doing"},
			{From: "blocked", To: "cancelled"},
			{From: "done", To: "todo"},
			{From: "cancelled", To: "todo"},
		},
		DefaultLease: DefaultLease,
	}
}

// starterProjects are the lists a brand new installation opens on, so the
// first screen shows a workspace rather than an empty table. They are seeded
// once, when the tenant is first created, and never afterwards: see
// EnsureDefaults.
var starterProjects = []core.Project{
	{Key: DefaultProjectKey, Name: "Default", Color: core.ColorSlate, Icon: "\U0001F4CB"},
	{Key: "work", Name: "Work", Color: core.ColorBlue, Icon: "\U0001F4BC"},
	{Key: "homelab", Name: "Homelab", Color: core.ColorViolet, Icon: "\U0001F5A5"},
	{Key: "house", Name: "House", Color: core.ColorGreen, Icon: "\U0001F3E1"},
}

// EnsureDefaults creates the default tenant and the builtin workflow if they
// are missing, and on a tenant it has just created seeds the starter lists. It
// is idempotent, so it can run on every start.
//
// Seeding is deliberately tied to creating the tenant rather than to a project
// being absent. "Create if missing" would resurrect a list someone deleted on
// purpose every time the process dialled, and would drop new lists into an
// installation that has been running for a year.
func (l *Local) EnsureDefaults(ctx context.Context) (*core.Tenant, error) {
	tenant, fresh, err := l.ensureDefaultTenant(ctx)
	if err != nil {
		return nil, err
	}
	actor := core.SystemActor(tenant.ID)
	if err := l.write(ctx, actor, func(m *mutation) error {
		wf, err := ensureBuiltinWorkflow(ctx, m)
		if err != nil {
			return err
		}
		if !fresh {
			return nil
		}
		return l.seedProjects(ctx, m, wf)
	}); err != nil {
		return nil, err
	}
	return tenant, nil
}

// seedProjects writes the starter lists into a tenant that has just been
// created.
func (l *Local) seedProjects(ctx context.Context, m *mutation, wf *core.Workflow) error {
	wanted := starterProjects
	if l.noStarterProjects {
		wanted = starterProjects[:1]
	}
	for _, seed := range wanted {
		p := seed
		p.WorkflowID = wf.ID
		if err := m.tx.CreateProject(ctx, &p); err != nil {
			return err
		}
		if err := m.Record(auditProjectCreate, core.EventProjectCreated, "project", p.ID, p.ID, nil, &p,
			map[string]any{"key": p.Key, "seeded": true}); err != nil {
			return err
		}
	}
	return nil
}

// ensureDefaultTenant returns the default tenant, reporting whether this call
// is what created it.
func (l *Local) ensureDefaultTenant(ctx context.Context) (*core.Tenant, bool, error) {
	existing, err := l.findDefaultTenant(ctx)
	if err != nil {
		return nil, false, err
	}
	if existing != nil {
		return existing, false, nil
	}

	tenantID := l.ids.New()
	var created *core.Tenant
	err = l.write(ctx, core.SystemActor(tenantID), func(m *mutation) error {
		t, err := createTenantRow(ctx, m, tenantID, DefaultTenantKey, "Default")
		created = t
		return err
	})
	if core.IsKind(err, core.KindConflict) {
		t, err := l.requireDefaultTenant(ctx)
		return t, false, err
	}
	if err != nil {
		return nil, false, err
	}
	return created, true, nil
}

// findDefaultTenant returns the default tenant, or nil when it does not exist.
func (l *Local) findDefaultTenant(ctx context.Context) (*core.Tenant, error) {
	var out *core.Tenant
	err := l.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		t, err := u.GetTenantByKey(ctx, DefaultTenantKey)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		out = t
		return err
	})
	if err != nil {
		return nil, err
	}
	if out != nil && out.DeletedAt != nil {
		return nil, core.Conflict("the default tenant is deleted")
	}
	return out, nil
}

// requireDefaultTenant returns the default tenant or reports it missing.
func (l *Local) requireDefaultTenant(ctx context.Context) (*core.Tenant, error) {
	t, err := l.findDefaultTenant(ctx)
	if err != nil {
		return nil, err
	}
	if t == nil {
		return nil, core.NotFound("tenant with key %q", DefaultTenantKey)
	}
	return t, nil
}

// ensureBuiltinWorkflow returns the builtin workflow, creating it if missing.
func ensureBuiltinWorkflow(ctx context.Context, m *mutation) (*core.Workflow, error) {
	switch existing, err := m.tx.GetWorkflow(ctx, BuiltinWorkflowKey); {
	case err == nil:
		return existing, nil
	case !core.IsKind(err, core.KindNotFound):
		return nil, err
	}

	wf := &core.Workflow{
		Key:        BuiltinWorkflowKey,
		Name:       "Default",
		Definition: BuiltinWorkflow(),
		Builtin:    true,
	}
	if err := m.tx.PutWorkflow(ctx, wf); err != nil {
		return nil, err
	}
	if err := m.Record(auditWorkflowPut, core.EventWorkflowUpdated, "workflow", wf.ID, "", nil, wf,
		map[string]any{"key": wf.Key, "builtin": true}); err != nil {
		return nil, err
	}
	return wf, nil
}
