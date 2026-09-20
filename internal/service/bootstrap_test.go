package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store/sqlite"
)

// newEmpty returns a service over a migrated but otherwise empty database, so
// bootstrap can be observed from the state a fresh installation starts in.
func newEmpty(t *testing.T) *Local {
	t.Helper()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return New(st, WithClock(clk))
}

func TestEnsureDefaultsCreatesTenantWorkflowAndProject(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	if tenant.Key != DefaultTenantKey {
		t.Errorf("tenant key = %q, want %q", tenant.Key, DefaultTenantKey)
	}

	admin := core.SystemActor(tenant.ID)
	actorCtx := core.WithActor(ctx, admin)

	wf, err := l.GetWorkflow(actorCtx, BuiltinWorkflowKey)
	if err != nil {
		t.Fatalf("GetWorkflow: %v", err)
	}
	if !wf.Builtin {
		t.Error("the shipped workflow is not marked builtin")
	}
	if wf.Definition.Initial != "todo" {
		t.Errorf("initial state = %q, want todo", wf.Definition.Initial)
	}
	for _, key := range []string{"todo", "doing", "blocked", "done", "cancelled"} {
		if !wf.Definition.HasState(key) {
			t.Errorf("builtin workflow is missing state %q", key)
		}
	}
	terminal := wf.Definition.TerminalStates()
	if len(terminal) != 2 || !wf.Definition.IsTerminal("done") || !wf.Definition.IsTerminal("cancelled") {
		t.Errorf("terminal states = %v, want done and cancelled", terminal)
	}

	p, err := l.GetProject(actorCtx, DefaultProjectKey)
	if err != nil {
		t.Fatalf("GetProject: %v", err)
	}
	if p.WorkflowID != wf.ID {
		t.Errorf("default project workflow = %q, want %q", p.WorkflowID, wf.ID)
	}
}

func TestEnsureDefaultsIsIdempotent(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	first, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("first EnsureDefaults: %v", err)
	}
	scope := core.TenantScope{TenantID: first.ID}
	events, audits := countRows(t, l, scope)

	second, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("second EnsureDefaults: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("second run created tenant %q, want %q", second.ID, first.ID)
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events || afterAudits != audits {
		t.Errorf("a repeat run wrote %d events and %d audit entries", afterEvents-events, afterAudits-audits)
	}

	admin := core.WithActor(ctx, core.SystemActor(first.ID))
	projects, _, err := l.ListProjects(admin, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 {
		t.Errorf("projects after two runs = %d, want 1", len(projects))
	}
}

func TestEnsureDefaultsRecordsWhatItCreated(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	events, audits := countRows(t, l, core.TenantScope{TenantID: tenant.ID})
	if events != audits {
		t.Errorf("bootstrap wrote %d events and %d audit entries; they must move together", events, audits)
	}
	if events != 3 {
		t.Errorf("bootstrap wrote %d records, want one each for the tenant, the workflow and the project", events)
	}
}

func TestBuiltinWorkflowValidates(t *testing.T) {
	in := core.WorkflowInput{Key: BuiltinWorkflowKey, Name: "Default", Definition: BuiltinWorkflow()}
	if err := in.Validate(); err != nil {
		t.Fatalf("the shipped workflow does not validate: %v", err)
	}
}

func TestDefaultTenantLookupOnAnEmptyDatabase(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	got, err := l.findDefaultTenant(ctx)
	if err != nil || got != nil {
		t.Fatalf("findDefaultTenant on an empty database = %+v, %v", got, err)
	}
	if _, err := l.requireDefaultTenant(ctx); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("requireDefaultTenant on an empty database = %v, want not found", err)
	}
}

// A deleted default tenant must not be silently resurrected under the same key.
func TestEnsureDefaultsRefusesADeletedDefaultTenant(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	admin := core.WithActor(ctx, core.SystemActor(tenant.ID))
	if err := l.DeleteTenant(admin, ""); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	if _, err := l.EnsureDefaults(ctx); !core.IsKind(err, core.KindConflict) {
		t.Errorf("EnsureDefaults after the default tenant was deleted = %v, want conflict", err)
	}
}
