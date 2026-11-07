// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestUpgradeLinksEachUserActorToItsAccount writes actors the way every
// version before the link did, rewinds to the last numbered migration, and
// migrates again, which is what an upgrade of an existing database does.
func TestUpgradeLinksEachUserActorToItsAccount(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	person := core.User{Email: "ada@example.com"}
	legacy := core.Actor{Kind: core.ActorUser, Handle: "ada"}
	placeholder := core.Actor{Kind: core.ActorUser, Handle: "imported-bob"}
	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &person, ""); err != nil {
			return err
		}
		legacy.ID = person.ID
		if err := tx.CreateActor(ctx, &legacy); err != nil {
			return err
		}
		return tx.CreateActor(ctx, &placeholder)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}

	rewindTo(t, s, 34)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}

	read := func(id string) core.Actor {
		t.Helper()
		var a *core.Actor
		if err := s.View(ctx, f.scope, func(tx store.Tx) error {
			var err error
			a, err = tx.GetActor(ctx, id)
			return err
		}); err != nil {
			t.Fatalf("reading actor %q: %v", id, err)
		}
		return *a
	}
	if got := read(legacy.ID); got.UserID != person.ID {
		t.Errorf("legacy user actor links to %q, want its account %q", got.UserID, person.ID)
	}
	if got := read(placeholder.ID); got.UserID != "" {
		t.Errorf("a user actor with no account links to %q, want nothing", got.UserID)
	}
	if got := read(f.actor.ID); got.UserID != "" {
		t.Errorf("an agent links to %q, want nothing", got.UserID)
	}

	if err := s.View(ctx, f.scope, func(tx store.Tx) error {
		a, err := tx.GetActorByUser(ctx, person.ID)
		if err == nil && a.ID != legacy.ID {
			t.Errorf("the account's actor is %q, want %q", a.ID, legacy.ID)
		}
		return err
	}); err != nil {
		t.Fatalf("GetActorByUser: %v", err)
	}

	second := core.Actor{Kind: core.ActorUser, Handle: "ada-again", UserID: person.ID}
	err := s.Update(ctx, f.scope, func(tx store.Tx) error { return tx.CreateActor(ctx, &second) })
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a second actor for one account in one tenant = %v, want a conflict", err)
	}
}

// TestAnAccountsActorIsFoundOnlyInItsTenant keeps the lookup by account inside
// the tenant boundary: the same account with an actor in one tenant is not
// found from another.
func TestAnAccountsActorIsFoundOnlyInItsTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "other")

	person := core.User{Email: "ada@example.com"}
	actor := core.Actor{Kind: core.ActorUser, Handle: "ada"}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if err := tx.CreateUser(ctx, &person, ""); err != nil {
			return err
		}
		actor.UserID = person.ID
		return tx.CreateActor(ctx, &actor)
	}); err != nil {
		t.Fatalf("seeding: %v", err)
	}
	err := s.View(ctx, b.scope, func(tx store.Tx) error {
		_, err := tx.GetActorByUser(ctx, person.ID)
		return err
	})
	if !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("looking the account up from another tenant = %v, want not found", err)
	}
}
