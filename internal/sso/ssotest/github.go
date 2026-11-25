// SPDX-License-Identifier: AGPL-3.0-or-later

package ssotest

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
)

// GitHub is a fake GitHub: OAuth under the server's root and the API under
// /api, as GitHub Enterprise Server lays them out.
type GitHub struct {
	// Server is the fake's HTTP server.
	Server *httptest.Server
	// ClientID and ClientSecret are what the fake accepts.
	ClientID     string
	ClientSecret string

	mu      sync.Mutex
	codes   map[string]string
	tokens  map[string]GitHubUser
	account GitHubUser
}

// GitHubUser is the account the next sign-in is for.
type GitHubUser struct {
	ID     int64
	Login  string
	Name   string
	Emails []GitHubEmail
	// Orgs are the organisations the account is an active member of.
	Orgs []string
}

// GitHubEmail is one of an account's addresses.
type GitHubEmail struct {
	Email    string `json:"email"`
	Primary  bool   `json:"primary"`
	Verified bool   `json:"verified"`
}

// NewGitHub starts a fake GitHub and stops it when the test ends.
func NewGitHub(t testing.TB) *GitHub {
	t.Helper()
	// #nosec G101 -- a fake GitHub's test client, not a credential.
	g := &GitHub{ClientID: "gh-client", ClientSecret: "gh-client-secret-value",
		codes: map[string]string{}, tokens: map[string]GitHubUser{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /login/oauth/authorize", g.authorize)
	mux.HandleFunc("POST /login/oauth/access_token", g.token)
	mux.HandleFunc("GET /api/user", g.user)
	mux.HandleFunc("GET /api/user/emails", g.emails)
	mux.HandleFunc("GET /api/user/memberships/orgs/{org}", g.membership)
	g.Server = httptest.NewServer(mux)
	t.Cleanup(g.Server.Close)
	return g
}

// WebURL and APIURL are what a provider names to reach the fake.
func (g *GitHub) WebURL() string { return g.Server.URL }

// APIURL is the fake's API address.
func (g *GitHub) APIURL() string { return g.Server.URL + "/api" }

// SetUser sets the account the next sign-ins are for.
func (g *GitHub) SetUser(u GitHubUser) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.account = u
}

// Grant issues a code for the sign-in an auth URL began.
func (g *GitHub) Grant(authURL string) string {
	u, _ := url.Parse(authURL)
	code := randomText()
	g.mu.Lock()
	defer g.mu.Unlock()
	g.codes[code] = u.Query().Get("code_challenge")
	return code
}

func (g *GitHub) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	code := g.Grant(r.URL.String())
	back, _ := url.Parse(q.Get("redirect_uri"))
	bq := back.Query()
	bq.Set("code", code)
	bq.Set("state", q.Get("state"))
	back.RawQuery = bq.Encode()
	// #nosec G710 -- a test fake answering the redirect its own caller named.
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (g *GitHub) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, "invalid_request")
		return
	}
	if r.PostForm.Get("client_id") != g.ClientID || r.PostForm.Get("client_secret") != g.ClientSecret {
		oauthError(w, "incorrect_client_credentials")
		return
	}
	g.mu.Lock()
	want, ok := g.codes[r.PostForm.Get("code")]
	delete(g.codes, r.PostForm.Get("code"))
	account := g.account
	g.mu.Unlock()
	if !ok || want != challenge(r.PostForm.Get("code_verifier")) {
		oauthError(w, "bad_verification_code")
		return
	}
	access := "gho_" + randomText()
	g.mu.Lock()
	g.tokens[access] = account
	g.mu.Unlock()
	writeJSON(w, map[string]any{"access_token": access, "token_type": "bearer", "scope": "read:user,user:email"})
}

func (g *GitHub) caller(r *http.Request) (GitHubUser, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	u, ok := g.tokens[strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")]
	return u, ok
}

func (g *GitHub) user(w http.ResponseWriter, r *http.Request) {
	u, ok := g.caller(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]any{"id": u.ID, "login": u.Login, "name": u.Name})
}

func (g *GitHub) emails(w http.ResponseWriter, r *http.Request) {
	u, ok := g.caller(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	out := u.Emails
	if out == nil {
		out = []GitHubEmail{}
	}
	writeJSON(w, out)
}

func (g *GitHub) membership(w http.ResponseWriter, r *http.Request) {
	u, ok := g.caller(r)
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	for _, org := range u.Orgs {
		if strings.EqualFold(org, r.PathValue("org")) {
			writeJSON(w, map[string]any{"state": "active"})
			return
		}
	}
	http.Error(w, "not found", http.StatusNotFound)
}
