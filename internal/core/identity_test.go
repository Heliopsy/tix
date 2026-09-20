package core

import (
	"context"
	"testing"
)

func TestActorHasScopeExplicit(t *testing.T) {
	a := &Actor{Scopes: []Scope{ScopeTaskRead, ScopeTaskClaim}}

	if !a.HasScope(ScopeTaskClaim) {
		t.Error("explicitly granted scope should be held")
	}
	if a.HasScope(ScopeTaskDelete) {
		t.Error("ungranted scope must not be held")
	}
}

func TestActorHasScopeWildcard(t *testing.T) {
	a := &Actor{Scopes: []Scope{ScopeAll}}
	for _, s := range AllScopes {
		if !a.HasScope(s) {
			t.Errorf("wildcard actor should hold %q", s)
		}
	}
}

func TestActorHasScopeFromRole(t *testing.T) {
	// A human's scopes come from their role without being enumerated on the actor.
	member := &Actor{Role: RoleMember}
	if !member.HasScope(ScopeTaskClaim) {
		t.Error("member role should grant task:claim")
	}
	if member.HasScope(ScopeUserAdmin) {
		t.Error("member role must not grant user:admin")
	}

	admin := &Actor{Role: RoleAdmin}
	for _, s := range AllScopes {
		if !admin.HasScope(s) {
			t.Errorf("admin role should grant %q", s)
		}
	}
}

func TestViewerCannotWrite(t *testing.T) {
	v := &Actor{Role: RoleViewer}
	for _, s := range []Scope{
		ScopeTaskWrite, ScopeTaskDelete, ScopeTaskClaim, ScopeTaskTransition,
		ScopeProjectWrite, ScopeWorkflowWrite, ScopeCommentWrite, ScopeUserAdmin,
	} {
		if v.HasScope(s) {
			t.Errorf("viewer must not hold %q", s)
		}
	}
	for _, s := range []Scope{ScopeTaskRead, ScopeProjectRead, ScopeEventSubscribe} {
		if !v.HasScope(s) {
			t.Errorf("viewer should hold %q", s)
		}
	}
}

func TestAgentTokenCannotDeleteProject(t *testing.T) {
	// A typical queue-working agent token. The spec requires that such a token
	// can work a queue but cannot destroy the project it works in.
	agent := &Actor{
		Kind: ActorAgent,
		Scopes: []Scope{
			ScopeTaskRead, ScopeTaskClaim, ScopeTaskTransition,
			ScopeArtifactWrite, ScopeCommentWrite, ScopeEventSubscribe,
		},
	}
	for _, s := range []Scope{ScopeProjectWrite, ScopeTaskDelete, ScopeTenantAdmin, ScopeUserAdmin} {
		if agent.HasScope(s) {
			t.Errorf("queue-working agent must not hold %q", s)
		}
	}
	if !agent.HasScope(ScopeTaskClaim) {
		t.Error("queue-working agent must be able to claim")
	}
}

func TestNilActorHoldsNothing(t *testing.T) {
	var a *Actor
	if a.HasScope(ScopeTaskRead) {
		t.Error("nil actor must hold no scope")
	}
	if a.ScopedToProject() {
		t.Error("nil actor is not project scoped")
	}
}

func TestSystemActor(t *testing.T) {
	a := SystemActor("tenant-1")
	if a.Kind != ActorSystem || a.TenantID != "tenant-1" {
		t.Errorf("SystemActor() = %+v, want system kind and tenant-1", a)
	}
	if !a.HasScope(ScopeImport) {
		t.Error("system actor should hold every scope")
	}
}

func TestActorScopedToProject(t *testing.T) {
	if (&Actor{}).ScopedToProject() {
		t.Error("actor with no project is not project scoped")
	}
	if !(&Actor{ProjectID: "p1"}).ScopedToProject() {
		t.Error("actor with a project is project scoped")
	}
}

