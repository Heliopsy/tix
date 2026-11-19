// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// withdrawnTokens answers every lookup as a token whose actor is disabled.
type withdrawnTokens struct{ id, name string }

func (w withdrawnTokens) TokenByHash(context.Context, string) (*core.APIToken, error) {
	return nil, Withdrawn(w.id, w.name)
}

func TestTokenVerifierNotesWhyItRefused(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	v := NewTokenVerifier(store, clk)
	mint := func(in core.CreateTokenInput) *core.IssuedToken {
		in.ActorID = "actor-1"
		if in.Scopes == nil {
			in.Scopes = []core.Scope{core.ScopeTaskRead}
		}
		return mintInto(t, store, clk, in)
	}
	soon := clk.Now().Add(time.Minute)
	expired := mint(core.CreateTokenInput{Name: "old", ExpiresAt: &soon})
	revoked := mint(core.CreateTokenInput{Name: "gone"})
	revokedAt := clk.Now()
	store.byHash[HashToken(revoked.Token)].RevokedAt = &revokedAt
	scim := mint(core.CreateTokenInput{Name: "idp", Scopes: []core.Scope{core.ScopeSCIMProvision}})
	clk.Advance(time.Hour)
	stranger, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{Name: "x", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		name     string
		verifier *TokenVerifier
		value    string
		want     Refusal
	}{
		{"malformed", v, "tix_pat_short", Refusal{Reason: RefusedMalformedToken}},
		{"unknown", v, stranger.Issued.Token,
			Refusal{Reason: RefusedUnknownToken, TokenRef: CredentialRef(stranger.Issued.Token)}},
		{"expired", v, expired.Token, Refusal{Reason: RefusedTokenExpired, TokenID: expired.ID, TokenName: "old"}},
		{"revoked", v, revoked.Token, Refusal{Reason: RefusedTokenRevoked, TokenID: revoked.ID, TokenName: "gone"}},
		{"scim only", v, scim.Token, Refusal{Reason: RefusedSCIMOnly, TokenID: scim.ID, TokenName: "idp"}},
		{"actor disabled", NewTokenVerifier(withdrawnTokens{"tok-1", "ci"}, clk), stranger.Issued.Token,
			Refusal{Reason: RefusedActorDisabled, TokenID: "tok-1", TokenName: "ci"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, note := WithRefusalNote(context.Background())
			_, err := tc.verifier.Verify(ctx, tc.value)
			if err == nil {
				t.Fatal("verified, want a refusal")
			}
			if got, ok := note.Get(); !ok || got != tc.want {
				t.Errorf("noted %+v (%v), want %+v", got, ok, tc.want)
			}
			if strings.Contains(err.Error(), tc.value) {
				t.Errorf("error %q carries the presented value", err)
			}
		})
	}
}

func TestCredentialRefIsStableAndNotTheValueOrItsHash(t *testing.T) {
	value := TokenPrefix + strings.Repeat("a", tokenEncoding.EncodedLen(tokenBytes))
	ref := CredentialRef(value)
	if ref != CredentialRef(value) {
		t.Fatal("the reference changed between calls")
	}
	if !strings.HasPrefix(ref, "t-") || len(ref) != 18 {
		t.Errorf("reference %q, want t- and 16 hex", ref)
	}
	if strings.Contains(HashToken(value), ref[2:]) || strings.Contains(value, ref[2:]) {
		t.Errorf("reference %q is part of the value or of its stored hash", ref)
	}
}

