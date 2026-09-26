// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// everyView is every view the interface has, which is the list gating has to
// resolve a name for.
var everyView = []viewKind{
	viewProjects, viewBoard, viewDetail, viewHelp, viewSettings,
	viewActivity, viewTenant, viewStats,
}

// fullAccess offers every view, which is what an administrator's authority
// resolves to. Tests state it rather than inheriting it from a permissive
// default, so a test about gating cannot pass because gating was off.
func fullAccess() ViewAccess {
	out := make(ViewAccess, len(everyView))
	for _, v := range everyView {
		out[viewName(v)] = true
	}
	return out
}

// allViews is fullAccess as the predicate the help overlay filters with.
func allViews(viewKind) bool { return true }

// TestEveryGatedViewResolvesToARegistryName is the half of the gating
// requirement that running the interface cannot reach. canReach looks a view up
// by the name viewName gives it, so a view whose name does not appear in the
// capability registry is asked about a key that is never set and is refused
// forever, and a registry view the interface cannot name is a screen nothing
// gates. Both are silent, so both are asserted here rather than observed.
func TestEveryGatedViewResolvesToARegistryName(t *testing.T) {
	declared := capability.TUIViews()
	if len(declared) == 0 {
		t.Fatal("the registry binds no view at all, so this guard is watching nothing")
	}

	named := make([]string, 0, len(everyView))
	for _, v := range everyView {
		name := viewName(v)
		if slices.Contains(named, name) {
			t.Errorf("two views share the name %q, so gating one gates the other", name)
		}
		named = append(named, name)
	}
	for _, view := range declared {
		if !slices.Contains(named, view) {
			t.Errorf("the registry binds operations to view %q, which viewName never produces, "+
				"so nothing in the interface gates it", view)
		}
	}
	for _, v := range everyView {
		if localOnlyViews[v] {
			continue
		}
		if !slices.Contains(declared, viewName(v)) {
			t.Errorf("view %q is gated on a registry name the registry does not declare, "+
				"so it can never be offered", viewName(v))
		}
	}
}

// TestLocalOnlyViewsReadNothingFromTheService pins the carve-out. A view listed
// there is offered to every reader, so the list is the one place a screen can
// escape the policy and it has to stay the three that read nothing a scope
// covers.
func TestLocalOnlyViewsReadNothingFromTheService(t *testing.T) {
	want := []viewKind{viewProjects, viewSettings, viewHelp}
	for v := range localOnlyViews {
		if !slices.Contains(want, v) {
			t.Errorf("view %q is exempt from gating and is not one of the three that may be",
				viewName(v))
		}
	}
	for _, v := range want {
		if !localOnlyViews[v] {
			t.Errorf("view %q lost its exemption, which makes the root unreachable", viewName(v))
		}
	}
	m := Model{}
	for _, v := range want {
		if !m.canReach(v) {
			t.Errorf("a session with no access at all cannot reach %q", viewName(v))
		}
	}
}

// TestNoAccessOffersNothingBeyondTheLocalViews states the closed default: a
// caller that supplies no access set offers a reader nothing the service would
// have to permit. The other default would hand every reader every screen.
func TestNoAccessOffersNothingBeyondTheLocalViews(t *testing.T) {
	m := New(Config{})
	for _, v := range everyView {
		if localOnlyViews[v] {
			continue
		}
		if m.canReach(v) {
			t.Errorf("a session with no access set was offered %q", viewName(v))
		}
		if entered := m.enterView(v); entered.view == v {
			t.Errorf("a session with no access set entered %q", viewName(v))
		}
	}
}

