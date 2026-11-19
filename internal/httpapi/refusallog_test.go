// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// teeSink writes every record as JSON into a buffer, which is what a secret
// is searched for in, and hands it to the test as logSink does.
type teeSink struct {
	*logSink
	mu   sync.Mutex
	buf  bytes.Buffer
	json slog.Handler
}

func newTeeSink() *teeSink {
	s := &teeSink{logSink: newLogSink()}
	s.json = slog.NewJSONHandler(lockedWriter{s}, &slog.HandlerOptions{Level: slog.LevelDebug})
	return s
}

type lockedWriter struct{ s *teeSink }

func (w lockedWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	return w.s.buf.Write(p)
}

func (s *teeSink) Handle(ctx context.Context, r slog.Record) error {
	if err := s.json.Handle(ctx, r); err != nil {
		return err
	}
	return s.logSink.Handle(ctx, r)
}

func (s *teeSink) WithAttrs([]slog.Attr) slog.Handler { return s }
func (s *teeSink) WithGroup(string) slog.Handler      { return s }

// output is everything logged so far.
func (s *teeSink) output() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// lines returns the records logged for one request: its request line and,
// when there is one, its refusal line. A probe request is sent after it so a
// missing refusal line is known to be missing rather than late.
func (s *teeSink) lines(t *testing.T, f *apiFixture, path string) (request loggedRecord, refusal *loggedRecord) {
	t.Helper()
	resp := f.do(http.MethodGet, wire.RouteHealth, f.hostA, "", nil)
	_ = resp.Body.Close()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case rec := <-s.records:
			switch {
			case rec.msg == "request" && rec.attrs["path"] == path:
				request = rec
			case rec.msg == httpapi.MsgAuthRefused && rec.attrs["path"] == path:
				refusal = &rec
			case rec.msg == "request" && rec.attrs["path"] == wire.RouteHealth:
				return request, refusal
			}
		case <-timeout:
			t.Fatalf("no request line for %s was logged", path)
		}
	}
}

// refusalFixture is the API fixture logging into a tee sink on its fake
// clock, behind a bearer and a session authenticator.
func refusalFixture(t *testing.T, mutators ...func(*apiFixture, *httpapi.Config)) (*apiFixture, *teeSink, *sessionTable) {
	t.Helper()
	sink := newTeeSink()
	sessions := &sessionTable{byHash: map[string]*auth.StoredSession{}}
	f := newFixtureWith(t, append([]func(*apiFixture, *httpapi.Config){func(f *apiFixture, cfg *httpapi.Config) {
		cfg.Logger = slog.New(sink)
		cfg.Clock = f.clock
		cfg.Authenticator = auth.NewChain(
			auth.NewBearerAuthenticator(auth.NewTokenVerifier(f.lookup, f.clock)),
			auth.NewCookieAuthenticator(auth.NewSessionVerifier(sessions, f.clock)),
		)
	}}, mutators...)...)
	return f, sink, sessions
}

// sessionTable is an in-memory session lookup.
type sessionTable struct {
	byHash map[string]*auth.StoredSession
}

func (s *sessionTable) SessionByHash(_ context.Context, hash string) (*auth.StoredSession, error) {
	return s.byHash[hash], nil
}

// addToken stores a token for actor A of tenant A and returns its value.
func addToken(t *testing.T, f *apiFixture, name string, scopes []core.Scope, edit func(*core.APIToken)) (string, *core.APIToken) {
	t.Helper()
	minted, err := auth.MintAPIToken(f.clock, f.tenantA.ID, core.CreateTokenInput{
		Name: name, ActorID: f.actorA.ID, Scopes: scopes,
	})
	if err != nil {
		t.Fatalf("minting token: %v", err)
	}
	stored := minted.Issued.APIToken
	if edit != nil {
		edit(&stored)
	}
	f.lookup.byHash[minted.Hash] = &stored
	return minted.Issued.Token, &stored
}

