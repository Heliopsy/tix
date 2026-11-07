// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

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

// A reissued link replaces the old one, which stops at once; a revoked link
// stops too, and both refuse an unknown invite.
func TestInviteReissueAndRevokeOverHTTP(t *testing.T) {
	f := newFixture(t)
	resp := f.call(http.MethodPost, wire.RouteInvites, core.CreateInviteInput{Email: "ada@example.com", Role: core.RoleMember})
	mustStatus(t, resp, http.StatusCreated)
	var first core.IssuedInvite
	decodeBody(t, resp, &first)

	reissue := strings.Replace(wire.RouteInviteReissue, "{id}", first.ID, 1)
	resp = f.call(http.MethodPost, reissue, core.ReissueInviteInput{ExpiresIn: core.Duration(48 * time.Hour)})
	mustStatus(t, resp, http.StatusOK)
	var second core.IssuedInvite
	decodeBody(t, resp, &second)
	if second.Token == "" || second.Token == first.Token || !strings.HasSuffix(second.URL, "/invite/"+second.Token) {
		t.Fatalf("reissued %+v", second)
	}
	old := inviteRequest(f, http.MethodGet, first.Token, nil)
	mustStatus(t, old, http.StatusNotFound)
	_ = old.Body.Close()
	live := inviteRequest(f, http.MethodGet, second.Token, nil)
	mustStatus(t, live, http.StatusOK)
	_ = live.Body.Close()

	bad := f.call(http.MethodPost, reissue, core.ReissueInviteInput{ExpiresIn: core.Duration(time.Minute)})
	mustStatus(t, bad, http.StatusBadRequest)
	_ = bad.Body.Close()

	// The replaced invite is finished, so revoking it is refused.
	stale := f.call(http.MethodDelete, strings.Replace(wire.RouteInvite, "{id}", first.ID, 1), nil)
	mustStatus(t, stale, http.StatusNotFound)
	_ = stale.Body.Close()
	resp = f.call(http.MethodDelete, strings.Replace(wire.RouteInvite, "{id}", second.ID, 1), nil)
	mustStatus(t, resp, http.StatusNoContent)
	_ = resp.Body.Close()
	gone := inviteRequest(f, http.MethodGet, second.Token, nil)
	mustStatus(t, gone, http.StatusNotFound)
	_ = gone.Body.Close()

	for _, req := range []struct{ method, route string }{
		{http.MethodDelete, strings.Replace(wire.RouteInvite, "{id}", "nope", 1)},
		{http.MethodPost, strings.Replace(wire.RouteInviteReissue, "{id}", "nope", 1)},
	} {
		resp := f.call(req.method, req.route, nil)
		mustStatus(t, resp, http.StatusNotFound)
		_ = resp.Body.Close()
	}
}
