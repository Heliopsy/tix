// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// statusFixture is one tenant with a project, on a fake clock, so every
// staleness judgement below is the test's own rather than the wall clock's.
type statusFixture struct {
	local   *Local
	clock   *clock.Fake
	scope   core.TenantScope
	actor   *core.Actor
	ctx     context.Context
	project *core.Project
}

func newStatusFixture(t *testing.T, opts ...Option) *statusFixture {
	t.Helper()
	l, clk, scope, actor := newLocalWith(t, opts...)
	return &statusFixture{
		local: l, clock: clk, scope: scope, actor: actor,
		ctx:     taskContext(actor),
		project: seedTaskProject(t, l, scope, "infra"),
	}
}

// register writes a server row directly, which is what a running process does.
func (f *statusFixture) register(t *testing.T, id, address string, lastSeen time.Time) {
	t.Helper()
	srv := core.Server{
		ID:         id,
		Address:    address,
		Version:    "9.9.9",
		Surfaces:   []core.ServerSurface{core.ServerSurfaceAPI, core.ServerSurfaceWeb},
		StartedAt:  lastSeen,
		LastSeenAt: lastSeen,
	}
	if err := f.local.store.Unscoped(context.Background(), func(u store.UnscopedTx) error {
		return u.RegisterServer(context.Background(), &srv)
	}); err != nil {
		t.Fatalf("registering %q: %v", id, err)
	}
}

func (f *statusFixture) status(t *testing.T) *core.StatusReport {
	t.Helper()
	got, err := f.local.Status(f.ctx)
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	return got
}

func TestStatusReportsTheInstallation(t *testing.T) {
	f := newStatusFixture(t)
	got := f.status(t)

	if got.Installation.SchemaVersion == 0 {
		t.Error("the report states no schema version")
	}
	if got.Installation.Engine != string(store.SQLite) {
		t.Errorf("engine = %q, want %q", got.Installation.Engine, store.SQLite)
	}
	if !got.Installation.ObservedAt.Equal(f.clock.Now()) {
		t.Errorf("observed at %v, clock says %v", got.Installation.ObservedAt, f.clock.Now())
	}
}

// TestStatusWithNoServerRunning is the ordinary single-user case: a database
// opened directly, with nothing serving it. The honest answer is none, not an
// error and not a refusal.
func TestStatusWithNoServerRunning(t *testing.T) {
	f := newStatusFixture(t)
	got := f.status(t)

	if got.Servers == nil {
		t.Fatal("the server list is nil; a caller ranging over it should not have to check")
	}
	if len(got.Servers) != 0 {
		t.Fatalf("an unserved database reported %d servers", len(got.Servers))
	}
	if got.AttachedCount() != 0 {
		t.Errorf("attached = %d, want 0", got.AttachedCount())
	}
}

// TestStatusCountsAttachedAndStale is the behaviour the whole design exists
// for. Nothing runs between the registration and the read: no sweeper, no
// pruner. The reader compares each row to its own clock.
func TestStatusCountsAttachedAndStale(t *testing.T) {
	f := newStatusFixture(t)
	now := f.clock.Now()

	f.register(t, "fresh000000000000000000001", "10.0.0.4:8080", now)
	f.register(t, "fresh000000000000000000002", "10.0.0.5:8080", now.Add(-core.ServerStaleAfter))
	f.register(t, "gone0000000000000000000003", "10.0.0.9:8080", now.Add(-core.ServerStaleAfter-time.Minute))

	got := f.status(t)
	if len(got.Servers) != 3 {
		t.Fatalf("listed %d servers, want 3: a stale one must be shown, not hidden", len(got.Servers))
	}
	if got.AttachedCount() != 2 {
		t.Errorf("attached = %d, want 2", got.AttachedCount())
	}

	byID := map[string]core.ServerStatus{}
	for _, s := range got.Servers {
		byID[s.ID] = s
	}
	for id, want := range map[string]bool{
		"fresh000000000000000000001": true,
		"fresh000000000000000000002": true,
		"gone0000000000000000000003": false,
	} {
		if byID[id].Attached != want {
			t.Errorf("%s attached = %v, want %v", id, byID[id].Attached, want)
		}
		if byID[id].LastSeenAt.IsZero() {
			t.Errorf("%s reports no last seen instant, so a consumer cannot judge for itself", id)
		}
	}
	if addr := byID["fresh000000000000000000001"].Address; addr != "10.0.0.4:8080" {
		t.Errorf("address = %q", addr)
	}
	if s := byID["fresh000000000000000000001"].Surfaces; len(s) != 2 {
		t.Errorf("surfaces = %v, want the two that were registered", s)
	}
}

