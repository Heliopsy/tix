package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// seedActor creates a second actor in the tenant, for membership tests.
func seedActor(t *testing.T, l *Local, scope core.TenantScope, handle string) *core.Actor {
	t.Helper()
	ctx := context.Background()
	a := &core.Actor{Kind: core.ActorAgent, Handle: handle}
	if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateActor(ctx, a)
	}); err != nil {
		t.Fatalf("creating actor %q: %v", handle, err)
	}
	a.TenantID = scope.TenantID
	return a
}

func TestCreateTenantRecordsRowAuditAndEvent(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	events, audits := countRows(t, l, scope)

	got, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if got.ID == "" || got.Key != "beta" {
		t.Errorf("created tenant = %+v", got)
	}

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Errorf("CreateTenant wrote %d events and %d audit entries, want one of each",
			afterEvents-events, afterAudits-audits)
	}
}

func TestCreateTenantRejections(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	tests := []struct {
		name string
		in   core.CreateTenantInput
		kind core.Kind
	}{
		{"duplicate key", core.CreateTenantInput{Key: "acme", Name: "Copy"}, core.KindConflict},
		{"empty key", core.CreateTenantInput{Key: "", Name: "Nameless"}, core.KindInvalid},
		{"bad key", core.CreateTenantInput{Key: "9lives", Name: "Nine"}, core.KindInvalid},
		{"no name", core.CreateTenantInput{Key: "gamma"}, core.KindInvalid},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := l.CreateTenant(ctx, tc.in); !core.IsKind(err, tc.kind) {
				t.Errorf("CreateTenant = %v, want %s", err, tc.kind)
			}
		})
	}
}

func TestCreateTenantRequiresAuthentication(t *testing.T) {
	l, _, _, _ := newLocal(t)
	if _, err := l.CreateTenant(context.Background(), core.CreateTenantInput{Key: "beta", Name: "Beta"}); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("CreateTenant without an actor = %v, want unauthenticated", err)
	}
}

func TestGetTenantResolvesByIDKeyAndDefault(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	for _, ref := range []string{"", actor.TenantID, "acme", "ACME"} {
		got, err := l.GetTenant(ctx, ref)
		if err != nil {
			t.Fatalf("GetTenant(%q): %v", ref, err)
		}
		if got.ID != actor.TenantID {
			t.Errorf("GetTenant(%q) = %q, want %q", ref, got.ID, actor.TenantID)
		}
	}
}

// Another tenant must be indistinguishable from one that does not exist.
func TestGetTenantCrossTenantReportsNotFound(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	other, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"})
	if err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	for _, ref := range []string{other.ID, other.Key, "nosuchtenant"} {
		if _, err := l.GetTenant(ctx, ref); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("GetTenant(%q) = %v, want not found", ref, err)
		}
	}
}

func TestListTenantsReturnsOnlyTheActorsOwn(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"}); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	got, cursor, err := l.ListTenants(ctx, core.Page{})
	if err != nil {
		t.Fatalf("ListTenants: %v", err)
	}
	if len(got) != 1 || got[0].ID != actor.TenantID {
		t.Errorf("ListTenants = %+v, want only the actor's own tenant", got)
	}
	if cursor != "" {
		t.Errorf("cursor = %q, want empty", cursor)
	}
	if _, _, err := l.ListTenants(ctx, core.Page{Limit: -1}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("ListTenants with a negative limit = %v, want invalid", err)
	}
}

func TestUpdateTenant(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	events, audits := countRows(t, l, scope)

	name := "Acme Incorporated"
	got, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{Name: &name})
	if err != nil {
		t.Fatalf("UpdateTenant: %v", err)
	}
	if got.Name != name {
		t.Errorf("name = %q, want %q", got.Name, name)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("UpdateTenant did not write both an audit entry and an event")
	}

	empty := "  "
	if _, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{Name: &empty}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("UpdateTenant with a blank name = %v, want invalid", err)
	}
	if _, err := l.UpdateTenant(ctx, "nosuchtenant", core.UpdateTenantInput{Name: &name}); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("UpdateTenant on an unknown tenant = %v, want not found", err)
	}
}