func TestRefusedCredentialsAreLoggedWithTheirReason(t *testing.T) {
	f, sink, sessions := refusalFixture(t)
	past := f.clock.Now().Add(-time.Minute)
	expired, expiredTok := addToken(t, f, "nightly", []core.Scope{core.ScopeAll},
		func(tok *core.APIToken) { tok.ExpiresAt = &past })
	revoked, revokedTok := addToken(t, f, "laptop", []core.Scope{core.ScopeAll},
		func(tok *core.APIToken) { tok.RevokedAt = &past })
	narrow, narrowTok := addToken(t, f, "reader", []core.Scope{core.ScopeTaskRead}, nil)
	stranger, err := auth.MintAPIToken(f.clock, f.tenantA.ID, core.CreateTokenInput{Name: "x", Scopes: []core.Scope{core.ScopeAll}})
	if err != nil {
		t.Fatal(err)
	}
	unknown := stranger.Issued.Token
	staleSession, err := auth.MintSession(f.clock, auth.SessionInput{ActorID: f.actorA.ID, TenantID: f.tenantA.ID,
		Role: core.RoleMember, TTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	staleSession.Stored.ExpiresAt = past
	sessions.byHash[staleSession.Stored.TokenHash] = &staleSession.Stored
	sessionRef := auth.SessionRef(staleSession.Stored.TokenHash)
	tokenB := f.tokenB

	cases := []struct {
		name    string
		path    string
		host    string
		header  string
		cookie  string
		status  int
		reason  string
		want    map[string]string
		secrets []string
	}{
		{name: "no credentials", path: wire.RouteProjects, status: 401, reason: auth.RefusedNoCredentials},
		{name: "another scheme", path: wire.RouteProjects, header: "Basic dXNlcjpwYXNzd29yZA==", status: 401,
			reason: auth.RefusedUnsupportedScheme, secrets: []string{"dXNlcjpwYXNzd29yZA"}},
		{name: "malformed", path: wire.RouteProjects, header: "Bearer hunter2-not-a-token", status: 401,
			reason: auth.RefusedMalformedToken, secrets: []string{"hunter2"}},
		{name: "unknown", path: wire.RouteWorkflows, header: "Bearer " + unknown, status: 401,
			reason: auth.RefusedUnknownToken, want: map[string]string{"token_ref": auth.CredentialRef(unknown)},
			secrets: []string{unknown, strings.TrimPrefix(unknown, auth.TokenPrefix), auth.HashToken(unknown)}},
		{name: "expired", path: wire.RouteProjects, header: "Bearer " + expired, status: 401,
			reason: auth.RefusedTokenExpired, want: map[string]string{"token_id": expiredTok.ID, "token_name": "nightly"},
			secrets: []string{expired, strings.TrimPrefix(expired, auth.TokenPrefix)}},
		{name: "revoked", path: wire.RouteProjects, header: "Bearer " + revoked, status: 401,
			reason: auth.RefusedTokenRevoked, want: map[string]string{"token_id": revokedTok.ID, "token_name": "laptop"},
			secrets: []string{revoked}},
		{name: "wrong tenant", path: wire.RouteProjects, header: "Bearer " + tokenB, status: 401,
			reason: auth.RefusedWrongTenant, want: map[string]string{"token_name": "agent"}, secrets: []string{tokenB}},
		{name: "scope denied", path: wire.RouteProjects, header: "Bearer " + narrow, status: 403,
			reason: auth.RefusedScopeDenied, want: map[string]string{"token_id": narrowTok.ID, "token_name": "reader"},
			secrets: []string{narrow}},
		{name: "stale browser session", path: "/board", cookie: staleSession.Session.Token, status: 404,
			reason: auth.RefusedSessionExpired, want: map[string]string{"session": sessionRef},
			secrets: []string{staleSession.Session.Token, staleSession.Stored.TokenHash}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, f.server.URL+tc.path, nil)
			if err != nil {
				t.Fatal(err)
			}
			req.Host = f.hostA
			req.Header.Set("User-Agent", "lan-poller/1.0")
			if tc.header != "" {
				req.Header.Set(wire.HeaderAuth, tc.header)
			}
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: tc.cookie})
			}
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			mustStatus(t, resp, tc.status)

			request, refusal := sink.lines(t, f, tc.path)
			if refusal == nil {
				t.Fatalf("no refusal line for %s; request line %v", tc.name, request.attrs)
			}
			if refusal.level != slog.LevelWarn {
				t.Errorf("refusal logged at %s, want warn", refusal.level)
			}
			want := map[string]string{
				"reason": tc.reason, "status": strconv.Itoa(tc.status), "method": http.MethodGet,
				"client_ip": "127.0.0.1", "user_agent": "lan-poller/1.0", "tenant": f.tenantA.ID,
			}
			for k, v := range tc.want {
				want[k] = v
			}
			for k, v := range want {
				if got := refusal.attrs[k]; got != v {
					t.Errorf("refusal %s = %q, want %q", k, got, v)
				}
			}
			if request.attrs["auth_reason"] != tc.reason || request.attrs["client_ip"] != "127.0.0.1" ||
				request.attrs["user_agent"] != "lan-poller/1.0" {
				t.Errorf("request line %v, want auth_reason, client_ip and user_agent", request.attrs)
			}
			out := sink.output()
			for _, secret := range tc.secrets {
				if strings.Contains(out, secret) {
					t.Errorf("the log carries the presented credential %q", secret)
				}
			}
		})
	}
}

