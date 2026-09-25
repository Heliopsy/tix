// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
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

// GetActor used to resolve only an identifier, even though actors also carry
// a handle a human would type. It now resolves either, mirroring how a
// project reference resolves a key or an identifier.
func TestGetActorAcceptsHandleOrID(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)

	cases := []struct {
		name string
		ref  string
	}{
		{"by id", admin.ID},
		{"by handle", "alice"},
		{"by handle, mixed case", "ALICE"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := l.GetActor(ctx, tc.ref)
			if err != nil {
				t.Fatalf("GetActor(%q): %v", tc.ref, err)
			}
			if got.ID != admin.ID {
				t.Errorf("GetActor(%q) = %q, want %q", tc.ref, got.ID, admin.ID)
			}
		})
	}

	if _, err := l.GetActor(ctx, "no-such-actor"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("GetActor for an unknown reference = %v, want not found", err)
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

// seedActors creates actors in a tenant, so a directory listing has both
// kinds of actor in it: agents hold no user, which is exactly why a picker
// built from ListUsers would not show them.
func seedActors(t *testing.T, l *Local, tenantID string, actors ...core.Actor) {
	t.Helper()
	ctx := context.Background()
	scope := core.TenantScope{TenantID: tenantID}
	for i := range actors {
		a := actors[i]
		if err := l.store.Update(ctx, scope, func(tx store.Tx) error {
			return tx.CreateActor(ctx, &a)
		}); err != nil {
			t.Fatalf("creating actor %q: %v", a.Handle, err)
		}
	}
}

// handlesOf reduces a listing to the handles it named.
func handlesOf(actors []core.Actor) []string {
	out := make([]string, 0, len(actors))
	for _, a := range actors {
		out = append(out, a.Handle)
	}
	return out
}

// TestListActorsIsTenantScoped is the guard on the isolation rule: a
// directory is a listing of people and agents, and one tenant's must never
// name another's.
func TestListActorsIsTenantScoped(t *testing.T) {
	l, _, _, admin := newLocal(t)
	seedActors(t, l, admin.TenantID,
		core.Actor{Kind: core.ActorUser, Handle: "ada"},
		core.Actor{Kind: core.ActorAgent, Handle: "mint"})

	intruder := otherTenantActor(t, l)
	seedActors(t, l, intruder.TenantID,
		core.Actor{Kind: core.ActorUser, Handle: "eve"},
		core.Actor{Kind: core.ActorAgent, Handle: "spy"})

	cases := []struct {
		name    string
		caller  *core.Actor
		want    []string
		refused []string
	}{
		{"this tenant", admin, []string{"ada", "alice", "mint"}, []string{"eve", "spy"}},
		{"the other tenant", intruder, []string{"bob", "eve", "spy"}, []string{"ada", "alice", "mint"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			actors, _, err := l.ListActors(authContext(tc.caller), core.Page{})
			if err != nil {
				t.Fatalf("ListActors: %v", err)
			}
			got := handlesOf(actors)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("ListActors = %v, want %v", got, tc.want)
			}
			for _, a := range actors {
				if a.TenantID != tc.caller.TenantID {
					t.Errorf("ListActors disclosed an actor of tenant %q", a.TenantID)
				}
			}
			for _, handle := range tc.refused {
				if slices.Contains(got, handle) {
					t.Errorf("ListActors disclosed another tenant's actor %q", handle)
				}
			}
		})
	}
}

func TestListActorsPaginatesByHandle(t *testing.T) {
	l, _, _, admin := newLocal(t)
	ctx := authContext(admin)
	seedActors(t, l, admin.TenantID,
		core.Actor{Kind: core.ActorAgent, Handle: "mint"},
		core.Actor{Kind: core.ActorUser, Handle: "ada"})

	first, next, err := l.ListActors(ctx, core.Page{Limit: 2})
	if err != nil {
		t.Fatalf("ListActors: %v", err)
	}
	if got := handlesOf(first); !slices.Equal(got, []string{"ada", "alice"}) {
		t.Fatalf("first page = %v, want [ada alice]", got)
	}
	if next == "" {
		t.Fatal("first page returned no cursor")
	}
	rest, last, err := l.ListActors(ctx, core.Page{Limit: 2, Cursor: next})
	if err != nil {
		t.Fatalf("ListActors page two: %v", err)
	}
	if got := handlesOf(rest); !slices.Equal(got, []string{"mint"}) {
		t.Fatalf("second page = %v, want [mint]", got)
	}
	if last != "" {
		t.Errorf("a short page returned cursor %q", last)
	}
}

// A member is not an administrator: the directory answers who is here, so
// every signed-in caller reads it and an unauthenticated one does not.
func TestListActorsRejectsWhatItCannotServe(t *testing.T) {
	l, _, _, admin := newLocal(t)
	member := &core.Actor{ID: "m1", TenantID: admin.TenantID, Kind: core.ActorUser, Role: core.RoleMember}

	cases := []struct {
		name string
		ctx  context.Context
		page core.Page
		kind core.Kind
	}{
		{"unauthenticated", context.Background(), core.Page{}, core.KindUnauthenticated},
		{"negative limit", authContext(admin), core.Page{Limit: -1}, core.KindInvalid},
		{"unsupported sort", authContext(admin), core.Page{Sort: core.SortCreatedAt}, core.KindInvalid},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, _, err := l.ListActors(tc.ctx, tc.page); !core.IsKind(err, tc.kind) {
				t.Errorf("ListActors = %v, want %v", err, tc.kind)
			}
		})
	}

	if _, _, err := l.ListActors(authContext(member), core.Page{}); err != nil {
		t.Errorf("ListActors as a member: %v", err)
	}
}
