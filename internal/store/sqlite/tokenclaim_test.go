// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// beforeRequireClaim is the newest migration before api_tokens.require_claim.
const beforeRequireClaim = 20261113134315

// TestATokenFromBeforeTheMigrationRequiresNoClaim is the upgrade path: a token
// minted before the column existed keeps behaving as it did.
func TestATokenFromBeforeTheMigrationRequiresNoClaim(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	rewindTo(t, s, beforeRequireClaim)
	insertToken(t, s, f, "01RCAAAAAAAAAAAAAAAAAAAAAA", "old")
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		tok, err := tx.GetToken(ctx, "01RCAAAAAAAAAAAAAAAAAAAAAA")
		if err != nil {
			return err
		}
		if tok.RequireClaim {
			t.Error("a token from before the migration requires a claim")
		}
		return nil
	}); err != nil {
		t.Fatalf("reading the old token: %v", err)
	}
}

func TestTheClaimRequirementRoundTrips(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	for _, want := range []bool{true, false} {
		tok := core.APIToken{ActorID: f.actor.ID, Name: "agent-" + map[bool]string{true: "a", false: "b"}[want],
			Scopes: []core.Scope{core.ScopeTaskRead}, RequireClaim: want}
		hash := "hash-" + tok.Name
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateToken(ctx, &tok, hash) }); err != nil {
			t.Fatalf("CreateToken: %v", err)
		}
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			byID, err := tx.GetToken(ctx, tok.ID)
			if err != nil {
				return err
			}
			byHash, err := tx.GetTokenByHash(ctx, hash)
			if err != nil {
				return err
			}
			if byID.RequireClaim != want || byHash.RequireClaim != want {
				t.Errorf("read back require_claim %v and %v, want %v", byID.RequireClaim, byHash.RequireClaim, want)
			}
			return nil
		}); err != nil {
			t.Fatalf("reading the token: %v", err)
		}
	}
}
