package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
)

// stubAuthenticator records calls and returns a fixed result.
type stubAuthenticator struct {
	actor  *core.Actor
	err    error
	called int
}

func (s *stubAuthenticator) Authenticate(context.Context, *http.Request) (*core.Actor, error) {
	s.called++
	return s.actor, s.err
}

func TestChain(t *testing.T) {
	found := &core.Actor{ID: "actor-1", TenantID: "tenant-1"}
	boom := errors.New("broken")

	tests := []struct {
		name      string
		members   []*stubAuthenticator
		wantActor bool
		wantErr   bool
		wantCalls []int
	}{
		{
			name:      "empty chain",
			members:   nil,
			wantErr:   true,
			wantCalls: nil,
		},
		{
			name:      "first resolves",
			members:   []*stubAuthenticator{{actor: found}, {actor: found}},
			wantActor: true,
			wantCalls: []int{1, 0},
		},
		{
			name:      "falls through",
			members:   []*stubAuthenticator{{err: ErrNoCredential}, {actor: found}},
			wantActor: true,
			wantCalls: []int{1, 1},
		},
		{
			name:      "all abstain",
			members:   []*stubAuthenticator{{err: ErrNoCredential}, {err: ErrNoCredential}},
			wantErr:   true,
			wantCalls: []int{1, 1},
		},
		{
			name:      "real failure short-circuits",
			members:   []*stubAuthenticator{{err: boom}, {actor: found}},
			wantErr:   true,
			wantCalls: []int{1, 0},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			members := make([]Authenticator, 0, len(tt.members))
			for _, m := range tt.members {
				members = append(members, m)
			}
			chain := NewChain(members...)
			actor, err := chain.Authenticate(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil))
			if tt.wantErr && err == nil {
				t.Fatal("expected an error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tt.wantActor && actor == nil {
				t.Fatal("expected an actor")
			}
			for i, want := range tt.wantCalls {
				if tt.members[i].called != want {
					t.Fatalf("member %d called %d times, want %d", i, tt.members[i].called, want)
				}
			}
		})
	}
}

func TestBearerAuthenticator(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	issued := mintInto(t, store, clk, core.CreateTokenInput{
		Name: "agent", ActorID: "actor-1", Scopes: []core.Scope{core.ScopeTaskRead},
	})
	a := NewBearerAuthenticator(NewTokenVerifier(store, clk))

	t.Run("resolves", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer "+issued.Token)
		actor, err := a.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if actor.TokenID != issued.ID {
			t.Fatalf("unexpected actor %+v", actor)
		}
	})

	t.Run("case insensitive scheme", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "bearer "+issued.Token)
		if _, err := a.Authenticate(context.Background(), req); err != nil {
			t.Fatalf("authenticate: %v", err)
		}
	})

	t.Run("abstains", func(t *testing.T) {
		tests := []struct{ name, header string }{
			{"no header", ""},
			{"other scheme", "Basic abcdef"},
			{"no value", "Bearer "},
		}
		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				if tt.header != "" {
					req.Header.Set("Authorization", tt.header)
				}
				_, err := a.Authenticate(context.Background(), req)
				if !errors.Is(err, ErrNoCredential) {
					t.Fatalf("expected the authenticator to abstain, got %v", err)
				}
			})
		}
	})

	t.Run("bad token fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Header.Set("Authorization", "Bearer tix_pat_"+"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		_, err := a.Authenticate(context.Background(), req)
		if errors.Is(err, ErrNoCredential) || err == nil {
			t.Fatalf("expected an authentication failure, got %v", err)
		}
	})
}

func TestCookieAuthenticator(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newSessionStore()
	minted := mintSessionInto(t, store, clk)
	a := NewCookieAuthenticator(NewSessionVerifier(store, clk))

	t.Run("resolves", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(NewSessionCookie(minted.Session.Token, clk.Now().Add(time.Hour), false))
		actor, err := a.Authenticate(context.Background(), req)
		if err != nil {
			t.Fatalf("authenticate: %v", err)
		}
		if actor.ID != "actor-1" {
			t.Fatalf("unexpected actor %+v", actor)
		}
	})

	t.Run("abstains without a cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if _, err := a.Authenticate(context.Background(), req); !errors.Is(err, ErrNoCredential) {
			t.Fatalf("expected the authenticator to abstain, got %v", err)
		}
	})

	t.Run("abstains on an empty cookie", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: ""})
		if _, err := a.Authenticate(context.Background(), req); !errors.Is(err, ErrNoCredential) {
			t.Fatalf("expected the authenticator to abstain, got %v", err)
		}
	})

	t.Run("unknown session fails", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: "nope"})
		_, err := a.Authenticate(context.Background(), req)
		if errors.Is(err, ErrNoCredential) || err == nil {
			t.Fatalf("expected an authentication failure, got %v", err)
		}
	})
}

func TestStaticAuthenticator(t *testing.T) {
	want := core.SystemActor("tenant-1")
	a := NewStaticAuthenticator(want)
	actor, err := a.Authenticate(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil))
	if err != nil {
		t.Fatalf("authenticate: %v", err)
	}
	if actor.ID != want.ID || actor.TenantID != want.TenantID {
		t.Fatalf("unexpected actor %+v", actor)
	}
	if _, err := NewStaticAuthenticator(nil).Authenticate(context.Background(), httptest.NewRequest(http.MethodGet, "/", nil)); !errors.Is(err, ErrNoCredential) {
		t.Fatalf("expected a nil actor to abstain, got %v", err)
	}
}

func TestNoErrorLeaksCredentials(t *testing.T) {
	clk := clock.NewFakeAt()
	tokenStore := newTokenStore()
	sessionStore := newSessionStore()
	expires := clk.Now().Add(time.Hour)
	issued := mintInto(t, tokenStore, clk, core.CreateTokenInput{
		Name: "agent", ActorID: "actor-1", Scopes: []core.Scope{core.ScopeTaskRead}, ExpiresAt: &expires,
	})
	minted := mintSessionInto(t, sessionStore, clk)
	hashed, err := NewHasherWithParams(TestParams()).Hash(testPassword)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	clk.Advance(48 * time.Hour)

	chain := NewChain(
		NewBearerAuthenticator(NewTokenVerifier(tokenStore, clk)),
		NewCookieAuthenticator(NewSessionVerifier(sessionStore, clk)),
	)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Authorization", "Bearer "+issued.Token)
	req.AddCookie(&http.Cookie{Name: SessionCookieName, Value: minted.Session.Token})

	_, chainErr := chain.Authenticate(context.Background(), req)
	verifyErr := Verify(hashed, "the wrong password entirely")

	secrets := []string{issued.Token, minted.Session.Token, testPassword}
	for _, err := range []error{chainErr, verifyErr, ErrNoCredential} {
		if err == nil {
			t.Fatal("expected an error")
		}
		for _, secret := range secrets {
			assertNoSecret(t, err.Error(), secret)
		}
	}
	summary := minted.Stored.String()
	if summary == "" {
		t.Fatal("expected a printable session summary")
	}
	assertNoSecret(t, summary, minted.Session.Token)
	assertNoSecret(t, summary, minted.Stored.TokenHash)
}
