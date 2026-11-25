// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// termAuthenticator answers a cookie with a session actor whose term it was
// given, as the session verifier does after renewing one.
type termAuthenticator struct {
	actor core.Actor
	term  core.SessionTerm
}

func (a termAuthenticator) Authenticate(_ context.Context, r *http.Request) (*core.Actor, error) {
	if _, err := r.Cookie(wire.SessionCookieName); err != nil {
		return nil, core.Unauthenticated("invalid credentials")
	}
	out := a.actor
	term := a.term
	out.Term, out.SessionID = &term, "s-test"
	return &out, nil
}

// A remembered session renewed by a request gets its cookie again, lasting to
// the new expiry; WhoAmI reports that expiry whatever the session's kind.
func TestARenewedRememberedSessionIsSentItsCookieAgain(t *testing.T) {
	checked := time.Date(2026, 11, 12, 9, 0, 0, 0, time.UTC)
	expires := checked.Add(30 * 24 * time.Hour)
	tests := []struct {
		name   string
		term   core.SessionTerm
		cookie bool
	}{
		{"remembered and renewed", core.SessionTerm{ExpiresAt: expires, Remembered: true, Renewed: true,
			CheckedAt: checked}, true},
		{"remembered, not renewed", core.SessionTerm{ExpiresAt: expires, Remembered: true, CheckedAt: checked}, false},
		{"renewed, not remembered", core.SessionTerm{ExpiresAt: expires, Renewed: true, CheckedAt: checked}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := newFixtureWith(t, func(f *apiFixture, cfg *httpapi.Config) {
				cfg.Authenticator = termAuthenticator{actor: f.actorA, term: tt.term}
			})
			req := f.newRequest(http.MethodGet, wire.RouteWhoAmI, nil)
			req.Header.Del(wire.HeaderAuth)
			req.AddCookie(&http.Cookie{Name: wire.SessionCookieName, Value: "the-session"})
			resp, err := f.server.Client().Do(req)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = resp.Body.Close() }()
			mustStatus(t, resp, http.StatusOK)

			var sent *http.Cookie
			for _, c := range resp.Cookies() {
				if c.Name == wire.SessionCookieName {
					sent = c
				}
			}
			switch {
			case tt.cookie && (sent == nil || sent.Value != "the-session" || sent.MaxAge != 30*24*3600 || !sent.HttpOnly):
				t.Errorf("cookie = %+v; want the same session again with a 30 day Max-Age", sent)
			case !tt.cookie && sent != nil:
				t.Errorf("cookie = %+v; want none", sent)
			}
			var who struct {
				SessionExpiresAt *time.Time `json:"session_expires_at"`
			}
			if err := json.NewDecoder(resp.Body).Decode(&who); err != nil {
				t.Fatal(err)
			}
			if who.SessionExpiresAt == nil || !who.SessionExpiresAt.Equal(expires) {
				t.Errorf("whoami session_expires_at = %v, want %v", who.SessionExpiresAt, expires)
			}
		})
	}
}
