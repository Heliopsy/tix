// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
)

// sessionFixture is a tenant with two members, ada signed in twice from two
// user agents and bob signed in once.
type sessionFixture struct {
	l          *Local
	verifier   *auth.SessionVerifier
	ada, bob   *core.Actor
	first, alt *core.Session
	bobs       *core.Session
}

func newSessionFixture(t *testing.T) sessionFixture {
	t.Helper()
	l, clk, scope, admin := newLocal(t)
	seedUser(t, l, admin, "ada@example.com")
	seedUser(t, l, admin, "bob@example.com")
	signInAs := func(email, agent string) *core.Session {
		t.Helper()
		ctx := core.WithVia(loginContext(scope), core.Via{Kind: core.ViaWeb, UserAgent: agent})
		result, err := l.Login(ctx, core.LoginInput{Email: email, Password: testPassword, Surface: core.SurfaceWeb})
		if err != nil || result.Session == nil {
			t.Fatalf("signing in %s: %v", email, err)
		}
		clk.Advance(time.Minute)
		return result.Session
	}
	f := sessionFixture{l: l, verifier: auth.NewSessionVerifier(newStoreCredentials(l, scope), clk)}
	f.first = signInAs("ada@example.com", "Firefox/140")
	f.alt = signInAs("ada@example.com", "tix-cli/0.50")
	f.bobs = signInAs("bob@example.com", "Safari/18")
	var err error
	if f.ada, err = f.verifier.Verify(context.Background(), f.first.Token); err != nil {
		t.Fatalf("verifying ada: %v", err)
	}
	if f.bob, err = f.verifier.Verify(context.Background(), f.bobs.Token); err != nil {
		t.Fatalf("verifying bob: %v", err)
	}
	return f
}

// as is a request by actor presenting the session token, if any.
func (f sessionFixture) as(actor *core.Actor, token string) context.Context {
	ctx := authContext(actor)
	if token != "" {
		ctx = WithSessionToken(ctx, token)
	}
	return ctx
}

func TestListSessionsListsOnlyTheCallersOwnAndMarksTheCurrent(t *testing.T) {
	f := newSessionFixture(t)
	got, next, err := f.l.ListSessions(f.as(f.ada, f.first.Token), core.Page{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if next != "" || len(got) != 2 {
		t.Fatalf("listed %d sessions with cursor %q, want ada's 2 and none: %+v", len(got), next, got)
	}
	if got[0].UserAgent != "tix-cli/0.50" || got[1].UserAgent != "Firefox/140" {
		t.Errorf("user agents newest first = %q, %q", got[0].UserAgent, got[1].UserAgent)
	}
	if got[0].Current || !got[1].Current {
		t.Errorf("current = %v, %v; want only the presented session", got[0].Current, got[1].Current)
	}
	for _, s := range got {
		if s.UserAgent == "Safari/18" {
			t.Errorf("bob's session was listed to ada: %+v", s)
		}
		if s.Method != core.LoginMethodPassword || s.ID == "" || !s.ExpiresAt.After(s.CreatedAt) {
			t.Errorf("session reads oddly: %+v", s)
		}
		if strings.Contains(s.ID, f.first.Token) || s.ID == auth.HashToken(f.first.Token) {
			t.Errorf("a session identifier carries the secret: %+v", s)
		}
	}
}

func TestListSessionsPagesByCursor(t *testing.T) {
	f := newSessionFixture(t)
	ctx := f.as(f.ada, "")
	page, next, err := f.l.ListSessions(ctx, core.Page{Limit: 1})
	if err != nil || len(page) != 1 || next == "" {
		t.Fatalf("first page = %d sessions, cursor %q, err %v", len(page), next, err)
	}
	rest, last, err := f.l.ListSessions(ctx, core.Page{Limit: 1, Cursor: next})
	if err != nil || len(rest) != 1 || rest[0].ID == page[0].ID {
		t.Fatalf("second page = %+v, err %v; want the other session", rest, err)
	}
	if more, _, _ := f.l.ListSessions(ctx, core.Page{Limit: 1, Cursor: last}); len(more) != 0 {
		t.Errorf("a third page listed %d sessions", len(more))
	}
}

func TestSessionManagementWithATokenNeedsAccountRead(t *testing.T) {
	f := newSessionFixture(t)
	token := func(scopes ...core.Scope) *core.Actor {
		a := *f.ada
		a.TokenID, a.Scopes = "tok", scopes
		return &a
	}
	if _, _, err := f.l.ListSessions(f.as(token(core.ScopeAll), ""), core.Page{}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("listing with a * token = %v, want forbidden", err)
	}
	if err := f.l.RevokeSession(f.as(token(core.ScopeAll), ""), "anything"); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("revoking with a * token = %v, want forbidden", err)
	}
	got, _, err := f.l.ListSessions(f.as(token(core.ScopeAccountRead), ""), core.Page{})
	if err != nil || len(got) != 2 {
		t.Fatalf("listing with account:read = %d sessions, %v; want ada's 2", len(got), err)
	}
	if slices.ContainsFunc(got, func(s core.SessionInfo) bool { return s.Current }) {
		t.Error("a token request marked a session current")
	}
}

func TestRevokeSessionEndsOneOfTheCallersOwn(t *testing.T) {
	f := newSessionFixture(t)
	listed, _, err := f.l.ListSessions(f.as(f.ada, f.first.Token), core.Page{})
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	other := listed[0]
	if other.Current {
		t.Fatal("the newest session is the presented one; the fixture changed")
	}
	if err := f.l.RevokeSession(f.as(f.ada, f.first.Token), other.ID); err != nil {
		t.Fatalf("RevokeSession: %v", err)
	}
	if _, err := f.verifier.Verify(context.Background(), f.alt.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("the revoked session still verifies: %v", err)
	}
	if _, err := f.verifier.Verify(context.Background(), f.first.Token); err != nil {
		t.Errorf("the session in use stopped verifying: %v", err)
	}
	if !slices.Contains(auditActions(t, f.l, core.TenantScope{TenantID: f.ada.TenantID}), auditSessionRevoke) {
		t.Errorf("no %s audit entry", auditSessionRevoke)
	}
	if err := f.l.RevokeSession(f.as(f.ada, ""), other.ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("revoking it again = %v, want not found", err)
	}
}

func TestRevokingSomebodyElsesSessionIsNotFound(t *testing.T) {
	f := newSessionFixture(t)
	bobs, _, err := f.l.ListSessions(f.as(f.bob, ""), core.Page{})
	if err != nil || len(bobs) != 1 {
		t.Fatalf("bob's sessions = %+v, %v", bobs, err)
	}
	if err := f.l.RevokeSession(f.as(f.ada, ""), bobs[0].ID); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("ada revoking bob's session = %v, want not found", err)
	}
	if _, err := f.verifier.Verify(context.Background(), f.bobs.Token); err != nil {
		t.Errorf("bob was signed out by ada's attempt: %v", err)
	}
	if err := f.l.RevokeSession(f.as(f.ada, ""), " "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("revoking a blank identifier = %v, want invalid", err)
	}
}
