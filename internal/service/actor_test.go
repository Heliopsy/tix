package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestGetActorResolvesAHandle(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	got, err := l.GetActor(ctx, admin.ID)
	if err != nil {
		t.Fatalf("GetActor: %v", err)
	}
	if got.Handle != "alice" {
		t.Errorf("handle = %q, want alice", got.Handle)
	}
	if got.ID != admin.ID {
		t.Errorf("id = %q, want %q", got.ID, admin.ID)
	}
}

// A directory lookup names an actor. It must not double as a way of reading
// what that actor is allowed to do.
func TestGetActorDisclosesNoAuthority(t *testing.T) {
	l, _, _, admin := newLocal(t)

	got, err := l.GetActor(authContext(admin), admin.ID)
	if err != nil {
		t.Fatalf("GetActor: %v", err)
	}
	if len(got.Scopes) != 0 {
		t.Errorf("scopes = %v, want none", got.Scopes)
	}
	if got.Role != "" || got.TokenID != "" {
		t.Errorf("role = %q, token = %q, want neither", got.Role, got.TokenID)
	}
}

// Reading the directory is what every reader of a record needs, so it asks for
// no administrative scope.
func TestGetActorNeedsNoAdministrativeScope(t *testing.T) {
	l, _, _, admin := newLocal(t)
	viewer := &core.Actor{ID: admin.ID, TenantID: admin.TenantID, Kind: core.ActorUser,
		Handle: "alice", Role: core.RoleViewer, Scopes: core.RoleViewer.Scopes()}

	if _, err := l.GetActor(authContext(viewer), admin.ID); err != nil {
		t.Fatalf("a viewer could not resolve an actor: %v", err)
	}
}

func TestGetActorIsTenantScoped(t *testing.T) {
	l, _, _, admin := newLocal(t)
	stranger := otherTenantActor(t, l)

	if _, err := l.GetActor(authContext(stranger), admin.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("an actor of another tenant = %v, want not found", err)
	}
	if _, err := l.GetActor(authContext(admin), stranger.ID); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("resolving across the tenant boundary = %v, want not found", err)
	}
}

func TestGetActorRejectsBadInput(t *testing.T) {
	l, _, _, admin := newLocal(t)

	if _, err := l.GetActor(authContext(admin), "  "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty identifier = %v, want invalid", err)
	}
	if _, err := l.GetActor(authContext(admin), "MISSINGMISSINGMISSING12345"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("an unknown identifier = %v, want not found", err)
	}
	if _, err := l.GetActor(context.Background(), admin.ID); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("an unauthenticated caller = %v, want unauthenticated", err)
	}
}

// Imports, sweeps and pruning act as "system", which owns no actor row. It
// still has to read as a word rather than as an unresolved identifier.
func TestGetActorNamesTheSystemActor(t *testing.T) {
	l, _, _, admin := newLocal(t)

	got, err := l.GetActor(authContext(admin), "system")
	if err != nil {
		t.Fatalf("GetActor(system): %v", err)
	}
	if got.Handle != "system" || got.Kind != core.ActorSystem {
		t.Errorf("system resolved to %q/%q", got.Handle, got.Kind)
	}
}
