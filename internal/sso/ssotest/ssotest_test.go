// SPDX-License-Identifier: AGPL-3.0-or-later

package ssotest

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"
)

var noRedirect = &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}

// TestTheFakeProviderRefusesWhatARealOneWould pins the fake itself: a fake
// that accepted anything would make every refusal test above it pass.
func TestTheFakeProviderRefusesWhatARealOneWould(t *testing.T) {
	p := NewOIDC(t, time.Now)
	verifier := strings.Repeat("v", 43)
	auth := p.Issuer() + "/authorize?" + url.Values{"client_id": {p.ClientID}, "code_challenge": {challenge(verifier)},
		"code_challenge_method": {"S256"}, "redirect_uri": {"http://127.0.0.1:9/cb"}, "state": {"s"}, "nonce": {"n"}}.Encode()
	resp, err := noRedirect.Get(auth)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	back, _ := url.Parse(resp.Header.Get("Location"))
	if resp.StatusCode != http.StatusFound || back.Query().Get("state") != "s" || back.Query().Get("code") == "" {
		t.Fatalf("authorize = %d %s", resp.StatusCode, back)
	}
	bad, _ := noRedirect.Get(p.Issuer() + "/authorize?client_id=other")
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown client = %d, want 400", bad.StatusCode)
	}
	_ = bad.Body.Close()
	for _, form := range []url.Values{
		{"client_id": {p.ClientID}, "client_secret": {"wrong"}},
		{"client_id": {p.ClientID}, "client_secret": {p.ClientSecret}, "code": {back.Query().Get("code")},
			"redirect_uri": {"http://127.0.0.1:9/cb"}, "code_verifier": {"wrong"}},
	} {
		r, err := http.PostForm(p.Issuer()+"/token", form)
		if err != nil {
			t.Fatal(err)
		}
		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		_ = r.Body.Close()
		if r.StatusCode != http.StatusBadRequest || body["error"] == "" {
			t.Errorf("token %v = %d %v, want an oauth error", form, r.StatusCode, body)
		}
	}
	info, err := http.Get(p.Issuer() + "/userinfo")
	if err != nil {
		t.Fatal(err)
	}
	_ = info.Body.Close()
	if info.StatusCode != http.StatusUnauthorized {
		t.Errorf("userinfo without a token = %d", info.StatusCode)
	}
}

func TestTheFakeGitHubRefusesWhatGitHubWould(t *testing.T) {
	g := NewGitHub(t)
	resp, err := noRedirect.Get(g.WebURL() + "/login/oauth/authorize?redirect_uri=" + url.QueryEscape("http://127.0.0.1:9/cb") + "&state=s")
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound || !strings.Contains(resp.Header.Get("Location"), "state=s") {
		t.Fatalf("authorize = %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
	r, err := http.PostForm(g.WebURL()+"/login/oauth/access_token", url.Values{"client_id": {g.ClientID}})
	if err != nil {
		t.Fatal(err)
	}
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("a wrong secret = %d, want 400", r.StatusCode)
	}
	_ = r.Body.Close()
	for _, path := range []string{"/user", "/user/emails", "/user/memberships/orgs/acme"} {
		r, err := http.Get(g.APIURL() + path)
		if err != nil {
			t.Fatal(err)
		}
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s without a token = %d", path, r.StatusCode)
		}
		_ = r.Body.Close()
	}
}
