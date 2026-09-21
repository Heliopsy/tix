package authz

import (
	"errors"
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

const (
	tenantA = "tenant-a"
	tenantB = "tenant-b"
	projA   = "proj-a"
	projB   = "proj-b"
)

func resource() Resource { return Resource{TenantID: tenantA, ProjectID: projA} }

func roleActor(r core.Role) *core.Actor {
	return &core.Actor{ID: "u1", TenantID: tenantA, Kind: core.ActorUser, Handle: "u1", Role: r}
}

func scopeActor(scopes ...core.Scope) *core.Actor {
	return &core.Actor{ID: "t1", TenantID: tenantA, Kind: core.ActorAgent, Handle: "agent", Scopes: scopes}
}

func TestEveryActionMapsToAScope(t *testing.T) {
	for _, a := range Actions() {
		scope, ok := a.Scope()
		if !ok {
			t.Errorf("action %q has no scope", a)
			continue
		}
		if !slices.Contains(core.AllScopes, scope) {
			t.Errorf("action %q maps to scope %q outside the vocabulary", a, scope)
		}
	}
}

func TestEveryScopeIsReachableFromSomeAction(t *testing.T) {
	for _, s := range core.AllScopes {
		used := false
		for _, a := range Actions() {
			if got, _ := a.Scope(); got == s {
				used = true
				break
			}
		}
		if !used {
			t.Errorf("scope %q is not required by any action", s)
		}
	}
}

func TestActionsAreUniqueAndNamed(t *testing.T) {
	seen := map[Action]bool{}
	for _, a := range Actions() {
		if a.String() != string(a) || a == "" {
			t.Fatalf("bad action name %q", a)
		}
		if seen[a] {
			t.Errorf("duplicate action %q", a)
		}
		seen[a] = true
	}
	if len(seen) != len(actionScopes) {
		t.Errorf("Actions() returned %d actions, map holds %d", len(seen), len(actionScopes))
	}
}

func TestRoleActionMatrix(t *testing.T) {
	viewer := []Action{
		ActionTaskRead, ActionProjectRead, ActionWorkflowRead,
		ActionEventSubscribe, ActionAuditRead,
	}
	member := append([]Action{
		ActionTaskCreate, ActionTaskUpdate, ActionTaskTransition, ActionTaskClaim,
		ActionCommentWrite, ActionArtifactWrite, ActionExport,
	}, viewer...)

	allowedByRole := map[core.Role][]Action{
		core.RoleViewer: viewer,
		core.RoleMember: member,
		core.RoleAdmin:  Actions(),
		core.Role(""):   nil,
	}

	p := New()
	for role, allowed := range allowedByRole {
		for _, action := range Actions() {
			want := slices.Contains(allowed, action)
			err := p.Can(roleActor(role), action, resource())
			got := err == nil
			if got != want {
				t.Errorf("role %q action %q: allowed=%v want=%v (err=%v)", role, action, got, want, err)
			}
			if !want && !core.IsKind(err, core.KindForbidden) {
				t.Errorf("role %q action %q: want forbidden, got %v", role, action, err)
			}
		}
	}
}

func TestRolesNest(t *testing.T) {
	p := New()
	for _, action := range Actions() {
		viewerOK := p.Allowed(roleActor(core.RoleViewer), action, resource())
		memberOK := p.Allowed(roleActor(core.RoleMember), action, resource())
		adminOK := p.Allowed(roleActor(core.RoleAdmin), action, resource())
		if viewerOK && !memberOK {
			t.Errorf("action %q allowed for viewer but not member", action)
		}
		if memberOK && !adminOK {
			t.Errorf("action %q allowed for member but not admin", action)
		}
	}
}

func TestScopeActionMatrix(t *testing.T) {
	p := New()
	for _, held := range core.AllScopes {
		actor := scopeActor(held)
		for _, action := range Actions() {
			required, _ := action.Scope()
			want := required == held
			err := p.Can(actor, action, resource())
			if (err == nil) != want {
				t.Errorf("scope %q action %q: allowed=%v want=%v (err=%v)", held, action, err == nil, want, err)
			}
		}
	}
}

func TestEmptyScopeSetDeniesEverything(t *testing.T) {
	p := New()
	actor := scopeActor()
	for _, action := range Actions() {
		err := p.Can(actor, action, resource())
		if !core.IsKind(err, core.KindForbidden) {
			t.Errorf("action %q: want forbidden, got %v", action, err)
		}
	}
}

func TestNoAuthWildcardActorAllowedEverywhere(t *testing.T) {
	p := New()
	actor := core.SystemActor(tenantA)
	for _, action := range Actions() {
		if err := p.Can(actor, action, resource()); err != nil {
			t.Errorf("wildcard actor denied %q: %v", action, err)
		}
	}
}

func TestNilActorIsUnauthenticated(t *testing.T) {
	p := New()
	err := p.Can(nil, ActionTaskRead, resource())
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("want unauthenticated, got %v", err)
	}
	if p.Allowed(nil, ActionTaskRead, resource()) {
		t.Fatal("nil actor reported as allowed")
	}
}

