package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/thereisnotime/tix/internal/auth"
	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/sqlite"
)

func newLocal(t *testing.T) (*Local, *clock.Fake, core.TenantScope, *core.Actor) {
	t.Helper()
	return newLocalWith(t)
}

// newLocalWith builds a service with extra options. Tests use cheap argon2id
// parameters: production cost is deliberately high, and paying it in every test
// made the package take a minute under the race detector.
func newLocalWith(t *testing.T, opts ...Option) (*Local, *clock.Fake, core.TenantScope, *core.Actor) {
	t.Helper()
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

	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}

	actor := core.Actor{Kind: core.ActorUser, Handle: "alice", Scopes: []core.Scope{core.ScopeAll}}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("creating actor: %v", err)
	}
	actor.TenantID = tenant.ID

	all := append([]Option{
		WithClock(clk),
		WithHasher(auth.NewHasherWithParams(auth.TestParams())),
	}, opts...)
	return New(st, all...), clk, scope, &actor
}

func countRows(t *testing.T, l *Local, scope core.TenantScope) (events, audits int) {
	t.Helper()
	ctx := context.Background()
	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		evs, err := tx.ReadEvents(ctx, 0, 1000)
		if err != nil {
			return err
		}
		events = len(evs)
		ents, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 1000}})
		if err != nil {
			return err
		}
		audits = len(ents)
		return nil
	}); err != nil {
		t.Fatalf("counting: %v", err)
	}
	return events, audits
}