func TestBearerAuthenticatorNotesAnotherScheme(t *testing.T) {
	cases := map[string]string{
		"Basic dXNlcjpwYXNz": RefusedUnsupportedScheme,
		"Token abc":          RefusedUnsupportedScheme,
		"Bearer   ":          RefusedMalformedToken,
	}
	a := NewBearerAuthenticator(NewTokenVerifier(newTokenStore(), clock.NewFakeAt()))
	for header, want := range cases {
		t.Run(header, func(t *testing.T) {
			r, _ := http.NewRequest(http.MethodGet, "/", nil)
			r.Header.Set("Authorization", header)
			ctx, note := WithRefusalNote(context.Background())
			if _, err := a.Authenticate(ctx, r); !errors.Is(err, ErrNoCredential) {
				t.Fatalf("err = %v, want the authenticator to abstain", err)
			}
			if got, _ := note.Get(); got.Reason != want {
				t.Errorf("noted %q, want %q", got.Reason, want)
			}
		})
	}
}

// withdrawnSessions answers every lookup as a session whose actor is disabled.
type withdrawnSessions struct{}

func (withdrawnSessions) SessionByHash(context.Context, string) (*StoredSession, error) {
	return nil, Withdrawn("", "")
}

func TestSessionVerifierNotesWhyItRefused(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newSessionStore()
	v := NewSessionVerifier(store, clk)
	session := func(edit func(*StoredSession)) string {
		m := mintSessionInto(t, store, clk)
		edit(store.byHash[m.Stored.TokenHash])
		return m.Session.Token
	}
	expired := session(func(s *StoredSession) { s.ExpiresAt = clk.Now().Add(-time.Second) })
	idle := session(func(s *StoredSession) { s.IdleTimeout, s.LastSeenAt = time.Minute, clk.Now().Add(-time.Hour) })
	badRole := session(func(s *StoredSession) { s.Role = "emperor" })
	unknown := strings.Repeat("b", tokenEncoding.EncodedLen(tokenBytes))
	ref := func(token string) string { return SessionRef(HashToken(token)) }

	cases := []struct {
		name     string
		verifier *SessionVerifier
		token    string
		want     Refusal
	}{
		{"unknown", v, unknown, Refusal{Reason: RefusedUnknownSession, Session: ref(unknown)}},
		{"unknown and malformed", v, "not-a-session", Refusal{Reason: RefusedUnknownSession}},
		{"expired", v, expired, Refusal{Reason: RefusedSessionExpired, Session: ref(expired)}},
		{"idle", v, idle, Refusal{Reason: RefusedSessionIdle, Session: ref(idle)}},
		{"invalid role", v, badRole, Refusal{Reason: RefusedSessionInvalid, Session: ref(badRole)}},
		{"actor disabled", NewSessionVerifier(withdrawnSessions{}, clk), unknown,
			Refusal{Reason: RefusedActorDisabled, Session: ref(unknown)}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx, note := WithRefusalNote(context.Background())
			if _, err := tc.verifier.Verify(ctx, tc.token); err == nil {
				t.Fatal("verified, want a refusal")
			}
			if got, ok := note.Get(); !ok || got != tc.want {
				t.Errorf("noted %+v (%v), want %+v", got, ok, tc.want)
			}
		})
	}
}

func TestRefusalNoteKeepsTheLastAndCanBeCleared(t *testing.T) {
	NoteRefusal(context.Background(), Refusal{Reason: RefusedUnknownToken})
	ctx, note := WithRefusalNote(context.Background())
	NoteRefusal(ctx, Refusal{Reason: RefusedUnsupportedScheme})
	NoteRefusal(ctx, Refusal{Reason: RefusedSessionExpired})
	NoteRefusalUnlessNoted(ctx, Refusal{Reason: RefusedNoCredentials})
	if got, _ := note.Get(); got.Reason != RefusedSessionExpired {
		t.Errorf("noted %q, want the last refusal", got.Reason)
	}
	ClearRefusal(ctx)
	if _, ok := note.Get(); ok {
		t.Error("a cleared note still reports a refusal")
	}
	NoteRefusalUnlessNoted(ctx, Refusal{Reason: RefusedNoCredentials})
	if got, _ := note.Get(); got.Reason != RefusedNoCredentials {
		t.Errorf("noted %q, want %q on an empty note", got.Reason, RefusedNoCredentials)
	}
}
