package auth

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// fakeTokenStore is an in-memory TokenLookup.
type fakeTokenStore struct {
	byHash map[string]*core.APIToken
	err    error
	touch  []string
}

func newTokenStore() *fakeTokenStore {
	return &fakeTokenStore{byHash: make(map[string]*core.APIToken)}
}

func (f *fakeTokenStore) TokenByHash(_ context.Context, hash string) (*core.APIToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	tok, ok := f.byHash[hash]
	if !ok {
		return nil, nil
	}
	return tok, nil
}

func (f *fakeTokenStore) TouchToken(_ context.Context, id string, _ time.Time) error {
	f.touch = append(f.touch, id)
	return nil
}

func mintInto(t *testing.T, store *fakeTokenStore, clk clock.Clock, in core.CreateTokenInput) *core.IssuedToken {
	t.Helper()
	minted, err := MintAPIToken(clk, "tenant-1", in)
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	store.byHash[minted.Hash] = &minted.Issued.APIToken
	return minted.Issued
}

func TestMintAPITokenFormat(t *testing.T) {
	clk := clock.NewFakeAt()
	minted, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{
		Name:   "agent",
		Scopes: []core.Scope{core.ScopeTaskRead},
	})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	value := minted.Issued.Token
	if !strings.HasPrefix(value, TokenPrefix) {
		t.Fatalf("token %q lacks the %q prefix", value, TokenPrefix)
	}
	if _, err := ParseAPIToken(value); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if minted.Hash == "" || strings.Contains(minted.Hash, value) {
		t.Fatal("stored hash must not contain the token value")
	}
	if minted.Issued.CreatedAt != clk.Now() {
		t.Fatalf("created at %v, want %v", minted.Issued.CreatedAt, clk.Now())
	}

	other, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{Name: "agent", Scopes: []core.Scope{core.ScopeTaskRead}})
	if err != nil {
		t.Fatalf("mint: %v", err)
	}
	if other.Issued.Token == value {
		t.Fatal("two mints produced the same token")
	}
}

func TestMintAPITokenRejectsBadInput(t *testing.T) {
	clk := clock.NewFakeAt()
	if _, err := MintAPIToken(clk, "", core.CreateTokenInput{Name: "a", Scopes: []core.Scope{core.ScopeTaskRead}}); err == nil {
		t.Fatal("expected a missing tenant to be rejected")
	}
	if _, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{Name: ""}); err == nil {
		t.Fatal("expected invalid input to be rejected")
	}
	if _, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{Name: "a", Scopes: []core.Scope{"bogus:scope"}}); err == nil {
		t.Fatal("expected an unknown scope to be rejected")
	}
}

func TestParseAPIToken(t *testing.T) {
	tests := []string{
		"",
		"nope",
		"tix_pat_",
		"tix_pat_UPPERCASE",
		"tix_pat_short",
		"tix_pat_" + strings.Repeat("!", 52),
		"tix_pat_" + strings.Repeat("A", 52),
		"tix_pat_" + strings.Repeat("1", 52),
	}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			if _, err := ParseAPIToken(in); err == nil {
				t.Fatalf("expected %q to be rejected", in)
			}
		})
	}
}

func TestTokenVerifier(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	v := NewTokenVerifier(store, clk)
	expires := clk.Now().Add(time.Hour)

	issued := mintInto(t, store, clk, core.CreateTokenInput{
		Name:      "agent",
		ActorID:   "actor-1",
		Scopes:    []core.Scope{core.ScopeTaskRead, core.ScopeTaskClaim},
		ProjectID: "project-1",
		ExpiresAt: &expires,
	})

	actor, err := v.Verify(context.Background(), issued.Token)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if actor.TenantID != "tenant-1" || actor.ProjectID != "project-1" || actor.TokenID != issued.ID {
		t.Fatalf("unexpected actor %+v", actor)
	}
	if actor.Kind != core.ActorAgent {
		t.Fatalf("actor kind = %q", actor.Kind)
	}
	if !actor.HasScope(core.ScopeTaskClaim) || actor.HasScope(core.ScopeTaskDelete) {
		t.Fatalf("unexpected scopes %v", actor.Scopes)
	}
	if len(store.touch) != 1 || store.touch[0] != issued.ID {
		t.Fatalf("last use not recorded: %v", store.touch)
	}

	t.Run("expiry", func(t *testing.T) {
		clk.Advance(2 * time.Hour)
		if _, err := v.Verify(context.Background(), issued.Token); !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatalf("expected an expired token to be rejected, got %v", err)
		}
	})
}

func TestTokenVerifierRejects(t *testing.T) {
	clk := clock.NewFakeAt()
	store := newTokenStore()
	v := NewTokenVerifier(store, clk)
	issued := mintInto(t, store, clk, core.CreateTokenInput{
		Name: "agent", ActorID: "actor-1", Scopes: []core.Scope{core.ScopeTaskRead},
	})

	t.Run("malformed", func(t *testing.T) {
		if _, err := v.Verify(context.Background(), "garbage"); err == nil {
			t.Fatal("expected a malformed token to be rejected")
		}
	})

	t.Run("unknown", func(t *testing.T) {
		other, err := MintAPIToken(clk, "tenant-1", core.CreateTokenInput{Name: "x", Scopes: []core.Scope{core.ScopeTaskRead}})
		if err != nil {
			t.Fatalf("mint: %v", err)
		}
		_, err = v.Verify(context.Background(), other.Issued.Token)
		if !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatalf("expected an unknown token to be rejected, got %v", err)
		}
		assertNoSecret(t, err.Error(), other.Issued.Token)
	})

	t.Run("store failure", func(t *testing.T) {
		store.err = errors.New("database is gone")
		defer func() { store.err = nil }()
		if _, err := v.Verify(context.Background(), issued.Token); err == nil {
			t.Fatal("expected a store failure to surface")
		}
	})

	t.Run("revoked", func(t *testing.T) {
		revoked := clk.Now()
		store.byHash[HashToken(issued.Token)].RevokedAt = &revoked
		_, err := v.Verify(context.Background(), issued.Token)
		if !core.IsKind(err, core.KindUnauthenticated) {
			t.Fatalf("expected a revoked token to be rejected, got %v", err)
		}
		assertNoSecret(t, err.Error(), issued.Token)
	})
}

func TestHashTokenIsStableAndOpaque(t *testing.T) {
	const value = "tix_pat_abcdefgh"
	first, second := HashToken(value), HashToken(value)
	if first != second {
		t.Fatal("hashing is not stable")
	}
	if strings.Contains(first, value) {
		t.Fatal("hash contains the token value")
	}
	if HashToken(value) == HashToken(value+"x") {
		t.Fatal("distinct tokens hashed equal")
	}
	if !EqualHash(first, second) || EqualHash(first, HashToken("other")) {
		t.Fatal("EqualHash is wrong")
	}
}
