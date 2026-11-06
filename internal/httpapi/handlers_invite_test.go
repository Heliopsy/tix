// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// inviteRequest sends a request to a public invite route with no credential.
func inviteRequest(f *apiFixture, method, token string, body any) *http.Response {
	f.t.Helper()
	return f.do(method, strings.Replace(wire.RouteInviteToken, "{token}", token, 1), f.hostA, "", body)
}

func TestInviteOverHTTP(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodPost, wire.RouteInvites, core.CreateInviteInput{Email: "ada@example.com", Role: core.RoleMember})
	mustStatus(t, resp, http.StatusCreated)
	var issued core.IssuedInvite
	decodeBody(t, resp, &issued)
	u, err := url.Parse(issued.URL)
	if err != nil || u.Host != f.hostA || u.Path != "/invite/"+issued.Token || issued.Path != u.Path {
		t.Fatalf("issued url %q, path %q", issued.URL, issued.Path)
	}

	resp = f.call(http.MethodGet, wire.RouteInvites, nil)
	mustStatus(t, resp, http.StatusOK)
	if body := readBody(t, resp); !strings.Contains(body, "ada@example.com") || strings.Contains(body, issued.Token) {
		t.Fatalf("GET /invites = %s; want the address and no secret", body)
	}

	resp = inviteRequest(f, http.MethodGet, issued.Token, nil)
	mustStatus(t, resp, http.StatusOK)
	if resp.Header.Get("Referrer-Policy") != "no-referrer" {
		t.Errorf("Referrer-Policy = %q, want no-referrer", resp.Header.Get("Referrer-Policy"))
	}
	var preview core.InvitePreview
	decodeBody(t, resp, &preview)
	if preview.Email != "ada@example.com" || preview.ExistingAccount {
		t.Fatalf("preview = %+v", preview)
	}

	resp = inviteRequest(f, http.MethodPost, issued.Token, core.AcceptInviteInput{Password: "a-brand-new-password"})
	mustStatus(t, resp, http.StatusOK)
	signedIn := false
	for _, c := range resp.Cookies() {
		signedIn = signedIn || (c.Name == wire.SessionCookieName && c.Value != "")
	}
	_ = resp.Body.Close()
	if !signedIn {
		t.Fatal("accepting an invite set no session cookie")
	}

	resp = inviteRequest(f, http.MethodPost, issued.Token, core.AcceptInviteInput{Password: "a-brand-new-password"})
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()
	used := inviteRequest(f, http.MethodGet, issued.Token, nil)
	unknown := inviteRequest(f, http.MethodGet, "tix_inv_"+strings.Repeat("a", 52), nil)
	usedBody, unknownBody := readBody(t, used), readBody(t, unknown)
	if used.StatusCode != http.StatusNotFound || unknown.StatusCode != used.StatusCode || usedBody != unknownBody {
		t.Fatalf("used link %d %q, unknown link %d %q; want one answer", used.StatusCode, usedBody, unknown.StatusCode, unknownBody)
	}

	if logs := f.logs.String(); strings.Contains(logs, issued.Token) || !strings.Contains(logs, wire.InviteTokenPrefix+"[redacted]") {
		t.Fatalf("the request log carries the invite secret, or never logged the route:\n%s", logs)
	}
}

func TestInviteAdministrationNeedsACredential(t *testing.T) {
	f := newFixture(t)
	resp := f.do(http.MethodPost, wire.RouteInvites, f.hostA, "", core.CreateInviteInput{Email: "ada@example.com"})
	mustStatus(t, resp, http.StatusUnauthorized)
	_ = resp.Body.Close()
}
