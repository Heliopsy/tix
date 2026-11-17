// SPDX-License-Identifier: AGPL-3.0-or-later

// Package ssotest runs fake single sign-on providers in process, for tests:
// an OpenID Connect provider with discovery, a key set, an authorization
// endpoint, a token endpoint and userinfo, and a GitHub with its OAuth and
// API endpoints. Each can be told to misbehave in the ways a real provider,
// or an attacker standing in for one, might.
package ssotest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// OIDC is a fake OpenID Connect provider.
type OIDC struct {
	// Server is the provider's HTTP server; its URL is the issuer.
	Server *httptest.Server
	// ClientID and ClientSecret are what the provider accepts.
	ClientID     string
	ClientSecret string

	mu     sync.Mutex
	key    *rsa.PrivateKey
	keyID  string
	other  *rsa.PrivateKey
	now    func() time.Time
	codes  map[string]grant
	claims map[string]any
	tweak  Tweak
	info   map[string]map[string]any

	userinfoOverride map[string]any

	// Discoveries and KeyFetches count requests, so a test can see caching.
	Discoveries int
	KeyFetches  int
}

// Tweak makes the next tokens wrong in one way.
type Tweak struct {
	// Audience replaces the token's audience.
	Audience string
	// Issuer replaces the token's issuer.
	Issuer string
	// ExpiresIn replaces the token's lifetime; negative is already expired.
	ExpiresIn time.Duration
	// Nonce replaces the nonce the sign-in sent.
	Nonce string
	// ForeignKey signs with a key the key set does not publish.
	ForeignKey bool
	// NoIDToken leaves the id token out of the token response.
	NoIDToken bool
}

type grant struct {
	challenge   string
	nonce       string
	redirectURI string
	claims      map[string]any
}

// NewOIDC starts a fake provider and stops it when the test ends. now is
// the clock tokens are issued by.
func NewOIDC(t testing.TB, now func() time.Time) *OIDC {
	t.Helper()
	// #nosec G101 -- a fake provider's test client, not a credential.
	p := &OIDC{ClientID: "tix-client", ClientSecret: "tix-client-secret-value", keyID: "k1", now: now,
		codes: map[string]grant{}, claims: map[string]any{}, info: map[string]map[string]any{}}
	p.key = mustKey(t)
	p.other = mustKey(t)
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /keys", p.keys)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("GET /userinfo", p.userinfo)
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Server.Close)
	return p
}

func mustKey(t testing.TB) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

// Issuer is the provider's issuer URL.
func (p *OIDC) Issuer() string { return p.Server.URL }

// SetClaims sets the claims the next sign-ins carry beside the standard ones.
// "sub" is required.
func (p *OIDC) SetClaims(c map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims = c
}

// SetTweak makes every following token wrong as t says.
func (p *OIDC) SetTweak(t Tweak) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tweak = t
}

// SetUserinfo sets what the userinfo endpoint answers for the next
// sign-ins, in place of the ID token's claims.
func (p *OIDC) SetUserinfo(c map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userinfoOverride = c
}

// Grant issues a code as the authorization endpoint would after the person
// signed in, for the sign-in an auth URL began. It returns the code.
func (p *OIDC) Grant(authURL string) string {
	u, _ := url.Parse(authURL)
	q := u.Query()
	code := randomText()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.codes[code] = grant{challenge: q.Get("code_challenge"), nonce: q.Get("nonce"),
		redirectURI: q.Get("redirect_uri"), claims: p.claims}
	return code
}

func (p *OIDC) discovery(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	p.Discoveries++
	p.mu.Unlock()
	writeJSON(w, map[string]any{
		"issuer":                                p.Issuer(),
		"authorization_endpoint":                p.Issuer() + "/authorize",
		"token_endpoint":                        p.Issuer() + "/token",
		"jwks_uri":                              p.Issuer() + "/keys",
		"userinfo_endpoint":                     p.Issuer() + "/userinfo",
		"id_token_signing_alg_values_supported": []string{"RS256"},
	})
}