func TestCrossTenantIsNotFoundNotForbidden(t *testing.T) {
	p := New()
	res := Resource{TenantID: tenantB, ProjectID: projA}
	for _, actor := range []*core.Actor{
		roleActor(core.RoleAdmin),
		core.SystemActor(tenantA),
		scopeActor(core.ScopeTaskRead),
	} {
		err := p.Can(actor, ActionTaskRead, res)
		if !core.IsKind(err, core.KindNotFound) {
			t.Errorf("actor %q cross-tenant: want not_found, got %v", actor.ID, err)
		}
		if core.IsKind(err, core.KindForbidden) {
			t.Errorf("actor %q cross-tenant leaked a forbidden error", actor.ID)
		}
	}
}

func TestCrossTenantCheckPrecedesScopeCheck(t *testing.T) {
	p := New()
	err := p.Can(scopeActor(), ActionTaskDelete, Resource{TenantID: tenantB})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("want not_found ahead of forbidden, got %v", err)
	}
}

func TestResourceWithoutTenantUsesActorTenant(t *testing.T) {
	p := New()
	if err := p.Can(roleActor(core.RoleAdmin), ActionTenantAdmin, Resource{}); err != nil {
		t.Fatalf("tenant-level action denied: %v", err)
	}
}

func TestProjectScopedTokenOutsideItsProject(t *testing.T) {
	p := New()
	actor := scopeActor(core.ScopeTaskRead)
	actor.ProjectID = projA

	if err := p.Can(actor, ActionTaskRead, Resource{TenantID: tenantA, ProjectID: projA}); err != nil {
		t.Fatalf("token denied inside its own project: %v", err)
	}
	err := p.Can(actor, ActionTaskRead, Resource{TenantID: tenantA, ProjectID: projB})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("want forbidden outside project, got %v", err)
	}
	if err := p.Can(actor, ActionTaskRead, Resource{TenantID: tenantA}); err != nil {
		t.Fatalf("project-scoped token denied a project-less resource: %v", err)
	}
}

func TestProjectScopedWildcardTokenStillBoundToProject(t *testing.T) {
	p := New()
	actor := scopeActor(core.ScopeAll)
	actor.ProjectID = projA
	err := p.Can(actor, ActionTaskRead, Resource{TenantID: tenantA, ProjectID: projB})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("want forbidden, got %v", err)
	}
}

func TestAgentTokenLeastPrivilege(t *testing.T) {
	agent := scopeActor(
		core.ScopeTaskRead, core.ScopeTaskClaim, core.ScopeTaskTransition, core.ScopeTaskWrite,
		core.ScopeCommentWrite, core.ScopeArtifactWrite, core.ScopeEventSubscribe,
	)
	p := New()

	denied := []Action{
		ActionProjectWrite, ActionTaskDelete, ActionUserAdmin, ActionTokenAdmin,
		ActionAuditRead, ActionTenantAdmin, ActionRetentionWrite, ActionImport,
	}
	for _, action := range denied {
		err := p.Can(agent, action, resource())
		if !core.IsKind(err, core.KindForbidden) {
			t.Errorf("agent token allowed %q (err=%v)", action, err)
		}
	}

	granted := []Action{
		ActionTaskRead, ActionTaskCreate, ActionTaskUpdate, ActionTaskTransition,
		ActionTaskClaim, ActionCommentWrite, ActionArtifactWrite, ActionEventSubscribe,
	}
	for _, action := range granted {
		if err := p.Can(agent, action, resource()); err != nil {
			t.Errorf("agent token denied %q: %v", action, err)
		}
	}
}

func TestUnknownActionIsForbidden(t *testing.T) {
	p := New()
	err := p.Can(core.SystemActor(tenantA), Action("task.nope"), resource())
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("want forbidden for unknown action, got %v", err)
	}
	if _, ok := Action("task.nope").Scope(); ok {
		t.Fatal("unknown action resolved a scope")
	}
}

func TestDenialDetailsNameActionAndScope(t *testing.T) {
	p := New()
	err := p.Can(scopeActor(), ActionTaskDelete, Resource{TenantID: tenantA, ProjectID: projA, OwnerID: "u9"})
	var domain *core.Error
	if !errors.As(err, &domain) {
		t.Fatalf("want *core.Error, got %T", err)
	}
	if domain.Details["action"] != string(ActionTaskDelete) {
		t.Errorf("missing action detail: %v", domain.Details)
	}
	if domain.Details["scope"] != string(core.ScopeTaskDelete) {
		t.Errorf("missing scope detail: %v", domain.Details)
	}
	if domain.Details["owner_id"] != "u9" {
		t.Errorf("missing owner detail: %v", domain.Details)
	}
}

func TestAllowedMirrorsCan(t *testing.T) {
	p := New()
	cases := []*core.Actor{
		roleActor(core.RoleViewer), roleActor(core.RoleMember), roleActor(core.RoleAdmin),
		scopeActor(core.ScopeTaskRead), core.SystemActor(tenantA),
	}
	for _, actor := range cases {
		for _, action := range Actions() {
			for _, res := range []Resource{resource(), {TenantID: tenantB}, {}} {
				want := p.Can(actor, action, res) == nil
				if got := p.Allowed(actor, action, res); got != want {
					t.Errorf("actor %q action %q res %+v: Allowed=%v Can=%v", actor.ID, action, res, got, want)
				}
			}
		}
	}
}
