package service

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store/sqlite"
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
	if len(projects) != len(starterProjects) {
		t.Errorf("projects after two runs = %d, want %d", len(projects), len(starterProjects))
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
	if want := 2 + len(starterProjects); events != want {
		t.Errorf("bootstrap wrote %d records, want %d: the tenant, the workflow and one per starter list",
			events, want)
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

// A fresh installation opens on a workspace, not on an empty table.
func TestEnsureDefaultsSeedsTheStarterLists(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	admin := core.WithActor(ctx, core.SystemActor(tenant.ID))
	projects, _, err := l.ListProjects(admin, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}

	byKey := map[string]core.Project{}
	for _, p := range projects {
		byKey[p.Key] = p
	}
	for _, want := range []string{DefaultProjectKey, "work", "homelab", "house"} {
		p, ok := byKey[want]
		if !ok {
			t.Errorf("a fresh installation has no %q list", want)
			continue
		}
		if p.Color == core.ColorNone || p.Icon == "" {
			t.Errorf("list %q has no colour or icon, so it is not distinguishable", want)
		}
	}
	seen := map[core.ProjectColor]bool{}
	for _, p := range projects {
		if seen[p.Color] {
			t.Errorf("two starter lists share the colour %q", p.Color)
		}
		seen[p.Color] = true
	}
}

// Seeding is tied to creating the tenant, not to a list being absent, so a
// list somebody deleted on purpose stays deleted.
func TestEnsureDefaultsDoesNotResurrectADeletedList(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	admin := core.WithActor(ctx, core.SystemActor(tenant.ID))
	if err := l.DeleteProject(admin, "homelab"); err != nil {
		t.Fatalf("DeleteProject: %v", err)
	}
	if _, err := l.EnsureDefaults(ctx); err != nil {
		t.Fatalf("second EnsureDefaults: %v", err)
	}
	if _, err := l.GetProject(admin, "homelab"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a deleted list came back on the next dial: %v", err)
	}
}

// An installation that predates the starter lists is left exactly as it is.
func TestEnsureDefaultsLeavesAnExistingTenantAlone(t *testing.T) {
	ctx := context.Background()
	l := newEmpty(t)

	// Stand in for the older shape: the tenant and the workflow exist, and the
	// only project is the default one.
	tenant, fresh, err := l.ensureDefaultTenant(ctx)
	if err != nil || !fresh {
		t.Fatalf("ensureDefaultTenant = %v, fresh %v", err, fresh)
	}
	admin := core.WithActor(ctx, core.SystemActor(tenant.ID))
	if err := l.write(ctx, core.SystemActor(tenant.ID), func(m *mutation) error {
		_, err := ensureBuiltinWorkflow(ctx, m)
		return err
	}); err != nil {
		t.Fatalf("seeding the workflow: %v", err)
	}

	if _, err := l.EnsureDefaults(ctx); err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	projects, _, err := l.ListProjects(admin, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("an existing tenant gained %d lists on upgrade", len(projects))
	}
}

// An operator installing tix for one purpose can keep it empty.
func TestStarterListsAreSuppressible(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	l := New(st, WithClock(clk), WithoutStarterProjects())

	tenant, err := l.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("EnsureDefaults: %v", err)
	}
	admin := core.WithActor(ctx, core.SystemActor(tenant.ID))
	projects, _, err := l.ListProjects(admin, core.ProjectFilter{})
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(projects) != 1 || projects[0].Key != DefaultProjectKey {
		t.Errorf("suppressed starter lists left %d projects, want only %q", len(projects), DefaultProjectKey)
	}
}
