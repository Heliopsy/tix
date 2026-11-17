// SPDX-License-Identifier: AGPL-3.0-or-later

package sso_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/sso"
	"github.com/heliopsy/tix/internal/sso/ssotest"
)

const redirect = "http://127.0.0.1:4000/callback"

func oidcProvider(idp *ssotest.OIDC) core.SSOProvider {
	return core.SSOProvider{Key: "kc", Kind: core.SSOKindOIDC, Issuer: idp.Issuer(), ClientID: idp.ClientID,
		GroupsClaim: "roles", MFAValues: []string{"otp"}}
}

func TestOIDCReadsTheConfiguredClaimsAndCachesDiscovery(t *testing.T) {
	idp := ssotest.NewOIDC(t, time.Now)
	idp.SetClaims(map[string]any{"sub": "s1", "email": " Ada@Example.COM ", "email_verified": "true",
		"name": "Ada", "preferred_username": "ada", "roles": []any{"a", "b"}, "amr": []any{"pwd", "otp"}})
	client := sso.NewClient(http.DefaultClient, time.Now)
	ctx := context.Background()
	adapter, err := client.Adapter(ctx, oidcProvider(idp), idp.ClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	auth := adapter.AuthURL("st", "nn", strings.Repeat("v", 43), redirect)
	u, _ := url.Parse(auth)
	if u.Query().Get("nonce") != "nn" || u.Query().Get("code_challenge_method") != "S256" ||
		u.Query().Get("scope") != "openid profile email" {
		t.Errorf("auth url = %s", auth)
	}
	got, err := adapter.Exchange(ctx, idp.Grant(auth), strings.Repeat("v", 43), redirect, "nn")
	if err != nil {
		t.Fatal(err)
	}
	if got.Subject != "s1" || got.Email != "ada@example.com" || !got.EmailVerified || got.Username != "ada" ||
		len(got.Groups) != 2 || !got.MFA {
		t.Errorf("identity = %+v", got)
	}
	if _, err := client.Adapter(ctx, oidcProvider(idp), ""); err != nil {
		t.Fatal(err)
	}
	if idp.Discoveries != 1 {
		t.Errorf("discovery read %d times, want once", idp.Discoveries)
	}
	client.Forget(idp.Issuer())
	if _, err := client.Adapter(ctx, oidcProvider(idp), ""); err != nil || idp.Discoveries != 2 {
		t.Errorf("after Forget discovery read %d times (%v), want twice", idp.Discoveries, err)
	}
}

func TestOIDCFillsAMissingEmailFromUserinfo(t *testing.T) {
	idp := ssotest.NewOIDC(t, time.Now)
	ctx := context.Background()
	adapter, err := sso.NewClient(http.DefaultClient, time.Now).Adapter(ctx, oidcProvider(idp), idp.ClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	verifier := strings.Repeat("w", 43)
	auth := adapter.AuthURL("st", "nn", verifier, redirect)
	idp.SetClaims(map[string]any{"sub": "s1"})
	code := idp.Grant(auth)
	idp.SetUserinfo(map[string]any{"sub": "s1", "email": "ada@example.com", "email_verified": true})
	got, err := adapter.Exchange(ctx, code, verifier, redirect, "nn")
	if err != nil {
		t.Fatal(err)
	}
	if got.Email != "ada@example.com" || !got.EmailVerified {
		t.Errorf("identity = %+v, want the email from userinfo", got)
	}
}

func TestOIDCRefusesAWrongVerifierAndAnUnreachableIssuer(t *testing.T) {
	idp := ssotest.NewOIDC(t, time.Now)
	ctx := context.Background()
	client := sso.NewClient(http.DefaultClient, time.Now)
	adapter, err := client.Adapter(ctx, oidcProvider(idp), idp.ClientSecret)
	if err != nil {
		t.Fatal(err)
	}
	auth := adapter.AuthURL("st", "nn", strings.Repeat("v", 43), redirect)
	if _, err := adapter.Exchange(ctx, idp.Grant(auth), strings.Repeat("x", 43), redirect, "nn"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("wrong verifier = %v, want refused", err)
	}
	gone := oidcProvider(idp)
	gone.Issuer = "http://127.0.0.1:1/nowhere"
	if _, err := client.Adapter(ctx, gone, ""); !core.IsKind(err, core.KindUpstream) {
		t.Errorf("unreachable issuer = %v, want an upstream error", err)
	}
}

func githubProvider(gh *ssotest.GitHub, orgs ...string) core.SSOProvider {
	return core.SSOProvider{Key: "gh", Kind: core.SSOKindGitHub, ClientID: gh.ClientID, AllowedOrgs: orgs,
		WebURL: gh.WebURL(), APIURL: gh.APIURL(), RoleMapping: map[string]core.Role{"admins": core.RoleAdmin}}
}

func TestGitHubReadsThePrimaryEmailAndOrganisations(t *testing.T) {
	cases := []struct {
		name     string
		user     ssotest.GitHubUser
		orgs     []string
		verified bool
		groups   int
		refused  bool
	}{
		{"primary verified", ssotest.GitHubUser{ID: 7, Login: "Ada", Orgs: []string{"acme", "admins"},
			Emails: []ssotest.GitHubEmail{{Email: "x@y.z"}, {Email: "ada@example.com", Primary: true, Verified: true}}},
			[]string{"acme"}, true, 2, false},
		{"primary unverified", ssotest.GitHubUser{ID: 7, Login: "ada", Orgs: []string{"acme"},
			Emails: []ssotest.GitHubEmail{{Email: "ada@example.com", Primary: true}}}, []string{"acme"}, false, 1, false},
		{"outside the organisation", ssotest.GitHubUser{ID: 7, Login: "ada"}, []string{"acme"}, false, 0, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gh := ssotest.NewGitHub(t)
			gh.SetUser(tc.user)
			ctx := context.Background()
			adapter, err := sso.NewClient(http.DefaultClient, time.Now).Adapter(ctx, githubProvider(gh, tc.orgs...), gh.ClientSecret)
			if err != nil {
				t.Fatal(err)
			}
			verifier := strings.Repeat("v", 43)
			auth := adapter.AuthURL("st", "", verifier, redirect)
			if !strings.Contains(auth, "read%3Aorg") {
				t.Errorf("auth url %s does not ask for read:org", auth)
			}
			got, err := adapter.Exchange(ctx, gh.Grant(auth), verifier, redirect, "")
			if tc.refused {
				if !core.IsKind(err, core.KindUnauthenticated) {
					t.Fatalf("exchange = %+v, %v; want refused", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got.Subject != "7" || got.Username != "ada" || got.EmailVerified != tc.verified || len(got.Groups) != tc.groups {
				t.Errorf("identity = %+v", got)
			}
		})
	}
}
