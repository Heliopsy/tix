package core_test

import (
	"go/build"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

// TestCoreImportsOnlyStdlib enforces the invariant the whole architecture rests
// on. Both the local service and the remote HTTP client depend on core; if core
// could depend on either, or on anything with its own opinions, the two
// transports could drift apart. Keeping core at stdlib-only makes that a
// compile-time property rather than a code-review habit.
func TestCoreImportsOnlyStdlib(t *testing.T) {
	pkg, err := build.Default.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	for _, imp := range pkg.Imports {
		// Standard library import paths have no dot in their first segment.
		first, _, _ := strings.Cut(imp, "/")
		if strings.Contains(first, ".") {
			t.Errorf("internal/core imports %q; it must depend on the standard library only", imp)
		}
	}
}

// TestServiceInterfaceIsComplete guards against a sub-interface being dropped
// from Service during a refactor. Each embedded interface is a distinct area of
// the product, and a caller holding a Service must reach all of them.
func TestServiceInterfaceIsComplete(t *testing.T) {
	var svc core.Service

	// These assignments fail to compile if Service stops embedding an area.
	var (
		_ core.TenantService   = svc
		_ core.ProjectService  = svc
		_ core.WorkflowService = svc
		_ core.TaskService     = svc
		_ core.ClaimService    = svc
		_ core.HistoryService  = svc
		_ core.AuthService     = svc
		_ core.WebhookService  = svc
		_ core.TransferService = svc
		_ core.SyncService     = svc
	)
}

// TestScopeVocabularyIsUnique catches a copy-paste error in the scope constants,
// which would silently grant one permission when another was intended.
func TestScopeVocabularyIsUnique(t *testing.T) {
	seen := make(map[core.Scope]bool, len(core.AllScopes))
	for _, s := range core.AllScopes {
		if s == "" {
			t.Error("AllScopes contains an empty scope")
		}
		if seen[s] {
			t.Errorf("scope %q appears twice in AllScopes", s)
		}
		seen[s] = true
	}
	if seen[core.ScopeAll] {
		t.Error("AllScopes must not contain the wildcard scope")
	}
}

// TestRoleScopesAreKnown ensures a role never grants a scope outside the
// vocabulary, which would be a permission nothing checks for.
func TestRoleScopesAreKnown(t *testing.T) {
	known := make(map[core.Scope]bool, len(core.AllScopes))
	for _, s := range core.AllScopes {
		known[s] = true
	}
	known[core.ScopeAll] = true

	for _, r := range []core.Role{core.RoleViewer, core.RoleMember, core.RoleAdmin} {
		for _, s := range r.Scopes() {
			if !known[s] {
				t.Errorf("role %q grants unknown scope %q", r, s)
			}
		}
	}
}

// TestRolesAreOrderedByPrivilege asserts the roles nest: anything a viewer can
// do, a member can do. A gap here would mean promoting someone removed a
// permission.
func TestRolesAreOrderedByPrivilege(t *testing.T) {
	viewer := &core.Actor{Role: core.RoleViewer}
	member := &core.Actor{Role: core.RoleMember}
	admin := &core.Actor{Role: core.RoleAdmin}

	for _, s := range core.AllScopes {
		if viewer.HasScope(s) && !member.HasScope(s) {
			t.Errorf("member lacks %q which viewer holds", s)
		}
		if member.HasScope(s) && !admin.HasScope(s) {
			t.Errorf("admin lacks %q which member holds", s)
		}
	}
}
