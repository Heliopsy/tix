// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// TestAnAccountsMembershipsAreListedAcrossTenants lists the tenants an account
// holds a membership in: not one where its actor has none, and not a deleted
// tenant.
func TestAnAccountsMembershipsAreListedAcrossTenants(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	fixtures := []fixture{seed(t, s, clk, "acme"), seed(t, s, clk, "globex"), seed(t, s, clk, "initech"), seed(t, s, clk, "gone")}
	pat := core.User{Email: "pat@example.com"}
	for i, f := range fixtures {
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
			if i == 0 {
				if err := tx.CreateUser(ctx, &pat, ""); err != nil {
					return err
				}
			}
			actor := core.Actor{Kind: core.ActorUser, Handle: "pat", UserID: pat.ID}
			if err := tx.CreateActor(ctx, &actor); err != nil {
				return err
			}
			if f.tenant.Key == "initech" {
				return nil
			}
			return tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleAdmin})
		}); err != nil {
			t.Fatalf("seeding pat in %s: %v", f.tenant.Key, err)
		}
	}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.DeleteTenant(ctx, fixtures[3].tenant.ID)
	}); err != nil {
		t.Fatal(err)
	}

	var found []core.AccountMembership
	if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		var err error
		found, err = u.ListMemberships(ctx, pat.ID)
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if len(found) != 2 || found[0].TenantKey != "acme" || found[1].TenantKey != "globex" ||
		found[0].Role != core.RoleAdmin || found[0].Handle != "pat" {
		t.Fatalf("memberships = %+v, want acme and globex as admin", found)
	}
}
