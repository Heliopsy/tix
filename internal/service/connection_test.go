package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/connections"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// newLocalWithConnections builds a service over its own registry, so one test
// never sees another's connections.
func newLocalWithConnections(t *testing.T) (*Local, *connections.Registry, *core.Actor) {
	t.Helper()
	reg := connections.New(connections.WithClock(clock.NewFakeAt()), connections.WithServerID("srv-test"))
	l, _, _, actor := newLocalWith(t, WithConnections(reg))
	return l, reg, actor
}

// liveConnection registers one connection and reports the reason it was closed
// with, which stays empty until something ends it.
func liveConnection(reg *connections.Registry, tenantID, actorID string, surface core.ConnectionSurface, closed *string) *connections.Handle {
	return reg.Register(connections.Entry{
		Surface: surface, TenantID: tenantID, ActorID: actorID, ActorHandle: actorID, Remote: "203.0.113.9",
	}, func(reason string) error {
		*closed = reason
		return nil
	})
}

// auditEntries returns every audit entry recorded for the tenant.
func auditEntries(t *testing.T, l *Local, actor *core.Actor) []core.AuditEntry {
	t.Helper()
	var out []core.AuditEntry
	if err := l.read(context.Background(), actor, func(tx store.Tx) error {
		got, err := tx.ListAudit(context.Background(), core.AuditFilter{Page: core.Page{Limit: 100}})
		out = got
		return err
	}); err != nil {
		t.Fatalf("reading audit: %v", err)
	}
	return out
}

func TestListConnectionsReportsBothSurfacesAndTheServer(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var closed string
	liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &closed)
	liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionSSH, &closed)

	got, err := l.ListConnections(ctx)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if got.ServerID != "srv-test" {
		t.Errorf("the response does not name the server that answered: %q", got.ServerID)
	}
	if len(got.Connections) != 2 {
		t.Fatalf("listed %d connections, want 2", len(got.Connections))
	}
	surfaces := map[core.ConnectionSurface]bool{}
	for _, c := range got.Connections {
		surfaces[c.Surface] = true
		if c.ActorID != actor.ID || c.Since.IsZero() {
			t.Errorf("connection %+v does not name its actor and when it opened", c)
		}
	}
	if !surfaces[core.ConnectionEvents] || !surfaces[core.ConnectionSSH] {
		t.Errorf("both surfaces are not listed: %+v", got.Connections)
	}
	if got.Counts.Events != 1 || got.Counts.SSH != 1 || got.Counts.Tenant != 2 {
		t.Errorf("per-surface counts = %+v", got.Counts)
	}
}

func TestListConnectionsWithNothingLiveIsEmptyNotAnError(t *testing.T) {
	l, _, actor := newLocalWithConnections(t)
	got, err := l.ListConnections(core.WithActor(context.Background(), actor))
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(got.Connections) != 0 || got.Counts.Process != 0 {
		t.Errorf("an idle server answered %+v", got)
	}
}

func TestAClosedConnectionIsGoneFromTheList(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var closed string
	h := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &closed)
	h.Unregister()

	got, err := l.ListConnections(ctx)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(got.Connections) != 0 {
		t.Errorf("a connection that closed is still listed: %+v", got.Connections)
	}
}

func TestEndConnectionClosesItAndRecordsTheCut(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var ended, spared string
	target := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &ended)
	other := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionSSH, &spared)

	before := len(auditEntries(t, l, actor))
	if err := l.EndConnection(ctx, target.ID()); err != nil {
		t.Fatalf("EndConnection: %v", err)
	}
	if ended == "" {
		t.Error("the connection was not closed")
	}
	if spared != "" {
		t.Errorf("another connection was closed with %q", spared)
	}
	if _, ok := reg.Lookup(actor.TenantID, other.ID()); !ok {
		t.Error("a connection nobody named is gone")
	}

	entries := auditEntries(t, l, actor)
	if len(entries) != before+1 {
		t.Fatalf("EndConnection wrote %d audit entries, want one", len(entries)-before)
	}
	found := false
	for _, e := range entries {
		if e.Action == auditConnectionEnd && e.SubjectID == target.ID() {
			found = true
			if e.ActorID != actor.ID {
				t.Errorf("the audit entry names caller %q, want %q", e.ActorID, actor.ID)
			}
			if !strings.Contains(string(e.After), actor.ID) {
				t.Errorf("the audit entry does not name the actor ended: %s", e.After)
			}
			if !strings.Contains(string(e.After), string(core.ConnectionEvents)) {
				t.Errorf("the audit entry does not name the surface: %s", e.After)
			}
		}
	}
	if !found {
		t.Error("ending a connection recorded no audit entry naming it")
	}
}

// The audit entry and the event commit before the socket closes, so a cut that
// fails still leaves a record of the attempt.
func TestTheRecordSurvivesACloseThatFailed(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	h := reg.Register(connections.Entry{
		Surface: core.ConnectionSSH, TenantID: actor.TenantID, ActorID: actor.ID,
	}, func(string) error { return errors.New("socket already gone") })

	before := len(auditEntries(t, l, actor))
	if err := l.EndConnection(ctx, h.ID()); err == nil {
		t.Fatal("EndConnection hid a close that failed")
	}
	if got := len(auditEntries(t, l, actor)); got != before+1 {
		t.Errorf("a failed close left %d audit entries, want one", got-before)
	}
}

func TestEndConnectionWritesAnEventBesideTheAuditEntry(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var closed string
	h := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &closed)

	scope := core.TenantScope{TenantID: actor.TenantID}
	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.EndConnection(ctx, h.ID()); err != nil {
		t.Fatalf("EndConnection: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("EndConnection wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}
}

