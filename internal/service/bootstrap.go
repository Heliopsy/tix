package service

import (
	"context"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
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

// EnsureDefaults creates the default tenant, the builtin workflow and the
// default project if they are missing, and returns the default tenant. It is
// idempotent, so it can run on every start.
func (l *Local) EnsureDefaults(ctx context.Context) (*core.Tenant, error) {
	tenant, err := l.ensureDefaultTenant(ctx)
	if err != nil {
		return nil, err
	}
	actor := core.SystemActor(tenant.ID)
	if err := l.write(ctx, actor, func(m *mutation) error {
		wf, err := ensureBuiltinWorkflow(ctx, m)
		if err != nil {
			return err
		}
		return ensureDefaultProject(ctx, m, wf)
	}); err != nil {
		return nil, err
	}
	return tenant, nil
}

// ensureDefaultTenant returns the default tenant, creating it on first use.
func (l *Local) ensureDefaultTenant(ctx context.Context) (*core.Tenant, error) {
	existing, err := l.findDefaultTenant(ctx)
	if err != nil {
		return nil, err
	}
	if existing != nil {
		return existing, nil
	}

	tenantID := l.ids.New()
	var created *core.Tenant
	err = l.write(ctx, core.SystemActor(tenantID), func(m *mutation) error {
		t, err := createTenantRow(ctx, m, tenantID, DefaultTenantKey, "Default")
		created = t
		return err
	})
	if core.IsKind(err, core.KindConflict) {
		return l.requireDefaultTenant(ctx)
	}
	if err != nil {
		return nil, err
	}
	return created, nil
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

// ensureDefaultProject creates the default project if it is missing.
func ensureDefaultProject(ctx context.Context, m *mutation, wf *core.Workflow) error {
	switch _, err := m.tx.GetProject(ctx, DefaultProjectKey); {
	case err == nil:
		return nil
	case !core.IsKind(err, core.KindNotFound):
		return err
	}

	p := &core.Project{Key: DefaultProjectKey, Name: "Default", WorkflowID: wf.ID}
	if err := m.tx.CreateProject(ctx, p); err != nil {
		return err
	}
	return m.Record(auditProjectCreate, core.EventProjectCreated, "project", p.ID, p.ID, nil, p,
		map[string]any{"key": p.Key})
}