// TestStatusKnowsNoConnectionCountWithoutAServer holds the honesty rule: a
// count belongs to the process holding the sockets, and this process is not one
// of the listed servers.
func TestStatusKnowsNoConnectionCountWithoutAServer(t *testing.T) {
	f := newStatusFixture(t)
	f.register(t, "other000000000000000000001", "10.0.0.4:8080", f.clock.Now())

	for _, s := range f.status(t).Servers {
		if s.Connections != nil {
			t.Errorf("%s reports %d connections, which no reader here can know", s.ID, *s.Connections)
		}
	}
}

// TestStatusAttributesItsOwnConnectionCount is the other half: when the
// answering process is one of the listed servers, its own count reaches the
// report and reaches no other row.
func TestStatusAttributesItsOwnConnectionCount(t *testing.T) {
	reg := connections.New(clockOption(clock.NewFakeAt()), connections.WithServerID("self00000000000000000001"))
	f := newStatusFixture(t, WithConnections(reg))

	f.register(t, "self00000000000000000001", "10.0.0.4:8080", f.clock.Now())
	f.register(t, "peer00000000000000000002", "10.0.0.5:8080", f.clock.Now())

	var closed string
	liveConnection(reg, f.scope.TenantID, f.actor.ID, core.ConnectionEvents, &closed)
	liveConnection(reg, f.scope.TenantID, f.actor.ID, core.ConnectionSSH, &closed)

	for _, s := range f.status(t).Servers {
		switch s.ID {
		case "self00000000000000000001":
			if s.Connections == nil || *s.Connections != 2 {
				t.Errorf("the answering server reports %v connections, want 2", s.Connections)
			}
		default:
			if s.Connections != nil {
				t.Errorf("%s reports a count it cannot know: %d", s.ID, *s.Connections)
			}
		}
	}
}

// clockOption exists only to keep the connections option spelled once here.
func clockOption(c *clock.Fake) connections.Option { return connections.WithClock(c) }

func TestStatusCountsTheTenantsOwnWork(t *testing.T) {
	f := newStatusFixture(t)
	for _, title := range []string{"one", "two", "three"} {
		if _, err := f.local.CreateTask(f.ctx, core.CreateTaskInput{
			ProjectRef: f.project.Key, Title: title,
		}); err != nil {
			t.Fatalf("creating %q: %v", title, err)
		}
	}

	got := f.status(t)
	if got.Work.Projects != 1 {
		t.Errorf("projects = %d, want 1", got.Work.Projects)
	}
	if got.Work.Tasks != 3 {
		t.Errorf("tasks = %d, want 3", got.Work.Tasks)
	}
	if got.Work.Tenants != 1 {
		t.Errorf("tenants = %d, want 1", got.Work.Tenants)
	}
}

// TestStatusWorkStaysInsideTheTenant is the isolation half. Both tenants are
// seeded with identical-looking work, so a leak is visible as a number rather
// than as a row somebody has to recognise.
func TestStatusWorkStaysInsideTheTenant(t *testing.T) {
	f := newStatusFixture(t)
	if _, err := f.local.CreateTask(f.ctx, core.CreateTaskInput{
		ProjectRef: f.project.Key, Title: "mine",
	}); err != nil {
		t.Fatalf("creating: %v", err)
	}

	other := f.otherTenant(t)
	for _, title := range []string{"theirs one", "theirs two", "theirs three"} {
		if _, err := other.local.CreateTask(other.ctx, core.CreateTaskInput{
			ProjectRef: other.project.Key, Title: title,
		}); err != nil {
			t.Fatalf("creating %q: %v", title, err)
		}
	}

	mine := f.status(t)
	if mine.Work.Tasks != 1 {
		t.Errorf("this tenant reports %d tasks; the other tenant's three have leaked in", mine.Work.Tasks)
	}
	theirs := other.status(t)
	if theirs.Work.Tasks != 3 {
		t.Errorf("the other tenant reports %d tasks, want 3", theirs.Work.Tasks)
	}
}

