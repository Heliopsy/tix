// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestInvitesAreOnePendingPerUserAndStayInTheirTenant(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	b := seed(t, s, clk, "beta")
	now := clk.Now().Truncate(time.Second)

	inv := core.Invite{ID: "01INV00000000000000000000A", UserID: a.actor.ID, Email: "ada@example.com",
		Role: core.RoleMember, InvitedBy: a.actor.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateInvite(ctx, &inv, "hash-a") }); err != nil {
		t.Fatalf("creating: %v", err)
	}
	second := inv
	second.ID = "01INV00000000000000000000B"
	if err := s.Update(ctx, a.scope, func(tx store.Tx) error { return tx.CreateInvite(ctx, &second, "hash-b") }); err == nil {
		t.Fatal("a second pending invite for one user was stored")
	}

	if err := s.View(ctx, a.scope, func(tx store.Tx) error {
		got, stored, err := tx.InviteByHash(ctx, "hash-a")
		if err != nil {
			return err
		}
		if got.ID != inv.ID || stored != "hash-a" || got.TenantID != a.scope.TenantID || got.Role != core.RoleMember ||
			!got.ExpiresAt.Equal(inv.ExpiresAt) || got.AcceptedAt != nil {
			t.Fatalf("read back %+v, %q", got, stored)
		}
		pending, err := tx.ListPendingInvites(ctx)
		if err != nil || len(pending) != 1 {
			t.Fatalf("pending = %+v, %v", pending, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.View(ctx, b.scope, func(tx store.Tx) error {
		if _, _, err := tx.InviteByHash(ctx, "hash-a"); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("another tenant found the invite: %v", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if err := tx.AcceptInvite(ctx, inv.ID, now); err != nil {
			return err
		}
		if err := tx.AcceptInvite(ctx, inv.ID, now); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("accepting twice: err = %v, want not found", err)
		}
		return tx.CreateInvite(ctx, &second, "hash-b")
	}); err != nil {
		t.Fatalf("after accepting, a new invite: %v", err)
	}
}
