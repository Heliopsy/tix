// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestTheAvatarUsesLettersNotPunctuation covers the seeded agents, whose names
// carry a parenthetical: taking the first rune of the second word drew "Atlas
// (build agent)" as "A(".
func TestTheAvatarUsesLettersNotPunctuation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ label, want string }{
		{"Atlas (build agent)", "AB"},
		{"Nadia Petrova", "NP"},
		{"mint", "M"},
		{"ada@example.test", "A"},
		{"(parenthetical)", "P"},
	} {
		got := monogramOf(tc.label)
		if got != tc.want {
			t.Errorf("monogram of %q = %q, want %q", tc.label, got, tc.want)
		}
	}
}

// TestTheAdvancedNavigationFollowsTheScopeNotTheURL covers a member who is
// refused at an administration screen: they used to be served the whole
// administration navigation on the refusal page, every entry of which would
// refuse them too. The function's own doc comment already said an entry that
// leads to a refusal is worse than no entry; the path check simply ran first.
func TestTheAdvancedNavigationFollowsTheScopeNotTheURL(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		scopes []core.Scope
		want   bool
	}{
		{"a member refused at an admin screen", []core.Scope{core.ScopeTaskRead}, false},
		{"a tenant administrator", []core.Scope{core.ScopeTenantAdmin}, true},
		{"a user administrator, who can use one of them", []core.Scope{core.ScopeUserAdmin}, true},
		{"nobody at all", nil, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/admin/users", nil)
			if tc.scopes != nil {
				actor := &core.Actor{ID: "a", Scopes: tc.scopes}
				r = r.WithContext(core.WithActor(r.Context(), actor))
			}
			if got := advancedMode(r); got != tc.want {
				t.Errorf("advancedMode on /admin/users = %v, want %v", got, tc.want)
			}
		})
	}

	// Away from those paths the preference still decides, in either direction.
	r := httptest.NewRequest(http.MethodGet, "/tasks", nil)
	r = r.WithContext(core.WithActor(r.Context(), &core.Actor{ID: "a", Scopes: []core.Scope{core.ScopeTenantAdmin}}))
	if !advancedMode(r) {
		t.Error("a tenant administrator lost the advanced navigation away from an admin path")
	}
}
