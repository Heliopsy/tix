// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTenantScreenListsSignInProviders(t *testing.T) {
	state := TenantAdminState{Loaded: true, Tenant: &core.Tenant{Key: "acme", PasswordLogin: core.PasswordLoginDisabled},
		Providers: []core.SSOProvider{
			{Key: "kc", Kind: core.SSOKindOIDC, DisplayName: "Keycloak", Enabled: true, AutoCreate: true},
			{Key: "gh", Kind: core.SSOKindGitHub, DisplayName: "GitHub"},
		}}
	var text []string
	for _, l := range TenantAdminView(state) {
		text = append(text, l.Text)
	}
	page := strings.Join(text, "\n")
	for _, want := range []string{"sign-in providers (2)", "kc", "oidc Keycloak, creates accounts", "gh", "GitHub (off)",
		"single sign-on only"} {
		if !strings.Contains(page, want) {
			t.Errorf("the tenant screen does not say %q:\n%s", want, page)
		}
	}
}

func TestTenantScreenLoadsProviders(t *testing.T) {
	svc := &fakeService{ssoProviders: []core.SSOProvider{{Key: "kc", Kind: core.SSOKindOIDC, DisplayName: "Keycloak"}}}
	m := Model{svc: svc}
	msg, ok := m.loadTenantInfo()().(tenantInfoMsg)
	if !ok || len(msg.providers) != 1 || msg.providers[0].Key != "kc" {
		t.Fatalf("tenant info = %+v, want the provider read", msg)
	}
}
