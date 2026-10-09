// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// nameInUse asks one tenant's transaction whether a token name is taken.
func nameInUse(t *testing.T, s *Store, scope core.TenantScope, name string) bool {
	t.Helper()
	ctx := context.Background()
	var taken bool
	if err := s.View(ctx, scope, func(tx store.Tx) error {
		var err error
		taken, err = tx.TokenNameInUse(ctx, name)
		return err
	}); err != nil {
		t.Fatalf("TokenNameInUse: %v", err)
	}
	return taken
}

// putToken creates a token through the store, which is the only path that
// writes the hash.
func putToken(t *testing.T, s *Store, scope core.TenantScope, actorID, id, name string) {
	t.Helper()
	ctx := context.Background()
	tok := core.APIToken{ID: id, ActorID: actorID, Name: name,
		Scopes: []core.Scope{core.ScopeTaskRead}}
	if err := s.Update(ctx, scope, func(tx store.Tx) error {
		return tx.CreateToken(ctx, &tok, "hash-"+id)
	}); err != nil {
		t.Fatalf("creating token %q: %v", name, err)
	}
}

// TestTokenNameInUseOnPostgres is the same rule the SQLite suite pins, run
// against the other engine: the predicate and the partial index are written
// once in portable SQL, and this is what says the translation produced them.
func TestTokenNameInUseOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	mine := seed(t, s, clk, "acme")
	theirs := seed(t, s, clk, "beta")

	putToken(t, s, theirs.scope, theirs.actor.ID, "01PGAAAAAAAAAAAAAAAAAAAAAA", "ci")
	if nameInUse(t, s, mine.scope, "ci") {
		t.Error("another tenant's token name reads as taken here")
	}

	putToken(t, s, mine.scope, mine.actor.ID, "01PGBBBBBBBBBBBBBBBBBBBBBB", "ci")
	if !nameInUse(t, s, mine.scope, "ci") {
		t.Error("this tenant's own token name does not read as taken")
	}

	if err := s.Update(ctx, mine.scope, func(tx store.Tx) error {
		return tx.RevokeToken(ctx, "01PGBBBBBBBBBBBBBBBBBBBBBB", clk.Now())
	}); err != nil {
		t.Fatalf("RevokeToken: %v", err)
	}
	if nameInUse(t, s, mine.scope, "ci") {
		t.Error("a revoked token still holds its name")
	}

	// The index has to agree with the predicate, or the service would refuse a
	// reissue the database would have accepted, or the other way round.
	putToken(t, s, mine.scope, mine.actor.ID, "01PGCCCCCCCCCCCCCCCCCCCCCC", "ci")
	err := s.Update(ctx, mine.scope, func(tx store.Tx) error {
		tok := core.APIToken{ID: "01PGDDDDDDDDDDDDDDDDDDDDDD", ActorID: mine.actor.ID,
			Name: "ci", Scopes: []core.Scope{core.ScopeTaskRead}}
		return tx.CreateToken(ctx, &tok, "hash-d")
	})
	if err == nil {
		t.Error("the database accepted a second live token named ci")
	}
}
