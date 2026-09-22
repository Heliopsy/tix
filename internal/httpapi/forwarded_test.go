package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// login posts a password login carrying the given forwarded headers and
// returns the session cookie the server set.
func loginWithHeaders(t *testing.T, f *apiFixture, headers map[string]string) *http.Cookie {
	t.Helper()
	raw, err := json.Marshal(httpapi.LoginRequest{
		Email: "ada@example.com", Password: "correct-horse-battery"})
	if err != nil {
		t.Fatalf("marshalling login: %v", err)
	}
	req := f.newRequest(http.MethodPost, wire.RouteLogin, bytes.NewReader(raw))
	req.Header.Set(wire.HeaderContentType, wire.ContentJSON)
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatalf("login: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	mustStatus(t, resp, http.StatusOK)
	for _, c := range resp.Cookies() {
		if c.Name == auth.SessionCookieName {
			return c
		}
	}
	t.Fatal("login set no session cookie")
	return nil
}

// TestSessionCookieSecureFollowsForwardedProto pins that the documented
// reverse-proxy deployment ships a Secure session cookie, and that a client
// cannot obtain or suppress one by sending the header itself.
func TestSessionCookieSecureFollowsForwardedProto(t *testing.T) {
	tests := []struct {
		name     string
		trusted  []string
		security string
		forced   bool
		header   string
		want     bool
	}{
		{name: "plain http, nothing trusted", want: false},
		{name: "forged header from an untrusted client", header: "https", want: false},
		{name: "trusted proxy terminating tls", trusted: []string{"127.0.0.1", "::1"},
			header: "https", want: true},
		{name: "trusted proxy on plain http", trusted: []string{"127.0.0.1", "::1"},
			header: "http", want: false},
		{name: "explicit always", security: httpapi.CookieSecurityAlways, want: true},
		{name: "explicit never overrides a trusted proxy", security: httpapi.CookieSecurityNever,
			trusted: []string{"127.0.0.1", "::1"}, header: "https", want: false},
		{name: "own certificate still forces secure", forced: true, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
				cfg.TrustedProxies = tt.trusted
				cfg.CookieSecurity = tt.security
				cfg.SecureCookies = tt.forced
			})
			headers := map[string]string{}
			if tt.header != "" {
				headers[auth.HeaderForwardedProto] = tt.header
			}
			if got := loginWithHeaders(t, f, headers).Secure; got != tt.want {
				t.Fatalf("session cookie Secure = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestRouterRejectsBadProxyConfiguration pins that an address that cannot be
// parsed fails at startup rather than silently trusting nothing.
func TestRouterRejectsBadProxyConfiguration(t *testing.T) {
	base := func() httpapi.Config {
		return httpapi.Config{
			Service: newAPIService(nil),
			Authenticator: auth.NewChain(auth.NewBearerAuthenticator(
				auth.NewTokenVerifier(mapTokens{byHash: map[string]*core.APIToken{}}, clock.NewFakeAt()))),
			Resolver: stubResolver{},
		}
	}
	tests := []struct {
		name   string
		mutate func(*httpapi.Config)
	}{
		{"unparseable proxy", func(cfg *httpapi.Config) { cfg.TrustedProxies = []string{"not-an-ip"} }},
		{"unknown cookie security", func(cfg *httpapi.Config) { cfg.CookieSecurity = "sometimes" }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := base()
			tt.mutate(&cfg)
			if _, err := httpapi.New(cfg); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("expected an invalid error, got %v", err)
			}
		})
	}
}

// stubResolver maps no host to a tenant.
type stubResolver struct{}

func (stubResolver) ResolveDomain(context.Context, string) (*core.Tenant, error) {
	return nil, core.NotFound("no tenant")
}

// TestClientIPIsResolvedOnce pins that the client address a later guard, such
// as rate limiting, needs is resolved by the middleware and honours the
// forwarded header only from a trusted proxy.
func TestClientIPIsResolvedOnce(t *testing.T) {
	tests := []struct {
		name    string
		trusted []string
		xff     string
		want    string
	}{
		{"untrusted peer keeps its own address", nil, "198.51.100.7", "127.0.0.1"},
		{"trusted proxy speaks for its client", []string{"127.0.0.1", "::1"}, "198.51.100.7", "198.51.100.7"},
		{"trusted proxy with no header", []string{"127.0.0.1", "::1"}, "", "127.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			seen := make(chan string, 1)
			f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
				cfg.TrustedProxies = tt.trusted
				cfg.WebHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					seen <- httpapi.ClientIPFrom(r.Context()).String()
					w.WriteHeader(http.StatusOK)
				})
			})
			req := f.newRequest(http.MethodGet, "/", nil)
			if tt.xff != "" {
				req.Header.Set(auth.HeaderForwardedFor, tt.xff)
			}
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatalf("request: %v", err)
			}
			_ = resp.Body.Close()
			if got := <-seen; got != tt.want {
				t.Fatalf("client ip = %q, want %q", got, tt.want)
			}
		})
	}
}