// TestAccessFollowsTheActorsScopes drives the registry-derived set for the
// authorities that actually connect: a demo visitor, the three membership roles
// and a project-pinned API token. The visitor and the administrator share this
// code path, so a mistake here is a sandbox visitor reaching a screen that is
// not theirs.
func TestAccessFollowsTheActorsScopes(t *testing.T) {
	// visitorScopes as internal/sshd/sandbox.go grants them. Named here rather
	// than imported because internal/sshd imports this package.
	visitor := []core.Scope{
		core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskTransition,
		core.ScopeTaskClaim, core.ScopeTaskDelete,
		core.ScopeProjectRead, core.ScopeProjectWrite,
		core.ScopeWorkflowRead, core.ScopeWorkflowWrite,
		core.ScopeCommentWrite, core.ScopeArtifactWrite,
		core.ScopeEventSubscribe, core.ScopeAuditRead, core.ScopeExport,
	}
	cases := []struct {
		name   string
		actor  *core.Actor
		offers []viewKind
		denies []viewKind
	}{
		{
			name:   "sandbox visitor",
			actor:  &core.Actor{ID: "v", TenantID: "sandbox", Scopes: visitor},
			offers: []viewKind{viewBoard, viewDetail, viewActivity, viewStats, viewTenant},
		},
		{
			name:   "enrolled admin",
			actor:  &core.Actor{ID: "a", TenantID: "t", Role: core.RoleAdmin},
			offers: []viewKind{viewBoard, viewDetail, viewActivity, viewStats, viewTenant},
		},
		{
			name:   "enrolled viewer",
			actor:  &core.Actor{ID: "r", TenantID: "t", Role: core.RoleViewer},
			offers: []viewKind{viewBoard, viewDetail, viewActivity, viewStats, viewTenant},
		},
		{
			name: "token that may only read projects",
			actor: &core.Actor{ID: "k", TenantID: "t", Kind: core.ActorAgent,
				Scopes: []core.Scope{core.ScopeProjectRead}},
			// The detail view is offered because resolving an identifier to a
			// handle needs no scope; the board is not, so there is no way in.
			offers: []viewKind{viewDetail, viewTenant},
			denies: []viewKind{viewBoard, viewActivity, viewStats},
		},
		{
			name: "token that may not watch events",
			actor: &core.Actor{ID: "p", TenantID: "t", Kind: core.ActorAgent,
				Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}},
			offers: []viewKind{viewBoard, viewDetail, viewStats},
			denies: []viewKind{viewActivity},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := New(Config{Actor: tc.actor, Access: capability.TUIAccess(tc.actor)})
			for _, v := range tc.offers {
				if !m.canReach(v) {
					t.Errorf("%q is refused and should be offered", viewName(v))
				}
			}
			for _, v := range tc.denies {
				if m.canReach(v) {
					t.Errorf("%q is offered and should be refused", viewName(v))
				}
			}
		})
	}
}

// TestTheHelpOverlayNamesOnlyOfferedViews is the requirement that the overlay
// agrees with the interface. A reader shown a key for a view they are refused
// has been told the refusal is their mistake.
func TestTheHelpOverlayNamesOnlyOfferedViews(t *testing.T) {
	keys := DefaultKeyMap()
	desc := func(entries []HelpEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Desc)
		}
		return out
	}

	full := desc(keys.GlobalHelp(allViews))
	for _, want := range []string{"activity", "statistics", "tenant", "projects", "settings"} {
		if !slices.Contains(full, want) {
			t.Errorf("a reader offered every view is not told about %q", want)
		}
	}

	actor := &core.Actor{ID: "p", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	m := New(Config{Actor: actor, Access: capability.TUIAccess(actor)})
	narrowed := desc(keys.GlobalHelp(m.offersView()))
	if slices.Contains(narrowed, "activity") {
		t.Error("a reader who may not subscribe is still told about the activity view")
	}
	for _, want := range []string{"statistics", "projects", "help", "quit"} {
		if !slices.Contains(narrowed, want) {
			t.Errorf("a narrowed overlay dropped %q, which the reader may still reach", want)
		}
	}
	if len(keys.GlobalHelp(nil)) >= len(full) {
		t.Error("a nil predicate offered as much as full access")
	}
}

// mayBuildWithoutAccess are the files allowed to build a session without
// stating its authority: this one, which asserts what happens when nobody does.
var mayBuildWithoutAccess = []string{"internal/tui/access_test.go"}

// TestEverySessionIsBuiltWithItsAuthority is the half of the gating requirement
// that running the interface cannot reach. The default is closed, so a caller
// that forgets Access loses navigation rather than opening it, but losing
// navigation in a surface nobody drove in a test would still ship. Every
// literal that builds a session is therefore required to name the field, which
// a walk of the tree can see and reflection cannot.
func TestEverySessionIsBuiltWithItsAuthority(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatalf("resolving the repository root: %v", err)
	}
	openings := []string{"tui.Config{", "tui.Options{", "New(Config{", "Run(Options{"}

	found := 0
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); path != root && (strings.HasPrefix(name, ".") || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		src := string(body)
		for _, opening := range openings {
			for at := 0; ; {
				i := strings.Index(src[at:], opening)
				if i < 0 {
					break
				}
				start := at + i
				at = start + len(opening)
				// A literal with no service reaches nothing, so it cannot offer
				// a reader a screen either way.
				literal := balanced(src[start+len(opening)-1:])
				if !strings.Contains(literal, "Service:") {
					continue
				}
				found++
				if strings.Contains(literal, "Access:") || slices.Contains(mayBuildWithoutAccess, rel) {
					continue
				}
				t.Errorf("%s builds a session without naming Access, so the reader would be "+
					"offered nothing but the local views:\n%s", rel, literal)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}
	if found < 3 {
		t.Fatalf("found only %d session literals in the tree; the walk is broken", found)
	}
}

// balanced returns the brace-balanced literal starting at src[0], which must be
// an opening brace.
func balanced(src string) string {
	depth := 0
	for i, r := range src {
		switch r {
		case '{':
			depth++
		case '}':
			if depth--; depth == 0 {
				return src[:i+1]
			}
		}
	}
	return src
}
