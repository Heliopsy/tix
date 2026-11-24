// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// davOwners answers every owner lookup with one person.
type davOwners map[string]*DAVOwner

func (o davOwners) DAVOwner(_ context.Context, actorID string) (*DAVOwner, error) {
	if owner, ok := o[actorID]; ok {
		return owner, nil
	}
	return nil, core.NotFound("actor not found")
}

func davRequest(t *testing.T, setup func(r *http.Request)) *http.Request {
	t.Helper()
	r, err := http.NewRequest("PROPFIND", "/dav/", nil)
	if err != nil {
		t.Fatal(err)
	}
	if setup != nil {
		setup(r)
	}
	return r
}

// Every credential but an app password over Basic is refused under /dav/,
// each with the reason the refusal log records, and the right one signs in
// by email or by handle in any case.
func TestBasicDAVAuthenticatorAcceptsOnlyAnAppPassword(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	verifier := NewTokenVerifier(store, clk)
	a := NewBasicDAVAuthenticator(verifier, davOwners{"ana": {Email: "ana@example.com", Handle: "ana"}})
	app := mintInto(t, store, clk, core.CreateTokenInput{Name: "dav-ana-iphone", ActorID: "ana",
		Scopes: []core.Scope{core.ScopeCaldav}})
	pat := mintInto(t, store, clk, core.CreateTokenInput{Name: "cli", ActorID: "ana", Scopes: []core.Scope{core.ScopeAll}})
	gone := mintInto(t, store, clk, core.CreateTokenInput{Name: "dav-ana-old", ActorID: "ana",
		Scopes: []core.Scope{core.ScopeCaldav}})
	revokedAt := clk.Now()
	store.byHash[HashToken(gone.Token)].RevokedAt = &revokedAt
	clk.Advance(time.Minute)

	cases := []struct {
		name   string
		setup  func(r *http.Request)
		reason string
		ok     bool
	}{
		{"nothing", nil, "", false},
		{"a session cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "abc"})
		}, RefusedDAVPasswordRequired, false},
		{"a bearer token", func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+app.Token) },
			RefusedDAVPasswordRequired, false},
		{"an account password", func(r *http.Request) { r.SetBasicAuth("ana@example.com", "correct horse") },
			RefusedDAVPasswordRequired, false},
		{"an ordinary token", func(r *http.Request) { r.SetBasicAuth("ana@example.com", pat.Token) },
			RefusedDAVPasswordRequired, false},
		{"somebody else's name", func(r *http.Request) { r.SetBasicAuth("raj@example.com", app.Token) },
			RefusedDAVUserMismatch, false},
		{"a revoked password", func(r *http.Request) { r.SetBasicAuth("ana@example.com", gone.Token) },
			RefusedTokenRevoked, false},
		{"the email", func(r *http.Request) { r.SetBasicAuth("ana@example.com", app.Token) }, "", true},
		{"the email in capitals", func(r *http.Request) { r.SetBasicAuth("ANA@Example.COM", app.Token) }, "", true},
		{"the handle", func(r *http.Request) { r.SetBasicAuth("Ana", app.Token) }, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store.touch = nil
			ctx, note := WithRefusalNote(core.WithDAVRequest(context.Background()))
			actor, err := a.Authenticate(ctx, davRequest(t, tc.setup))
			if tc.ok {
				if err != nil || actor == nil || actor.TokenID != app.ID || !actor.HasScope(core.ScopeCaldav) {
					t.Fatalf("Authenticate = %+v, %v; want the app password's actor", actor, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("signed in as %+v", actor)
			}
			if tc.reason == "" {
				if !errors.Is(err, ErrNoCredential) {
					t.Errorf("err = %v, want ErrNoCredential", err)
				}
				return
			}
			if !core.IsKind(err, core.KindUnauthenticated) {
				t.Errorf("err = %v, want unauthenticated", err)
			}
			if got, _ := note.Get(); got.Reason != tc.reason {
				t.Errorf("noted %q, want %q", got.Reason, tc.reason)
			}
			if slices.Contains(store.touch, pat.ID) {
				t.Error("a refused ordinary token was recorded as used")
			}
		})
	}
}

// An account password is refused by its shape, before any hash: with every
// hashing slot held, a check that hashed would answer busy instead.
func TestBasicDAVAuthenticatorNeverHashesAPassword(t *testing.T) {
	release := HoldHashSlotsInTests()
	defer release()
	a := NewBasicDAVAuthenticator(NewTokenVerifier(newTokenStore(), clock.NewFakeAt()), davOwners{})
	ctx, note := WithRefusalNote(core.WithDAVRequest(context.Background()))
	_, err := a.Authenticate(ctx, davRequest(t, func(r *http.Request) { r.SetBasicAuth("ana@example.com", "hunter2") }))
	if IsHashBusy(err) || !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("err = %v, want unauthenticated without waiting on a hash", err)
	}
	if got, _ := note.Get(); got.Reason != RefusedDAVPasswordRequired {
		t.Errorf("noted %q", got.Reason)
	}
}

// An app password is refused everywhere but /dav/, as a SCIM token is
// everywhere but /scim/v2.
func TestTokenVerifierConfinesAnAppPasswordToDAV(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	app := mintInto(t, store, clk, core.CreateTokenInput{Name: "dav", ActorID: "ana", Scopes: []core.Scope{core.ScopeCaldav}})
	v := NewTokenVerifier(store, clk)
	for _, route := range []struct {
		name string
		ctx  context.Context
		ok   bool
	}{
		{"the api", context.Background(), false},
		{"scim", core.WithSCIMRequest(context.Background()), false},
		{"dav", core.WithDAVRequest(context.Background()), true},
	} {
		t.Run(route.name, func(t *testing.T) {
			ctx, note := WithRefusalNote(route.ctx)
			_, err := v.Verify(ctx, app.Token)
			if route.ok {
				if err != nil {
					t.Fatalf("refused under /dav/: %v", err)
				}
				return
			}
			if !core.IsKind(err, core.KindForbidden) {
				t.Fatalf("err = %v, want forbidden", err)
			}
			if got, _ := note.Get(); got.Reason != RefusedCaldavOnly {
				t.Errorf("noted %q, want %q", got.Reason, RefusedCaldavOnly)
			}
		})
	}
}