func TestRoleValid(t *testing.T) {
	for _, r := range []Role{RoleViewer, RoleMember, RoleAdmin} {
		if !r.Valid() {
			t.Errorf("%q should be valid", r)
		}
	}
	if Role("owner").Valid() {
		t.Error("unknown role must be invalid")
	}
	if Role("").Scopes() != nil {
		t.Error("unknown role grants no scopes")
	}
}

func TestActorKindValid(t *testing.T) {
	for _, k := range []ActorKind{ActorUser, ActorAgent, ActorSystem} {
		if !k.Valid() {
			t.Errorf("%q should be valid", k)
		}
	}
	if ActorKind("robot").Valid() {
		t.Error("unknown actor kind must be invalid")
	}
}

func TestWithActorAlsoSetsTenant(t *testing.T) {
	// An actor must never be evaluated against a tenant it did not authenticate
	// for, so carrying the actor implies carrying its tenant.
	ctx := WithActor(context.Background(), &Actor{ID: "a1", TenantID: "t1"})

	got, ok := TenantFrom(ctx)
	if !ok || got.TenantID != "t1" {
		t.Errorf("TenantFrom() = %+v, %v; want t1", got, ok)
	}
}

func TestWithActorNil(t *testing.T) {
	ctx := WithActor(context.Background(), nil)
	if _, ok := ActorFrom(ctx); ok {
		t.Error("nil actor must not be reported as present")
	}
	if _, err := RequireActor(ctx); !IsKind(err, KindUnauthenticated) {
		t.Errorf("RequireActor() error = %v, want unauthenticated", err)
	}
}

func TestRequireActor(t *testing.T) {
	want := &Actor{ID: "a1", TenantID: "t1"}
	got, err := RequireActor(WithActor(context.Background(), want))
	if err != nil {
		t.Fatalf("RequireActor() error = %v", err)
	}
	if got.ID != want.ID {
		t.Errorf("RequireActor() = %q, want %q", got.ID, want.ID)
	}

	if _, err := RequireActor(context.Background()); !IsKind(err, KindUnauthenticated) {
		t.Errorf("empty context error = %v, want unauthenticated", err)
	}
}

func TestRequireTenant(t *testing.T) {
	ctx := WithTenant(context.Background(), TenantScope{TenantID: "t1"})
	got, err := RequireTenant(ctx)
	if err != nil || got.TenantID != "t1" {
		t.Fatalf("RequireTenant() = %+v, %v", got, err)
	}

	if _, err := RequireTenant(context.Background()); !IsKind(err, KindInvalid) {
		t.Errorf("empty context error = %v, want invalid", err)
	}
}

func TestTenantScopeValid(t *testing.T) {
	tests := []struct {
		in   string
		want bool
	}{
		{"t1", true},
		{"", false},
		{"   ", false},
	}
	for _, tt := range tests {
		if got := (TenantScope{TenantID: tt.in}).Valid(); got != tt.want {
			t.Errorf("TenantScope{%q}.Valid() = %v, want %v", tt.in, got, tt.want)
		}
	}
	// A blank tenant must not satisfy RequireTenant either.
	ctx := WithTenant(context.Background(), TenantScope{TenantID: "  "})
	if _, err := RequireTenant(ctx); err == nil {
		t.Error("blank tenant must not satisfy RequireTenant")
	}
}

func TestSourceDefaultsToSystem(t *testing.T) {
	if got := SourceFrom(context.Background()); got != SourceSystem {
		t.Errorf("SourceFrom(empty) = %q, want %q", got, SourceSystem)
	}
	ctx := WithSource(context.Background(), SourceCLI)
	if got := SourceFrom(ctx); got != SourceCLI {
		t.Errorf("SourceFrom() = %q, want %q", got, SourceCLI)
	}
	// An empty source must not mask the default.
	if got := SourceFrom(WithSource(context.Background(), "")); got != SourceSystem {
		t.Errorf("SourceFrom(empty source) = %q, want %q", got, SourceSystem)
	}
}
