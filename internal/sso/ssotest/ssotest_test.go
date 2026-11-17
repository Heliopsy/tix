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

// TestTheFakeProviderIssuesWhatItIsTold walks a whole sign-in against the
// fake and checks each tweak lands in the token it issues.
func TestTheFakeProviderIssuesWhatItIsTold(t *testing.T) {
	p := NewOIDC(t, time.Now)
	p.SetClaims(map[string]any{"sub": "s1", "email": "a@example.com"})
	p.SetUserinfo(map[string]any{"sub": "s1", "email": "b@example.com"})
	p.SetTweak(Tweak{Audience: "aud", Issuer: "iss", ExpiresIn: -time.Minute, Nonce: "other", ForeignKey: true})
	for _, path := range []string{"/.well-known/openid-configuration", "/keys"} {
		r, err := http.Get(p.Issuer() + path)
		if err != nil || r.StatusCode != http.StatusOK {
			t.Fatalf("%s = %v, %v", path, r, err)
		}
		_ = r.Body.Close()
	}
	verifier := strings.Repeat("v", 43)
	code := p.Grant("https://x/authorize?" + url.Values{"code_challenge": {challenge(verifier)}, "nonce": {"n"},
		"redirect_uri": {"http://127.0.0.1:9/cb"}}.Encode())
	r, err := http.PostForm(p.Issuer()+"/token", url.Values{"client_id": {p.ClientID}, "client_secret": {p.ClientSecret},
		"code": {code}, "redirect_uri": {"http://127.0.0.1:9/cb"}, "code_verifier": {verifier}})
	if err != nil {
		t.Fatal(err)
	}
	var tok map[string]any
	_ = json.NewDecoder(r.Body).Decode(&tok)
	_ = r.Body.Close()
	if tok["id_token"] == nil || tok["access_token"] == nil {
		t.Fatalf("token = %v", tok)
	}
	req, _ := http.NewRequest(http.MethodGet, p.Issuer()+"/userinfo", nil)
	req.Header.Set("Authorization", "Bearer "+tok["access_token"].(string))
	info, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var claims map[string]any
	_ = json.NewDecoder(info.Body).Decode(&claims)
	_ = info.Body.Close()
	if claims["email"] != "b@example.com" {
		t.Errorf("userinfo = %v, want the override", claims)
	}
	if p.Discoveries != 1 || p.KeyFetches != 1 {
		t.Errorf("counted %d discoveries and %d key fetches", p.Discoveries, p.KeyFetches)
	}
}

func TestTheFakeGitHubAnswersItsAccount(t *testing.T) {
	g := NewGitHub(t)
	g.SetUser(GitHubUser{ID: 9, Login: "ada", Orgs: []string{"acme"}})
	verifier := strings.Repeat("v", 43)
	code := g.Grant("https://x/?" + url.Values{"code_challenge": {challenge(verifier)}}.Encode())
	r, err := http.PostForm(g.WebURL()+"/login/oauth/access_token", url.Values{"client_id": {g.ClientID},
		"client_secret": {g.ClientSecret}, "code": {code}, "code_verifier": {verifier}})
	if err != nil {
		t.Fatal(err)
	}
	var tok map[string]any
	_ = json.NewDecoder(r.Body).Decode(&tok)
	_ = r.Body.Close()
	access, _ := tok["access_token"].(string)
	if access == "" {
		t.Fatalf("token = %v", tok)
	}
	for path, want := range map[string]int{"/user": 200, "/user/emails": 200, "/user/memberships/orgs/ACME": 200,
		"/user/memberships/orgs/other": 404} {
		req, _ := http.NewRequest(http.MethodGet, g.APIURL()+path, nil)
		req.Header.Set("Authorization", "Bearer "+access)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("%s = %d, want %d", path, resp.StatusCode, want)
		}
	}
	bad, err := http.PostForm(g.WebURL()+"/login/oauth/access_token", url.Values{"client_id": {g.ClientID},
		"client_secret": {g.ClientSecret}, "code": {"reused"}})
	if err != nil {
		t.Fatal(err)
	}
	_ = bad.Body.Close()
	if bad.StatusCode != http.StatusBadRequest {
		t.Errorf("an unknown code = %d", bad.StatusCode)
	}
}
