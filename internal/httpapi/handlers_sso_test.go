// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/egress"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/sso/ssotest"
	"github.com/heliopsy/tix/internal/wire"
)

const apiTOTPKey = "MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY="

func TestSSOProvidersOverHTTP(t *testing.T) {
	f := newFixtureOver(t, []service.Option{service.WithTOTPKey(apiTOTPKey), service.WithEgress(egress.Permissive())})
	idp := ssotest.NewOIDC(t, f.clock.Now)
	resp := f.call(http.MethodPost, wire.RouteSSOProviders, core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret, AutoCreate: true})
	mustStatus(t, resp, http.StatusCreated)
	var created core.SSOProvider
	decodeBody(t, resp, &created)
	if !created.SecretSet {
		t.Errorf("created = %+v, want the secret set", created)
	}
	path := strings.Replace(wire.RouteSSOProvider, "{id}", "kc", 1)
	off := false
	resp = f.call(http.MethodPatch, path, core.UpdateSSOProviderInput{AutoCreate: &off})
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	resp = f.call(http.MethodGet, wire.RouteSSOProviders, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()

	options := f.do(http.MethodGet, wire.RouteSignInOptions, f.hostA, "", nil)
	mustStatus(t, options, http.StatusOK)
	var opts core.SignInOptions
	decodeBody(t, options, &opts)
	if len(opts.Providers) != 1 || opts.Providers[0].Key != "kc" {
		t.Errorf("options = %+v", opts)
	}

	begin := f.do(http.MethodPost, wire.RouteSSOBegin, f.hostA, "", core.BeginSSOInput{Provider: "kc",
		RedirectURI: "http://127.0.0.1:5000/callback"})
	mustStatus(t, begin, http.StatusOK)
	var started core.SSOBegin
	decodeBody(t, begin, &started)
	idp.SetClaims(map[string]any{"sub": "n", "email": "nobody@example.com", "email_verified": true})
	done := f.do(http.MethodPost, wire.RouteSSOComplete, f.hostA, "", core.CompleteSSOInput{Flow: started.Flow,
		Code: idp.Grant(started.AuthURL), State: started.State})
	mustStatus(t, done, http.StatusForbidden)
	_ = done.Body.Close()

	resp = f.call(http.MethodGet, path, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	resp = f.call(http.MethodDelete, path, nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
}

func TestSSORoutesAnswerRefusalsInTheEnvelope(t *testing.T) {
	f := newFixtureOver(t, []service.Option{service.WithTOTPKey(apiTOTPKey)})
	missing := strings.Replace(wire.RouteSSOProvider, "{id}", "nope", 1)
	for _, tc := range []struct {
		method, path string
		body         any
		token        bool
		want         int
	}{
		{http.MethodGet, wire.RouteSSOProviders, nil, false, http.StatusUnauthorized},
		{http.MethodPost, wire.RouteSSOProviders, map[string]any{"key": "Bad Key"}, true, http.StatusBadRequest},
		{http.MethodPost, wire.RouteSSOProviders, "not an object", true, http.StatusBadRequest},
		{http.MethodGet, missing, nil, true, http.StatusNotFound},
		{http.MethodPatch, missing, map[string]any{}, true, http.StatusNotFound},
		{http.MethodPatch, missing, "not an object", true, http.StatusBadRequest},
		{http.MethodDelete, missing, nil, true, http.StatusNotFound},
		{http.MethodPost, wire.RouteSSOBegin, map[string]any{"provider": "nope", "redirect_uri": "http://127.0.0.1:1/cb"}, false, http.StatusNotFound},
		{http.MethodPost, wire.RouteSSOBegin, "not an object", false, http.StatusBadRequest},
		{http.MethodPost, wire.RouteSSOComplete, map[string]any{"flow": "x", "state": "y"}, false, http.StatusUnauthorized},
		{http.MethodPost, wire.RouteSSOComplete, "not an object", false, http.StatusBadRequest},
	} {
		token := ""
		if tc.token {
			token = f.tokenA
		}
		resp := f.do(tc.method, tc.path, f.hostA, token, tc.body)
		if resp.StatusCode != tc.want {
			t.Errorf("%s %s = %d, want %d", tc.method, tc.path, resp.StatusCode, tc.want)
		}
		_ = resp.Body.Close()
	}
}

func TestSAMLProviderAndItsMetadataOverHTTP(t *testing.T) {
	f := newFixtureOver(t, []service.Option{service.WithTOTPKey(apiTOTPKey), service.WithEgress(egress.Permissive())})
	idp := ssotest.NewSAML(t, f.clock.Now)
	resp := f.call(http.MethodPost, wire.RouteSSOProviders, core.CreateSSOProviderInput{Key: "okta",
		Kind: core.SSOKindSAML, IdPMetadataXML: idp.Metadata()})
	mustStatus(t, resp, http.StatusCreated)
	var created core.SSOProvider
	decodeBody(t, resp, &created)
	if created.IdPEntityID != idp.EntityID || len(created.IdPCertificates) != 1 {
		t.Errorf("created = %+v, want the IdP read from its metadata", created)
	}
	md := f.do(http.MethodGet, strings.Replace(wire.RouteSAMLMetadata, "{id}", "okta", 1), f.hostA, "", nil)
	mustStatus(t, md, http.StatusOK)
	body := readBody(t, md)
	if md.Header.Get("Content-Type") != wire.ContentSAMLMetadata || !strings.Contains(body, "/login/sso/saml/okta/acs") {
		t.Errorf("metadata = %s %s", md.Header.Get("Content-Type"), body)
	}
	missing := f.do(http.MethodGet, strings.Replace(wire.RouteSAMLMetadata, "{id}", "nobody", 1), f.hostA, "", nil)
	mustStatus(t, missing, http.StatusNotFound)
	_ = missing.Body.Close()
}

func TestAnAppExchangesItsSignInCodeOverHTTP(t *testing.T) {
	f := newFixtureOver(t, []service.Option{service.WithTOTPKey(apiTOTPKey), service.WithEgress(egress.Permissive())})
	idp := ssotest.NewOIDC(t, f.clock.Now)
	resp := f.call(http.MethodPost, wire.RouteSSOProviders, core.CreateSSOProviderInput{Key: "kc", Kind: core.SSOKindOIDC,
		Issuer: idp.Issuer(), ClientID: idp.ClientID, ClientSecret: idp.ClientSecret, AutoCreate: true})
	mustStatus(t, resp, http.StatusCreated)
	_ = resp.Body.Close()

	options := f.do(http.MethodGet, wire.RouteSignInOptions, f.hostA, "", nil)
	mustStatus(t, options, http.StatusOK)
	var raw map[string]any
	decodeBody(t, options, &raw)
	if raw["app_sign_in"] != true || raw["password_login"] != "allowed" || raw["providers"] == nil {
		t.Errorf("options = %v, want app_sign_in true beside password_login and providers", raw)
	}

	// The browser half, as /login/sso/kc runs it for the app.
	verifier := "app-verifier-0123456789-0123456789-0123456789"
	ctx := core.WithSource(core.WithTenant(context.Background(), core.TenantScope{TenantID: f.tenantA.ID}),
		core.SourceWeb)
	begin, err := f.local.BeginSSOLogin(ctx, core.BeginSSOInput{Provider: "kc",
		RedirectURI: "https://acme.test" + core.SSOCallbackPath, AppRedirect: "red.tix.mobile:/sso",
		CodeChallenge: auth.PKCEChallenge(verifier), CodeChallengeMethod: "S256", AppState: "s1"})
	if err != nil {
		t.Fatal(err)
	}
	idp.SetClaims(map[string]any{"sub": "n", "email": "ada@example.com", "email_verified": true})
	grant := idp.Grant(begin.AuthURL)
	// The app's flow is the browser's to finish: over the API nothing would
	// take the code to the app.
	overAPI := f.do(http.MethodPost, wire.RouteSSOComplete, f.hostA, "", core.CompleteSSOInput{Flow: begin.Flow,
		Code: grant, State: begin.State})
	mustStatus(t, overAPI, http.StatusBadRequest)
	var apiRefusal map[string]any
	decodeBody(t, overAPI, &apiRefusal)
	if !strings.Contains(fmt.Sprint(apiRefusal), "flow") {
		t.Errorf("an app flow completed over the API = %v, want refused naming flow", apiRefusal)
	}
	handed, err := f.local.CompleteSSOLogin(ctx, core.CompleteSSOInput{Flow: begin.Flow,
		Code: grant, State: begin.State})
	if err != nil {
		t.Fatal(err)
	}
	target, _ := url.Parse(handed.AppRedirect)
	code := target.Query().Get("code")

	for _, tc := range []struct {
		name string
		body any
		want int
	}{
		{"not an object", "nope", http.StatusBadRequest},
		{"no verifier", map[string]any{"code": code}, http.StatusBadRequest},
		{"unknown code", core.SSOExchangeInput{Code: "nothing-handed", CodeVerifier: verifier}, http.StatusUnauthorized},
	} {
		refused := f.do(http.MethodPost, wire.RouteSSOExchange, f.hostA, "", tc.body)
		mustStatus(t, refused, tc.want)
		_ = refused.Body.Close()
	}

	done := f.do(http.MethodPost, wire.RouteSSOExchange, f.hostA, "", core.SSOExchangeInput{Code: code,
		CodeVerifier: verifier})
	mustStatus(t, done, http.StatusOK)
	cookie := ""
	for _, c := range done.Cookies() {
		if c.Name == auth.SessionCookieName {
			cookie = c.Value
		}
	}
	var result core.LoginResult
	decodeBody(t, done, &result)
	if result.Session == nil || result.Session.Token == "" || cookie != result.Session.Token {
		t.Fatalf("exchange = %+v with cookie %q, want a session and its cookie, as a password sign-in", result, cookie)
	}
	if result.Session.TenantID != f.tenantA.ID || !result.Session.Remembered {
		t.Errorf("session = %+v, want a remembered session in the tenant the code was minted in", result.Session)
	}

	again := f.do(http.MethodPost, wire.RouteSSOExchange, f.hostA, "", core.SSOExchangeInput{Code: code,
		CodeVerifier: verifier})
	mustStatus(t, again, http.StatusUnauthorized)
	var refusal map[string]any
	decodeBody(t, again, &refusal)
	if !strings.Contains(fmt.Sprint(refusal), "invalid credentials") {
		t.Errorf("a reused code answered %v, want invalid credentials", refusal)
	}
}