// Deleting a tenant is a soft delete: it stops being reachable but nothing is
// destroyed, and its domains stop resolving.
func TestDeleteTenantSoftDeletes(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.AddDomain(ctx, core.AddDomainInput{Hostname: "acme.example.com"}); err != nil {
		t.Fatalf("AddDomain: %v", err)
	}
	if err := l.DeleteTenant(ctx, ""); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	if _, err := l.GetTenant(ctx, ""); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetTenant after deletion = %v, want not found", err)
	}
	if _, err := l.ResolveDomain(context.Background(), "acme.example.com"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("ResolveDomain for a deleted tenant = %v, want not found", err)
	}
	if err := l.DeleteTenant(ctx, "nosuchtenant"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("DeleteTenant on an unknown tenant = %v, want not found", err)
	}
}

func TestMembershipLifecycle(t *testing.T) {
	l, _, scope, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)
	worker := seedActor(t, l, scope, "worker")

	events, audits := countRows(t, l, scope)
	mem, err := l.AddMember(ctx, worker.ID, core.RoleMember)
	if err != nil {
		t.Fatalf("AddMember: %v", err)
	}
	if mem.Role != core.RoleMember || mem.TenantID != scope.TenantID {
		t.Errorf("membership = %+v", mem)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != events+1 || afterAudits != audits+1 {
		t.Error("AddMember did not write both an audit entry and an event")
	}

	members, err := l.ListMembers(ctx)
	if err != nil {
		t.Fatalf("ListMembers: %v", err)
	}
	if len(members) != 1 || members[0].ActorID != worker.ID {
		t.Errorf("ListMembers = %+v", members)
	}

	if _, err := l.AddMember(ctx, worker.ID, core.RoleAdmin); !core.IsKind(err, core.KindConflict) {
		t.Errorf("adding a member twice = %v, want conflict", err)
	}
	if err := l.RemoveMember(ctx, worker.ID); err != nil {
		t.Fatalf("RemoveMember: %v", err)
	}
	if err := l.RemoveMember(ctx, worker.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("removing a member twice = %v, want not found", err)
	}
}

func TestAddMemberRejections(t *testing.T) {
	l, _, _, actor := newLocal(t)
	ctx := core.WithActor(context.Background(), actor)

	if _, err := l.AddMember(ctx, "", core.RoleMember); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("AddMember with no actor = %v, want invalid", err)
	}
	if _, err := l.AddMember(ctx, actor.ID, core.Role("owner")); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("AddMember with an unknown role = %v, want invalid", err)
	}
	if _, err := l.AddMember(ctx, "MISSINGACTOR000000000000AA", core.RoleMember); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("AddMember for an unknown actor = %v, want not found", err)
	}
}

func TestTenantMethodsRequireTenantAdminScope(t *testing.T) {
	l, _, _, actor := newLocal(t)
	viewer := &core.Actor{ID: "v1", TenantID: actor.TenantID, Kind: core.ActorUser, Role: core.RoleViewer}
	ctx := core.WithActor(context.Background(), viewer)

	if _, err := l.CreateTenant(ctx, core.CreateTenantInput{Key: "beta", Name: "Beta"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer creating a tenant = %v, want forbidden", err)
	}
	if _, err := l.ListMembers(ctx); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer listing members = %v, want forbidden", err)
	}
	if err := l.RemoveMember(ctx, "anyone"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer removing a member = %v, want forbidden", err)
	}
	if _, err := l.UpdateTenant(ctx, "", core.UpdateTenantInput{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer updating a tenant = %v, want forbidden", err)
	}
	if err := l.DeleteTenant(ctx, ""); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer deleting a tenant = %v, want forbidden", err)
	}
	if _, err := l.GetTenant(ctx, ""); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer reading a tenant = %v, want forbidden", err)
	}
	if _, _, err := l.ListTenants(ctx, core.Page{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("viewer listing tenants = %v, want forbidden", err)
	}
}
