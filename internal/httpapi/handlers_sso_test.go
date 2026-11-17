// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

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