// The service-layer spec requires the domain rows, the audit entry and the
// outbox event to commit together, and requires that a failure leaves none of
// them behind.
func TestMutationCommitsRowsAuditAndEventTogether(t *testing.T) {
	ctx := context.Background()
	l, _, scope, actor := newLocal(t)

	beforeEvents, beforeAudits := countRows(t, l, scope)

	wf := core.Workflow{Key: "default", Name: "Default", Definition: core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
	}}
	err := l.write(core.WithActor(ctx, actor), actor, func(m *mutation) error {
		if err := m.tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		return m.Record("workflow.create", core.EventWorkflowUpdated,
			"workflow", wf.ID, "", nil, wf, map[string]any{"key": wf.Key})
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 {
		t.Errorf("events = %d, want %d", afterEvents, beforeEvents+1)
	}
	if afterAudits != beforeAudits+1 {
		t.Errorf("audit entries = %d, want %d", afterAudits, beforeAudits+1)
	}

	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		got, err := tx.GetWorkflow(ctx, "default")
		if err != nil {
			return err
		}
		if got.ID != wf.ID {
			t.Errorf("workflow not committed: %+v", got)
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back: %v", err)
	}
}

func TestFailedMutationLeavesNothingBehind(t *testing.T) {
	ctx := context.Background()
	l, _, scope, actor := newLocal(t)

	beforeEvents, beforeAudits := countRows(t, l, scope)
	boom := errors.New("deliberate failure")

	wf := core.Workflow{Key: "doomed", Name: "Doomed", Definition: core.WorkflowDefinition{
		Initial: "todo",
		States:  []core.State{{Key: "todo"}, {Key: "done", Terminal: true}},
	}}
	err := l.write(core.WithActor(ctx, actor), actor, func(m *mutation) error {
		if err := m.tx.PutWorkflow(ctx, &wf); err != nil {
			return err
		}
		if err := m.Record("workflow.create", core.EventWorkflowUpdated,
			"workflow", wf.ID, "", nil, wf, nil); err != nil {
			return err
		}
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("write error = %v, want the deliberate failure", err)
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents {
		t.Errorf("a failed mutation left %d events behind", afterEvents-beforeEvents)
	}
	if afterAudits != beforeAudits {
		t.Errorf("a failed mutation left %d audit entries behind", afterAudits-beforeAudits)
	}

	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		if _, err := tx.GetWorkflow(ctx, "doomed"); err == nil {
			t.Error("a failed mutation committed its domain row")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading back: %v", err)
	}
}

// Every mutation must produce both an audit entry and an event, never one
// alone, or history and the event stream disagree about what happened.
func TestRecordWritesBothOrNeither(t *testing.T) {
	ctx := context.Background()
	l, _, scope, actor := newLocal(t)

	beforeEvents, beforeAudits := countRows(t, l, scope)

	err := l.write(core.WithActor(ctx, actor), actor, func(m *mutation) error {
		return m.Record("noop", core.EventTaskUpdated, "task", "t1", "", nil, nil, nil)
	})
	if err != nil {
		t.Fatalf("write: %v", err)
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-beforeEvents != afterAudits-beforeAudits {
		t.Errorf("event count grew by %d but audit count grew by %d; they must move together",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
}

func TestAuditRecordsSourceAndActor(t *testing.T) {
	ctx := core.WithSource(context.Background(), core.SourceCLI)
	l, _, scope, actor := newLocal(t)

	if err := l.write(core.WithActor(ctx, actor), actor, func(m *mutation) error {
		return m.Record("task.create", core.EventTaskCreated, "task", "t1", "", nil,
			map[string]any{"title": "x"}, nil)
	}); err != nil {
		t.Fatalf("write: %v", err)
	}

	if err := l.store.View(ctx, scope, func(tx store.Tx) error {
		entries, err := tx.ListAudit(ctx, core.AuditFilter{Page: core.Page{Limit: 10}})
		if err != nil {
			return err
		}
		if len(entries) == 0 {
			t.Fatal("no audit entries written")
		}
		e := entries[len(entries)-1]
		if e.ActorID != actor.ID {
			t.Errorf("audit actor = %q, want %q", e.ActorID, actor.ID)
		}
		if e.Source != core.SourceCLI {
			t.Errorf("audit source = %q, want %q", e.Source, core.SourceCLI)
		}
		if e.OccurredAt.IsZero() {
			t.Error("audit entry has no timestamp")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading audit: %v", err)
	}
}

func TestWhoAmIRequiresActor(t *testing.T) {
	l, _, _, actor := newLocal(t)

	if _, err := l.WhoAmI(context.Background()); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("WhoAmI with no actor = %v, want unauthenticated", err)
	}
	got, err := l.WhoAmI(core.WithActor(context.Background(), actor))
	if err != nil || got.ID != actor.ID {
		t.Errorf("WhoAmI = %+v, %v", got, err)
	}
}

// authorize is the single point where the policy is consulted. It must reject
// an unauthenticated caller before any store access, and must default the
// resource tenant to the actor's own rather than trusting a caller-supplied one.
func TestAuthorizeRequiresActorAndScope(t *testing.T) {
	l, _, _, actor := newLocal(t)

	if _, err := l.authorize(context.Background(), authz.ActionTaskRead, authz.Resource{}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("authorize with no actor = %v, want unauthenticated", err)
	}

	ctx := core.WithActor(context.Background(), actor)
	got, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		t.Fatalf("authorize: %v", err)
	}
	if got.ID != actor.ID {
		t.Errorf("authorize returned actor %q, want %q", got.ID, actor.ID)
	}
}

func TestAuthorizeDeniesMissingScope(t *testing.T) {
	l, _, _, _ := newLocal(t)

	viewer := &core.Actor{ID: "v1", TenantID: "t1", Kind: core.ActorUser, Role: core.RoleViewer}
	ctx := core.WithActor(context.Background(), viewer)

	if _, err := l.authorize(ctx, authz.ActionTaskDelete, authz.Resource{TenantID: "t1"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer deleting a task = %v, want forbidden", err)
	}
	if _, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{TenantID: "t1"}); err != nil {
		t.Errorf("viewer reading a task = %v, want allowed", err)
	}
}

// A resource in another tenant reports not found, so the API never confirms
// that another tenant's record exists.
func TestAuthorizeCrossTenantReportsNotFound(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	_, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{TenantID: "another-tenant"})
	if !core.IsKind(err, core.KindNotFound) {
		t.Errorf("cross-tenant authorize = %v, want not found", err)
	}
}

func TestReadOpensAScopedTransaction(t *testing.T) {
	ctx := context.Background()
	l, _, _, actor := newLocal(t)

	var seen core.TenantScope
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		seen = tx.Scope()
		return nil
	}); err != nil {
		t.Fatalf("read: %v", err)
	}
	if seen.TenantID != actor.TenantID {
		t.Errorf("read scope = %q, want the actor's tenant %q", seen.TenantID, actor.TenantID)
	}
}

func TestReadPropagatesError(t *testing.T) {
	l, _, _, actor := newLocal(t)
	boom := errors.New("read failure")

	if err := l.read(context.Background(), actor, func(store.Tx) error { return boom }); !errors.Is(err, boom) {
		t.Errorf("read error = %v, want the propagated failure", err)
	}
}
