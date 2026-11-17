// SPDX-License-Identifier: AGPL-3.0-or-later

package core

import (
	"strings"
	"testing"
)

func validOIDC() SSOProvider {
	return SSOProvider{Key: "kc", Kind: SSOKindOIDC, DisplayName: "Keycloak", ClientID: "tix",
		Issuer: "https://id.example.com/realms/acme", DefaultRole: RoleMember}
}

func validGitHub() SSOProvider {
	return SSOProvider{Key: "gh", Kind: SSOKindGitHub, DisplayName: "GitHub", ClientID: "Iv1", DefaultRole: RoleMember}
}

func TestSSOProviderValidateNamesTheField(t *testing.T) {
	cases := []struct {
		name  string
		edit  func(*SSOProvider)
		base  func() SSOProvider
		field string
	}{
		{"bad key", func(p *SSOProvider) { p.Key = "Bad Key" }, validOIDC, "key"},
		{"saml", func(p *SSOProvider) { p.Kind = SSOKindSAML }, validOIDC, "kind"},
		{"unknown kind", func(p *SSOProvider) { p.Kind = "ldap" }, validOIDC, "kind"},
		{"no name", func(p *SSOProvider) { p.DisplayName = " " }, validOIDC, "display_name"},
		{"long name", func(p *SSOProvider) { p.DisplayName = strings.Repeat("x", 65) }, validOIDC, "display_name"},
		{"no client", func(p *SSOProvider) { p.ClientID = "" }, validOIDC, "client_id"},
		{"bad role", func(p *SSOProvider) { p.DefaultRole = "owner" }, validOIDC, "default_role"},
		{"bad mapping", func(p *SSOProvider) { p.RoleMapping = map[string]Role{"g": "owner"} }, validOIDC, "role_mapping"},
		{"blank group", func(p *SSOProvider) { p.RoleMapping = map[string]Role{" ": RoleAdmin} }, validOIDC, "role_mapping"},
		{"bad domain", func(p *SSOProvider) { p.AllowedDomains = []string{"a@b.com"} }, validOIDC, "allowed_domains"},
		{"dotless domain", func(p *SSOProvider) { p.AllowedDomains = []string{"localhost"} }, validOIDC, "allowed_domains"},
		{"no issuer", func(p *SSOProvider) { p.Issuer = "" }, validOIDC, "issuer"},
		{"relative issuer", func(p *SSOProvider) { p.Issuer = "id.example.com" }, validOIDC, "issuer"},
		{"issuer with query", func(p *SSOProvider) { p.Issuer = "https://id.example.com/?a=b" }, validOIDC, "issuer"},
		{"plain http issuer", func(p *SSOProvider) { p.Issuer = "http://id.example.com" }, validOIDC, "issuer"},
		{"orgs on oidc", func(p *SSOProvider) { p.AllowedOrgs = []string{"acme"} }, validOIDC, "allowed_orgs"},
		{"no openid scope", func(p *SSOProvider) { p.Scopes = []string{"email"} }, validOIDC, "scopes"},
		{"issuer on github", func(p *SSOProvider) { p.Issuer = "https://x.example" }, validGitHub, "issuer"},
		{"trust on github", func(p *SSOProvider) { p.TrustMFA = true }, validGitHub, "trust_mfa"},
		{"half enterprise", func(p *SSOProvider) { p.WebURL = "https://ghe.example" }, validGitHub, "web_url"},
		{"bad enterprise api", func(p *SSOProvider) {
			p.WebURL, p.APIURL = "https://ghe.example", "ftp://ghe.example"
		}, validGitHub, "api_url"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.base()
			tc.edit(&p)
			err := p.Validate()
			if !IsKind(err, KindInvalid) || FieldOf(err) != tc.field {
				t.Errorf("Validate = %v (field %q), want invalid naming %q", err, FieldOf(err), tc.field)
			}
		})
	}
	for _, p := range []SSOProvider{validOIDC(), validGitHub()} {
		if err := p.Validate(); err != nil {
			t.Errorf("%s: %v", p.Kind, err)
		}
	}
	loop := validOIDC()
	loop.Issuer = "http://127.0.0.1:8080"
	ghe := validGitHub()
	ghe.WebURL, ghe.APIURL = "https://ghe.example", "https://ghe.example/api/v3"
	for _, p := range []SSOProvider{loop, ghe} {
		if err := p.Validate(); err != nil {
			t.Errorf("%s %s: %v", p.Kind, p.Issuer, err)
		}
	}
}

func TestSSOProviderInputsBuildAndChangeAProvider(t *testing.T) {
	p := CreateSSOProviderInput{Key: " KC ", Kind: SSOKindOIDC, Issuer: " https://id.example.com/ ",
		ClientID: "tix", AllowedDomains: []string{"@Example.com", "example.com", " "}, Scopes: []string{"openid", "openid"},
		RoleMapping: map[string]Role{"admins": RoleAdmin}, Disabled: true}.Provider()
	p.Normalize()
	if p.Key != "kc" || p.DisplayName != "kc" || p.Enabled || p.DefaultRole != RoleMember ||
		len(p.AllowedDomains) != 1 || p.AllowedDomains[0] != "example.com" || len(p.Scopes) != 1 ||
		p.Issuer != "https://id.example.com/" {
		t.Fatalf("provider = %+v", p)
	}
	name, on, role := "Keycloak", true, RoleViewer
	scopes, mapping := []string{"openid", "groups"}, map[string]Role{}
	changed := UpdateSSOProviderInput{DisplayName: &name, Enabled: &on, DefaultRole: &role, Scopes: &scopes,
		RoleMapping: &mapping, TrustMFA: &on}.Apply(p)
	changed.Normalize()
	if changed.DisplayName != name || !changed.Enabled || changed.DefaultRole != role || len(changed.Scopes) != 2 ||
		changed.RoleMapping != nil || !changed.TrustMFA {
		t.Errorf("changed = %+v", changed)
	}
	p.SealedSecret, p.SecretNonce, p.SecretHint, p.RedirectURL = "s", "n", "h", "r"
	if r := p.Redacted(); r.SealedSecret != "" || r.SecretNonce != "" || r.SecretHint != "" || r.RedirectURL != "" {
		t.Errorf("redacted = %+v", r)
	}
}

func TestSSOSmallPredicates(t *testing.T) {
	if !PasswordLogin("").Valid() || PasswordLogin("maybe").Valid() || PasswordLogin("").Disabled() ||
		!PasswordLoginDisabled.Disabled() {
		t.Error("PasswordLogin predicates")
	}
	if !IsSSOMethod(LoginMethodSSO("kc")) || IsSSOMethod(LoginMethodPassword) {
		t.Error("IsSSOMethod")
	}
	if !IsLoopbackHost("::1") || IsLoopbackHost("example.com") {
		t.Error("IsLoopbackHost")
	}
	if SSOKindSAML.Implemented() || !SSOKindGitHub.Implemented() {
		t.Error("Implemented")
	}
	bad := PasswordLogin("maybe")
	if err := (UpdateTenantInput{PasswordLogin: &bad}).Validate(); FieldOf(err) != "password_login" {
		t.Errorf("UpdateTenantInput = %v", err)
	}
}