func TestARequestThatAuthenticatesLogsNoRefusal(t *testing.T) {
	f, sink, _ := refusalFixture(t)
	cases := map[string]struct{ path, token string }{
		"valid token":         {wire.RouteProjects, f.tokenA},
		"anonymous web visit": {"/board", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			resp := f.do(http.MethodGet, tc.path, f.hostA, tc.token, nil)
			_ = resp.Body.Close()
			request, refusal := sink.lines(t, f, tc.path)
			if refusal != nil {
				t.Errorf("logged a refusal %v", refusal.attrs)
			}
			if _, ok := request.attrs["auth_reason"]; ok {
				t.Errorf("request line carries auth_reason: %v", request.attrs)
			}
		})
	}
}

func TestRefusalClientAddressFollowsTheTrustedProxies(t *testing.T) {
	cases := map[string]struct {
		trusted []string
		want    string
	}{
		"trusted proxy":   {[]string{"127.0.0.0/8"}, "203.0.113.7"},
		"untrusted proxy": {nil, "127.0.0.1"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f, sink, _ := refusalFixture(t, func(_ *apiFixture, cfg *httpapi.Config) { cfg.TrustedProxies = tc.trusted })
			req := f.newRequest(http.MethodGet, wire.RouteProjects, nil)
			req.Header.Del(wire.HeaderAuth)
			req.Header.Set("X-Forwarded-For", "203.0.113.7")
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			request, refusal := sink.lines(t, f, wire.RouteProjects)
			if refusal == nil {
				t.Fatal("no refusal line")
			}
			if refusal.attrs["client_ip"] != tc.want || request.attrs["client_ip"] != tc.want {
				t.Errorf("client_ip = %q on the refusal and %q on the request, want %q",
					refusal.attrs["client_ip"], request.attrs["client_ip"], tc.want)
			}
		})
	}
}

func TestRepeatedRefusalsAreLoggedOnceAMinuteWithACount(t *testing.T) {
	f, sink, _ := refusalFixture(t)
	bad := "Bearer " + auth.TokenPrefix + "nope"
	refuse := func() *loggedRecord {
		t.Helper()
		req := f.newRequest(http.MethodGet, wire.RouteProjects, nil)
		req.Header.Set(wire.HeaderAuth, bad)
		resp, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		_, refusal := sink.lines(t, f, wire.RouteProjects)
		return refusal
	}
	steps := []struct {
		advance  time.Duration
		logged   bool
		withheld string
	}{
		{0, true, ""},
		{10 * time.Second, false, ""},
		{10 * time.Second, false, ""},
		{10 * time.Second, false, ""},
		{31 * time.Second, true, "3"},
		{time.Second, false, ""},
		{2 * time.Minute, true, "1"},
	}
	for i, s := range steps {
		f.clock.Advance(s.advance)
		got := refuse()
		if (got != nil) != s.logged {
			t.Fatalf("step %d: refusal logged = %v, want %v", i, got != nil, s.logged)
		}
		if got != nil && got.attrs["suppressed"] != s.withheld {
			t.Errorf("step %d: suppressed = %q, want %q", i, got.attrs["suppressed"], s.withheld)
		}
	}
}
