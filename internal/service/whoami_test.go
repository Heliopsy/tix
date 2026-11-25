// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestWhoAmINamesTheTenantAndThemeForEveryCredential pins the tenant's key,
// name and resolved theme on every whoami answer, whatever credential asks,
// while memberships stay with a session or account:read.
func TestWhoAmINamesTheTenantAndThemeForEveryCredential(t *testing.T) {
	l, _, scope, admin := newLocal(t)
	ocean := "ocean"
	if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{Theme: &ocean}); err != nil {
		t.Fatal(err)
	}
	agent := core.Actor{Kind: core.ActorAgent, Handle: "bot"}
	if err := l.store.Update(context.Background(), scope, func(tx store.Tx) error {
		return tx.CreateActor(context.Background(), &agent)
	}); err != nil {
		t.Fatal(err)
	}
	want, _ := l.themeRegistry().Lookup("ocean")
	tests := []struct {
		name  string
		actor core.Actor
	}{
		{"an agent's token", core.Actor{ID: agent.ID, Kind: core.ActorAgent, TokenID: "t1",
			Scopes: []core.Scope{core.ScopeTaskRead}}},
		{"a person's token", core.Actor{ID: admin.ID, Kind: core.ActorAgent, TokenID: "t2",
			Scopes: []core.Scope{core.ScopeTaskRead}}},
		{"an agent without a token", core.Actor{ID: agent.ID, Kind: core.ActorAgent}},
		{"the system", core.Actor{ID: "system", Kind: core.ActorSystem}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tc.actor.TenantID = scope.TenantID
			me, err := l.WhoAmI(authContext(&tc.actor))
			if err != nil {
				t.Fatal(err)
			}
			if me.TenantKey != "acme" || me.TenantName != "Acme" {
				t.Errorf("tenant = %q %q, want acme Acme", me.TenantKey, me.TenantName)
			}
			if me.Theme == nil || *me.Theme != want {
				t.Errorf("theme = %+v, want %+v", me.Theme, want)
			}
			if len(me.Memberships) != 0 {
				t.Errorf("memberships = %+v, want none", me.Memberships)
			}
		})
	}
}

// TestWhoAmIThemeIsWhatTheBrowserRenders pins the dark pair being filled and
// a custom accent being derived, so a client never repeats either rule.
func TestWhoAmIThemeIsWhatTheBrowserRenders(t *testing.T) {
	configured, err := core.NewThemeRegistry([]core.Theme{{Name: "brand", Accent: "#7c3aed", AccentSoft: "#f3eeff"}})
	if err != nil {
		t.Fatal(err)
	}
	custom, _ := core.CustomTheme("#7a3ef0")
	tests := []struct {
		name, theme string
		want        core.Theme
	}{
		{"a configured theme with no dark pair", "brand", core.Theme{Name: "brand", Accent: "#7c3aed",
			AccentSoft: "#f3eeff", AccentDark: "#7c3aed", AccentSoftDark: "#f3eeff"}},
		{"a custom accent", "#7a3ef0", custom},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			l, _, scope, admin := newLocalWith(t, WithThemes(configured))
			if _, err := l.UpdateTenant(authContext(admin), "", core.UpdateTenantInput{Theme: &tc.theme}); err != nil {
				t.Fatal(err)
			}
			token := &core.Actor{ID: admin.ID, TenantID: scope.TenantID, Kind: core.ActorAgent, TokenID: "t",
				Scopes: []core.Scope{core.ScopeTaskRead}}
			me, err := l.WhoAmI(authContext(token))
			if err != nil {
				t.Fatal(err)
			}
			if me.Theme == nil || *me.Theme != tc.want {
				t.Errorf("theme = %+v, want %+v", me.Theme, tc.want)
			}
		})
	}
}