func (p *OIDC) keys(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	p.KeyFetches++
	p.mu.Unlock()
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &p.key.PublicKey, KeyID: p.keyID, Algorithm: "RS256", Use: "sig"},
	}})
}

// authorize stands in for the person signing in: it grants a code at once
// and redirects back, as a browser following the provider would see.
func (p *OIDC) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	if q.Get("client_id") != p.ClientID || q.Get("code_challenge_method") != "S256" {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	code := p.Grant(r.URL.String())
	back, _ := url.Parse(q.Get("redirect_uri"))
	bq := back.Query()
	bq.Set("code", code)
	bq.Set("state", q.Get("state"))
	back.RawQuery = bq.Encode()
	// #nosec G710 -- a test fake answering the redirect its own caller named.
	http.Redirect(w, r, back.String(), http.StatusFound)
}

func (p *OIDC) token(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		oauthError(w, "invalid_request")
		return
	}
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.PostForm.Get("client_id"), r.PostForm.Get("client_secret")
	}
	if id != p.ClientID || secret != p.ClientSecret {
		oauthError(w, "invalid_client")
		return
	}
	p.mu.Lock()
	g, found := p.codes[r.PostForm.Get("code")]
	delete(p.codes, r.PostForm.Get("code"))
	tweak := p.tweak
	p.mu.Unlock()
	if !found || g.redirectURI != r.PostForm.Get("redirect_uri") || challenge(r.PostForm.Get("code_verifier")) != g.challenge {
		oauthError(w, "invalid_grant")
		return
	}
	out := map[string]any{"access_token": "at-" + randomText(), "token_type": "Bearer", "expires_in": 300}
	if !tweak.NoIDToken {
		out["id_token"] = p.idToken(g, tweak, out["access_token"].(string))
	}
	writeJSON(w, out)
}

func (p *OIDC) idToken(g grant, tweak Tweak, access string) string {
	now := p.now()
	claims := map[string]any{"iss": p.Issuer(), "aud": p.ClientID, "iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(), "nonce": g.nonce}
	for k, v := range g.claims {
		claims[k] = v
	}
	if tweak.Audience != "" {
		claims["aud"] = tweak.Audience
	}
	if tweak.Issuer != "" {
		claims["iss"] = tweak.Issuer
	}
	if tweak.ExpiresIn != 0 {
		claims["exp"] = now.Add(tweak.ExpiresIn).Unix()
	}
	if tweak.Nonce != "" {
		claims["nonce"] = tweak.Nonce
	}
	key := p.key
	if tweak.ForeignKey {
		key = p.other
	}
	p.mu.Lock()
	p.info[access] = g.claims
	if p.userinfoOverride != nil {
		p.info[access] = p.userinfoOverride
	}
	p.mu.Unlock()
	return sign(key, p.keyID, claims)
}

func (p *OIDC) userinfo(w http.ResponseWriter, r *http.Request) {
	access := r.Header.Get("Authorization")
	p.mu.Lock()
	claims, ok := p.info[trimBearer(access)]
	p.mu.Unlock()
	if !ok {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	writeJSON(w, claims)
}

func trimBearer(h string) string {
	if len(h) > 7 {
		return h[7:]
	}
	return ""
}

func sign(key *rsa.PrivateKey, keyID string, claims map[string]any) string {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", keyID))
	if err != nil {
		panic(err)
	}
	payload, _ := json.Marshal(claims)
	obj, err := signer.Sign(payload)
	if err != nil {
		panic(err)
	}
	out, err := obj.CompactSerialize()
	if err != nil {
		panic(err)
	}
	return out
}

func challenge(verifier string) string {
	sum := sha256.Sum256([]byte(verifier))
	return base64.RawURLEncoding.EncodeToString(sum[:])
}

func randomText() string {
	raw := make([]byte, 16)
	_, _ = rand.Read(raw)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func oauthError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}
