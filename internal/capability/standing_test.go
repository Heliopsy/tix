// SPDX-License-Identifier: AGPL-3.0-or-later

package capability_test

import (
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// TestGlobalAdminOperationsArePermittedOnTheReport holds Permits to the
// service's report rather than to a scope. A session never holds global:admin,
// so asking HasScope offered nothing to a global admin, and "*" must not carry
// it either.
func TestGlobalAdminOperationsArePermittedOnTheReport(t *testing.T) {
	t.Parallel()
	op, ok := capability.ByMethod("ListGlobalAdmins")
	if !ok || op.Scope != core.ScopeGlobalAdmin {
		t.Fatalf("ListGlobalAdmins is not declared as needing global admin: %+v", op)
	}
	cases := []struct {
		name  string
		actor *core.Actor
		want  bool
	}{
		{"reported global admin with no scopes", &core.Actor{ID: "g", GlobalAdmin: true}, true},
		{"tenant admin", &core.Actor{ID: "a", Role: core.RoleAdmin}, false},
		{"token holding every scope", &core.Actor{ID: "k", Scopes: []core.Scope{core.ScopeAll}}, false},
		{"token naming global:admin, not reported", &core.Actor{ID: "k", TokenID: "tok",
			Scopes: []core.Scope{core.ScopeGlobalAdmin}}, false},
	}
	for _, tc := range cases {
		if got := op.Permits(tc.actor); got != tc.want {
			t.Errorf("%s: Permits = %v, want %v", tc.name, got, tc.want)
		}
	}
	if tenant, _ := capability.ByMethod("ListTenants"); !tenant.Permits(&core.Actor{Role: core.RoleAdmin}) {
		t.Error("an operation needing an ordinary scope stopped being permitted by role")
	}
}

// TestServerAndGlobalAdministrationReachTheTerminal names the fourteen
// operations the server and global admin views bind, so losing one fails here
// rather than only moving a count.
func TestServerAndGlobalAdministrationReachTheTerminal(t *testing.T) {
	t.Parallel()
	want := map[string]string{
		"Status": "server", "ListConnections": "server", "EndConnection": "server",
		"CreateTenant": "globaladmin", "DeleteTenant": "globaladmin",
		"ListTenantUsers": "globaladmin", "CreateTenantUser": "globaladmin",
		"UpdateTenantUser": "globaladmin", "DeleteTenantUser": "globaladmin",
		"AddTenantMember": "globaladmin", "ListGlobalAdmins": "globaladmin",
		"GrantGlobalAdmin": "globaladmin", "RevokeGlobalAdmin": "globaladmin",
		"GetAccountLogFor": "globaladmin",
	}
	for method, view := range want {
		op, ok := capability.ByMethod(method)
		if !ok {
			t.Errorf("%s is not in the registry", method)
			continue
		}
		if op.TUI != view {
			t.Errorf("%s is bound to view %q, want %q", method, op.TUI, view)
		}
		if e, exempted := op.ExemptionFor(capability.SurfaceTUI); exempted {
			t.Errorf("%s still records a terminal exemption: %s", method, e.Reason)
		}
	}
	del, _ := capability.ByMethod("DeleteTenant")
	if _, limited := del.LimitationFor(capability.SurfaceTUI); !limited {
		t.Error("DeleteTenant does not record that the session's own tenant is not deleted from the terminal")
	}
}