func TestEndConnectionRejectsAnIdentifierThatIsNotLive(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var closed string
	survivor := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &closed)

	if err := l.EndConnection(ctx, "01JNOTHINGHERE0000000"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("ending an unknown identifier = %v, want not found", err)
	}
	if err := l.EndConnection(ctx, "  "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("ending an empty identifier = %v, want invalid", err)
	}
	if closed != "" {
		t.Error("a request that matched nothing closed something")
	}
	if _, ok := reg.Lookup(actor.TenantID, survivor.ID()); !ok {
		t.Error("a request that matched nothing removed a connection")
	}
}

// Ending is not revocation: the holder reconnects and is accepted.
func TestEndingIsNotRevocation(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var first string
	h := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &first)
	if err := l.EndConnection(ctx, h.ID()); err != nil {
		t.Fatalf("EndConnection: %v", err)
	}

	var second string
	again := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &second)
	got, err := l.ListConnections(ctx)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(got.Connections) != 1 || got.Connections[0].ID != again.ID() {
		t.Errorf("the holder's new connection was not accepted: %+v", got.Connections)
	}
	if second != "" {
		t.Error("the new connection was closed by the earlier end")
	}
}

// Another tenant's connections are absent from the list and from the tenant
// counts, and their identifiers, actors and addresses are not disclosed.
func TestAnotherTenantsConnectionsAreInvisible(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var theirs string
	other := reg.Register(connections.Entry{
		Surface: core.ConnectionSSH, TenantID: "other-tenant", ActorID: "spy",
		ActorHandle: "spy", Remote: "198.51.100.4", Fingerprint: "SHA256:theirs",
	}, func(reason string) error {
		theirs = reason
		return nil
	})
	var mine string
	liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &mine)

	got, err := l.ListConnections(ctx)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(got.Connections) != 1 || got.Connections[0].TenantID != actor.TenantID {
		t.Fatalf("the list crosses tenants: %+v", got.Connections)
	}
	for _, secret := range []string{other.ID(), "spy", "198.51.100.4", "SHA256:theirs", "other-tenant"} {
		for _, c := range got.Connections {
			if strings.Contains(c.ID+c.ActorID+c.ActorHandle+c.Remote+c.Fingerprint+c.TenantID, secret) {
				t.Errorf("the listing discloses %q", secret)
			}
		}
	}
	if got.Counts.Tenant != 1 || got.Counts.SSH != 0 {
		t.Errorf("the tenant counts include another tenant's connections: %+v", got.Counts)
	}
	if theirs != "" {
		t.Error("listing closed another tenant's connection")
	}
}

// The process total covers every connection and carries no breakdown.
func TestTheProcessTotalCountsEveryoneAndNamesNobody(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var ignored string
	liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &ignored)
	liveConnection(reg, "other-tenant", "spy", core.ConnectionEvents, &ignored)
	liveConnection(reg, "third-tenant", "ghost", core.ConnectionSSH, &ignored)

	got, err := l.ListConnections(ctx)
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if got.Counts.Process != 3 {
		t.Errorf("process total = %d, want every connection the server holds", got.Counts.Process)
	}
	if got.Counts.Tenant != 1 || got.Counts.Events != 1 || got.Counts.SSH != 0 {
		t.Errorf("tenant counts = %+v, want this tenant's one event stream", got.Counts)
	}
	if len(got.Connections) != 1 {
		t.Errorf("the process total came with %d connections, want only this tenant's", len(got.Connections))
	}
}

// Naming another tenant's identifier reports that nothing matched. Forbidden
// would confirm that it exists.
func TestAnotherTenantsConnectionCannotBeEndedAndIsNotFound(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	ctx := core.WithActor(context.Background(), actor)
	var theirs string
	other := reg.Register(connections.Entry{
		Surface: core.ConnectionEvents, TenantID: "other-tenant", ActorID: "spy",
	}, func(reason string) error {
		theirs = reason
		return nil
	})

	before := len(auditEntries(t, l, actor))
	err := l.EndConnection(ctx, other.ID())
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("ending another tenant's connection = %v, want not found", err)
	}
	if core.IsKind(err, core.KindForbidden) {
		t.Error("the refusal confirms the identifier exists")
	}
	if strings.Contains(err.Error(), "other-tenant") || strings.Contains(err.Error(), "spy") {
		t.Errorf("the refusal discloses the connection: %v", err)
	}
	if theirs != "" {
		t.Error("another tenant's connection was closed")
	}
	if _, ok := reg.Lookup("other-tenant", other.ID()); !ok {
		t.Error("another tenant's connection was removed")
	}
	if got := len(auditEntries(t, l, actor)); got != before {
		t.Errorf("a request that matched nothing wrote %d audit entries", got-before)
	}
}

func TestConnectionMethodsRequireTenantAdmin(t *testing.T) {
	l, reg, actor := newLocalWithConnections(t)
	var closed string
	h := liveConnection(reg, actor.TenantID, actor.ID, core.ConnectionEvents, &closed)

	viewer := &core.Actor{ID: "v1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	ctx := core.WithActor(context.Background(), viewer)
	if _, err := l.ListConnections(ctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a viewer listing connections = %v, want forbidden", err)
	}
	if err := l.EndConnection(ctx, h.ID()); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a viewer ending a connection = %v, want forbidden", err)
	}
	if closed != "" {
		t.Error("a viewer's refused request still closed the connection")
	}

	anon := context.Background()
	if _, err := l.ListConnections(anon); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("an unauthenticated listing = %v, want unauthenticated", err)
	}
	if err := l.EndConnection(anon, h.ID()); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("an unauthenticated end = %v, want unauthenticated", err)
	}
}