// TestStatusTenantCountMatchesWhatTheReaderCanList pins the one figure that
// crosses a tenant boundary to a capability the reader already has, so the
// report discloses nothing new.
func TestStatusTenantCountMatchesWhatTheReaderCanList(t *testing.T) {
	f := newStatusFixture(t)
	f.otherTenant(t)

	listed, _, err := f.local.ListTenants(f.ctx, core.Page{Limit: 100})
	if err != nil {
		t.Fatalf("listing tenants: %v", err)
	}
	if got := f.status(t).Work.Tenants; got != len(listed) {
		t.Errorf("the report states %d tenants, the reader can list %d", got, len(listed))
	}
}

// TestStatusCountsClaimsLazily holds the same rule the lease design holds: a
// claim past its expiry is not a claim, whether or not a sweeper has been.
func TestStatusCountsClaimsLazily(t *testing.T) {
	f := newStatusFixture(t)
	task, err := f.local.CreateTask(f.ctx, core.CreateTaskInput{
		ProjectRef: f.project.Key, Title: "claimed",
	})
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	ttl := core.Duration(time.Minute)
	if _, err := f.local.ClaimTask(f.ctx, core.TaskRef{ID: task.ID}, core.ClaimInput{TTL: ttl}); err != nil {
		t.Fatalf("claiming: %v", err)
	}

	held := f.status(t)
	if held.Work.Claimed != 1 || held.Work.LeasesExpiredUnswept != 0 {
		t.Fatalf("while held: claimed %d, unswept %d, want 1 and 0",
			held.Work.Claimed, held.Work.LeasesExpiredUnswept)
	}

	// Nothing sweeps. The clock simply passes the expiry.
	f.clock.Advance(2 * time.Minute)

	lapsed := f.status(t)
	if lapsed.Work.Claimed != 0 {
		t.Errorf("an expired claim still counts as claimed: %d", lapsed.Work.Claimed)
	}
	if lapsed.Work.LeasesExpiredUnswept != 1 {
		t.Errorf("unswept = %d, want 1", lapsed.Work.LeasesExpiredUnswept)
	}
}

func TestStatusRefusesAReaderWithoutTenantAdmin(t *testing.T) {
	f := newStatusFixture(t)
	viewer := seedTaskActor(t, f.local, f.scope, "viewer", core.RoleViewer, core.ScopeTaskRead)

	if _, err := f.local.Status(taskContext(viewer)); !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("Status() as a viewer = %v, want forbidden", err)
	}
}

// TestStatusWritesNoAuditEntryOrEvent states what reading is: a read. The
// registration path is covered where it lives, in internal/presence, because
// nothing about it goes through the service.
func TestStatusWritesNoAuditEntryOrEvent(t *testing.T) {
	f := newStatusFixture(t)
	f.register(t, "srv000000000000000000001", "10.0.0.4:8080", f.clock.Now())

	beforeEvents, beforeAudits := countRows(t, f.local, f.scope)
	f.status(t)
	afterEvents, afterAudits := countRows(t, f.local, f.scope)

	if afterEvents != beforeEvents || afterAudits != beforeAudits {
		t.Errorf("reading the status wrote %d events and %d audit entries",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
}

// otherTenant builds a second tenant on the same store, deliberately
// indistinguishable from the first by every value a reader could confuse.
func (f *statusFixture) otherTenant(t *testing.T) *statusFixture {
	t.Helper()
	ctx := context.Background()
	tenant := core.Tenant{Key: "other", Name: "Other"}
	if err := f.local.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &tenant)
	}); err != nil {
		t.Fatalf("creating the second tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	actor := seedTaskActor(t, f.local, scope, f.actor.Handle, core.RoleAdmin, core.ScopeAll)
	return &statusFixture{
		local: f.local, clock: f.clock, scope: scope, actor: actor,
		ctx:     taskContext(actor),
		project: seedTaskProject(t, f.local, scope, f.project.Key),
	}
}
