// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestTheClaimRequirementRoundTripsOnPostgres pins the BOOLEAN translation of
// api_tokens.require_claim on this engine.
func TestTheClaimRequirementRoundTripsOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	for name, want := range map[string]bool{"agent-a": true, "agent-b": false} {
		tok := core.APIToken{ActorID: f.actor.ID, Name: name, Scopes: []core.Scope{core.ScopeTaskRead}, RequireClaim: want}
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateToken(ctx, &tok, "hash-"+name) }); err != nil {
			t.Fatalf("CreateToken: %v", err)
		}
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			listed, err := tx.ListTokens(ctx, f.actor.ID)
			if err != nil {
				return err
			}
			for _, got := range listed {
				if got.ID == tok.ID && got.RequireClaim != want {
					t.Errorf("%s read back require_claim %v, want %v", name, got.RequireClaim, want)
				}
			}
			return nil
		}); err != nil {
			t.Fatalf("listing tokens: %v", err)
		}
	}
}
