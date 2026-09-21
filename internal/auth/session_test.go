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

// fakeSessionStore is an in-memory SessionLookup.
type fakeSessionStore struct {
	byHash map[string]*StoredSession
	err    error
}

func newSessionStore() *fakeSessionStore {
	return &fakeSessionStore{byHash: make(map[string]*StoredSession)}
}

func (f *fakeSessionStore) SessionByHash(_ context.Context, hash string) (*StoredSession, error) {
	if f.err != nil {
		return nil, f.err
	}
	s, ok := f.byHash[hash]
	if !ok {
		return nil, nil
	}
	return s, nil
}

func mintSessionInto(t *testing.T, store *fakeSessionStore, clk clock.Clock) *MintedSession {
	t.Helper()
	minted, err := MintSession(clk, SessionInput{
		ActorID:  "actor-1",
		TenantID: "tenant-1",
		Handle:   "ada@example.com",
		Role:     core.RoleMember,
		TTL:      time.Hour,
	})
	if err != nil {
		t.Fatalf("mint session: %v", err)
	}
	store.byHash[minted.Stored.TokenHash] = &minted.Stored
	return minted
}

func TestMintSession(t *testing.T) {
	clk := clock.NewFakeAt()
	minted, err := MintSession(clk, SessionInput{
		ActorID: "actor-1", TenantID: "tenant-1", Role: core.RoleMember, TTL: time.Hour,
	})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if minted.Session.Token == "" {
		t.Fatal("no session token returned")
	}
	if strings.Contains(minted.Stored.TokenHash, minted.Session.Token) {
		t.Fatal("stored session carries the token value")
	}
	if minted.Stored.TokenHash != HashToken(minted.Session.Token) {
		t.Fatal("stored hash does not match the token")
	}
	if want := clk.Now().Add(time.Hour); !minted.Session.ExpiresAt.Equal(want) {
		t.Fatalf("expiry = %v, want %v", minted.Session.ExpiresAt, want)
	}

	other, err := MintSession(clk, SessionInput{ActorID: "actor-1", TenantID: "tenant-1", TTL: time.Hour})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if other.Session.Token == minted.Session.Token {
		t.Fatal("two mints produced the same session token")
	}
}

func TestMintSessionRejectsBadInput(t *testing.T) {
	clk := clock.NewFakeAt()
	tests := []struct {
		name string
		in   SessionInput
	}{
		{"no actor", SessionInput{TenantID: "tenant-1", TTL: time.Hour}},
		{"no tenant", SessionInput{ActorID: "actor-1", TTL: time.Hour}},
		{"no ttl", SessionInput{ActorID: "actor-1", TenantID: "tenant-1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := MintSession(clk, tt.in); !core.IsKind(err, core.KindInvalid) {
				t.Fatalf("expected an invalid error, got %v", err)
			}
		})
	}
}

func TestSessionVerifier(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newSessionStore()
	v := NewSessionVerifier(store, clk)
	minted := mintSessionInto(t, store, clk)

	actor, err := v.Verify(context.Background(), minted.Session.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.ID != "actor-1" || actor.TenantID != "tenant-1" || actor.Role != core.RoleMember {
		t.Fatalf("unexpected actor %+v", actor)
	}
	if actor.Kind != core.ActorUser {
		t.Fatalf("actor kind = %q", actor.Kind)
	}

	clk.Advance(2 * time.Hour)
	_, err = v.Verify(context.Background(), minted.Session.Token)
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("expected an expired session to be rejected, got %v", err)
	}
	assertNoSecret(t, err.Error(), minted.Session.Token)
}

func TestSessionVerifierRejects(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newSessionStore()
	v := NewSessionVerifier(store, clk)
	minted := mintSessionInto(t, store, clk)

	t.Run("empty", func(t *testing.T) {
		if _, err := v.Verify(context.Background(), ""); !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatalf("expected an empty token to be rejected, got %v", err)
		}
	})

	t.Run("unknown", func(t *testing.T) {
		if _, err := v.Verify(context.Background(), "unknown-token"); !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatal("expected an unknown session to be rejected")
		}
	})

	t.Run("store failure", func(t *testing.T) {
		store.err = errors.New("database is gone")
		defer func() { store.err = nil }()
		_, err := v.Verify(context.Background(), minted.Session.Token)
		if err == nil {
			t.Fatal("expected a store failure to surface")
		}
		assertNoSecret(t, err.Error(), minted.Session.Token)
	})

	t.Run("revoked", func(t *testing.T) {
		at := clk.Now()
		store.byHash[minted.Stored.TokenHash].RevokedAt = &at
		if _, err := v.Verify(context.Background(), minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatal("expected a revoked session to be rejected")
		}
	})
}

func TestNewSessionCookie(t *testing.T) {
	clk := clock.NewFakeAt()
	expires := clk.Now().Add(time.Hour)

	plain := NewSessionCookie("value", expires, false)
	if plain.Name != SessionCookieName || !plain.HttpOnly || plain.Secure {
		t.Fatalf("unexpected cookie %+v", plain)
	}
	if plain.SameSite != http.SameSiteLaxMode || plain.Path != "/" {
		t.Fatalf("unexpected cookie attributes %+v", plain)
	}

	secure := NewSessionCookie("value", expires, true)
	if !secure.Secure || !secure.HttpOnly {
		t.Fatalf("unexpected cookie %+v", secure)
	}

	cleared := ClearSessionCookie(true)
	if cleared.Value != "" || cleared.MaxAge >= 0 {
		t.Fatalf("unexpected cleared cookie %+v", cleared)
	}
}

// TestSessionVerifierFollowsMembership pins that authority comes from the
// membership the lookup reports now, never from the role the session was
// minted with: a demotion lands on the next request, and an actor removed from
// the tenant is left holding nothing.
func TestSessionVerifierFollowsMembership(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newSessionStore()
	v := NewSessionVerifier(store, clk)
	minted := mintSessionInto(t, store, clk)
	stored := store.byHash[minted.Stored.TokenHash]

	t.Run("demotion applies", func(t *testing.T) {
		stored.Role = core.RoleViewer
		actor, err := v.Verify(context.Background(), minted.Session.Token)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if actor.Role != core.RoleViewer {
			t.Fatalf("role = %q, want %q", actor.Role, core.RoleViewer)
		}
		if actor.HasScope(core.ScopeUserAdmin) {
			t.Fatal("a demoted session kept administrative scope")
		}
	})

	t.Run("removed member holds nothing", func(t *testing.T) {
		stored.Role = ""
		actor, err := v.Verify(context.Background(), minted.Session.Token)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		for _, scope := range core.AllScopes {
			if actor.HasScope(scope) {
				t.Fatalf("an actor with no membership still holds %q", scope)
			}
		}
	})

	// mintSessionInto mints with RoleMember, so a verifier answering RoleViewer
	// can only be reading what the lookup reports now.
	t.Run("the minted role is never the answer", func(t *testing.T) {
		stored.Role = core.RoleViewer
		actor, err := v.Verify(context.Background(), minted.Session.Token)
		if err != nil {
			t.Fatalf("verify: %v", err)
		}
		if actor.Role != core.RoleViewer {
			t.Fatalf("role = %q, want the current %q", actor.Role, core.RoleViewer)
		}
	})

	t.Run("unknown role is refused", func(t *testing.T) {
		stored.Role = core.Role("superuser")
		if _, err := v.Verify(context.Background(), minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatalf("a session carrying an unknown role was accepted: %v", err)
		}
	})
}
