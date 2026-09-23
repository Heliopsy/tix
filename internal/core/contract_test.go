// SPDX-License-Identifier: AGPL-3.0-or-later

package core_test

import (
	"go/ast"
	"go/build"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestCoreImportsOnlyStdlib enforces the invariant the whole architecture rests
func TestCoreImportsOnlyStdlib(t *testing.T) {
	pkg, err := build.Default.ImportDir(".", 0)
	if err != nil {
		t.Fatalf("reading package: %v", err)
	}

	for _, imp := range pkg.Imports {
		first, _, _ := strings.Cut(imp, "/")
		if strings.Contains(first, ".") {
			t.Errorf("internal/core imports %q; it must depend on the standard library only", imp)
		}
	}
}

// service is the typed nil the assertions below are written against.
var service core.Service

// Dropping a sub-interface out of Service stops this file compiling, which is
// a build failure rather than a test failure and needs no test to report it.
var (
	_ core.TenantService     = service
	_ core.ProjectService    = service
	_ core.WorkflowService   = service
	_ core.TaskService       = service
	_ core.ClaimService      = service
	_ core.HistoryService    = service
	_ core.AuthService       = service
	_ core.WebhookService    = service
	_ core.TransferService   = service
	_ core.SyncService       = service
	_ core.BundleService     = service
	_ core.ConnectionService = service
)

// serviceParts names every sub-interface Service is assembled from.
var serviceParts = map[string]reflect.Type{
	"TenantService":     reflect.TypeOf((*core.TenantService)(nil)).Elem(),
	"ProjectService":    reflect.TypeOf((*core.ProjectService)(nil)).Elem(),
	"WorkflowService":   reflect.TypeOf((*core.WorkflowService)(nil)).Elem(),
	"TaskService":       reflect.TypeOf((*core.TaskService)(nil)).Elem(),
	"ClaimService":      reflect.TypeOf((*core.ClaimService)(nil)).Elem(),
	"HistoryService":    reflect.TypeOf((*core.HistoryService)(nil)).Elem(),
	"AuthService":       reflect.TypeOf((*core.AuthService)(nil)).Elem(),
	"WebhookService":    reflect.TypeOf((*core.WebhookService)(nil)).Elem(),
	"TransferService":   reflect.TypeOf((*core.TransferService)(nil)).Elem(),
	"SyncService":       reflect.TypeOf((*core.SyncService)(nil)).Elem(),
	"BundleService":     reflect.TypeOf((*core.BundleService)(nil)).Elem(),
	"ConnectionService": reflect.TypeOf((*core.ConnectionService)(nil)).Elem(),
}

// serviceOwnMethods are the only two methods Service may declare itself.
// Everything else belongs to one of the sub-interfaces above, which is what
// lets a caller depend on the narrow surface it needs.
var serviceOwnMethods = []string{"WhoAmI", "Close"}

// TestServiceIsExactlyItsParts checks at run time what the assertions above
// cannot: that Service grew no method outside the sub-interfaces it is
// assembled from, and that no sub-interface listed here has fallen out of it.
func TestServiceIsExactlyItsParts(t *testing.T) {
	svc := reflect.TypeOf((*core.Service)(nil)).Elem()

	owner := make(map[string]string, svc.NumMethod())
	for _, name := range serviceOwnMethods {
		owner[name] = "Service itself"
	}
	for part, typ := range serviceParts {
		for i := 0; i < typ.NumMethod(); i++ {
			name := typ.Method(i).Name
			if previous, ok := owner[name]; ok {
				t.Errorf("method %q is declared by both %s and %s", name, previous, part)
				continue
			}
			owner[name] = part
		}
	}

	on := make(map[string]bool, svc.NumMethod())
	for i := 0; i < svc.NumMethod(); i++ {
		name := svc.Method(i).Name
		on[name] = true
		if _, ok := owner[name]; !ok {
			t.Errorf("Service declares %q outside every sub-interface; give it one", name)
		}
	}
	for name, part := range owner {
		if !on[name] {
			t.Errorf("%s declares %q but Service does not carry it", part, name)
		}
	}
}

// TestScopeVocabularyIsUnique catches a copy-paste error in the scope constants,
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

// vocabularies pairs a constant type in the package source with the exported
// list that is supposed to carry every one of its constants. The list is the
// single owner every other layer derives its choices from, so a constant
// missing from it is a value the domain accepts and no screen, flag or
// subscriber may name; a value in the list that no constant declares is the
// opposite mistake. Nothing here restates a vocabulary: the expected set is
// read out of the source.
var vocabularies = []struct {
	// typeName is the declared type of the constants to scan for.
	typeName string
	// listName names the exported list in a failure message.
	listName string
	// list is the exported list, rendered as strings.
	list []string
	// except names constants that deliberately stay out of the list.
	except []string
}{
	{"Kind", "core.Kinds", asStrings(core.Kinds), nil},
	{"Role", "core.Roles", asStrings(core.Roles), nil},
	{"CertMode", "core.CertModes", asStrings(core.CertModes), nil},
	{"FieldType", "core.FieldTypes", asStrings(core.FieldTypes), nil},
	{"ArtifactKind", "core.ArtifactKinds", asStrings(core.ArtifactKinds), nil},
	{"ComponentKind", "core.ComponentKinds", asStrings(core.ComponentKinds), nil},
	{"CollisionPolicy", "core.CollisionPolicies", asStrings(core.CollisionPolicies), nil},
	{"ImportMode", "core.ImportModes", asStrings(core.ImportModes), nil},
	{"EventType", "core.EventTypes()", asStrings(core.EventTypes()), nil},
	// ScopeAll is the wildcard, not a scope anyone may be granted by name.
	{"Scope", "core.AllScopes", asStrings(core.AllScopes), []string{"ScopeAll"}},
}

// asStrings renders a vocabulary of named string constants as plain strings.
func asStrings[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, v := range values {
		out = append(out, string(v))
	}
	return out
}

// TestVocabulariesCoverEveryDeclaredConstant reads the constants out of the
// package source rather than restating them, so the assertion cannot drift the
// way a hand-written copy of the list does.
func TestVocabulariesCoverEveryDeclaredConstant(t *testing.T) {
	for _, v := range vocabularies {
		t.Run(v.typeName, func(t *testing.T) {
			declared := declaredConstants(t, v.typeName)
			for _, name := range v.except {
				if _, ok := declared[name]; !ok {
					t.Errorf("%s is named as an exception but no %s constant declares it",
						name, v.typeName)
				}
				delete(declared, name)
			}
			if len(declared) == 0 {
				t.Fatalf("found no %s constants in the package source; the scan is broken",
					v.typeName)
			}

			listed := make(map[string]bool, len(v.list))
			for _, value := range v.list {
				if listed[value] {
					t.Errorf("%s carries %q twice", v.listName, value)
				}
				listed[value] = true
			}
			for name, value := range declared {
				if !listed[value] {
					t.Errorf("core.%s is declared as %q but %s omits it", name, value, v.listName)
				}
			}
			for value := range listed {
				if !slices.Contains(slices.Collect(maps.Values(declared)), value) {
					t.Errorf("%s carries %q, which no %s constant declares",
						v.listName, value, v.typeName)
				}
			}
		})
	}
}

// declaredConstants returns every constant in the package source whose declared
// type is typeName, keyed by identifier.
func declaredConstants(t *testing.T, typeName string) map[string]string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading package directory: %v", err)
	}

	found := make(map[string]string)
	fset := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.CONST {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				ident, ok := value.Type.(*ast.Ident)
				if !ok || ident.Name != typeName {
					continue
				}
				for i, n := range value.Names {
					lit, ok := value.Values[i].(*ast.BasicLit)
					if !ok || lit.Kind != token.STRING {
						t.Fatalf("constant %s is not a string literal; teach this scan about it", n.Name)
					}
					unquoted, err := strconv.Unquote(lit.Value)
					if err != nil {
						t.Fatalf("unquoting %s: %v", n.Name, err)
					}
					found[n.Name] = unquoted
				}
			}
		}
	}
	return found
}
