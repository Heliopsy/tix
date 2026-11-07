// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// An invite is found by id and as its user's pending one until it is
// revoked; a revoked invite cannot be revoked or accepted again.
func TestARevokedInviteIsNoLongerPending(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	a := seed(t, s, clk, "acme")
	now := clk.Now().Truncate(time.Second)
	inv := core.Invite{ID: "01INV00000000000000000000C", UserID: a.actor.ID, Email: "ada@example.com",
		Role: core.RoleMember, InvitedBy: a.actor.ID, CreatedAt: now, ExpiresAt: now.Add(time.Hour)}

	if err := s.Update(ctx, a.scope, func(tx store.Tx) error {
		if _, err := tx.PendingInviteFor(ctx, a.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("a user with no invite has a pending one: %v", err)
		}
		if err := tx.CreateInvite(ctx, &inv, "hash-c"); err != nil {
			return err
		}
		got, err := tx.GetInvite(ctx, inv.ID)
		if err != nil || got.Email != "ada@example.com" || got.UserID != a.actor.ID {
			t.Fatalf("GetInvite = %+v, %v", got, err)
		}
		pending, err := tx.PendingInviteFor(ctx, a.actor.ID)
		if err != nil || pending.ID != inv.ID {
			t.Fatalf("PendingInviteFor = %+v, %v", pending, err)
		}
		if err := tx.RevokeInvite(ctx, inv.ID, now); err != nil {
			return err
		}
		revoked, err := tx.GetInvite(ctx, inv.ID)
		if err != nil || revoked.RevokedAt == nil || !revoked.RevokedAt.Equal(now) {
			t.Errorf("after revoking, GetInvite = %+v, %v", revoked, err)
		}
		if _, err := tx.PendingInviteFor(ctx, a.actor.ID); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("a revoked invite is still pending: %v", err)
		}
		if err := tx.RevokeInvite(ctx, inv.ID, now); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("revoking twice: %v, want not found", err)
		}
		if err := tx.AcceptInvite(ctx, inv.ID, now); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("accepting a revoked invite: %v, want not found", err)
		}
		if _, err := tx.GetInvite(ctx, "01INV0000000000000000NOPE0"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("an unknown id: %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// A user's own date format and zone round-trip, empty means inherit, and an
// unknown user is not found either way.
func TestUserTimePrefsRoundTrip(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	user := seedUser(t, f, "ada@example.com")

	if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if p, err := tx.UserTimePrefs(ctx, user.ID); err != nil || p != (core.TimePrefs{}) {
			t.Errorf("a new user's prefs = %+v, %v; want empty", p, err)
		}
		want := core.TimePrefs{TimeFormat: "dotted", Timezone: "Europe/Sofia"}
		if err := tx.SetUserTimePrefs(ctx, user.ID, want); err != nil {
			return err
		}
		if p, err := tx.UserTimePrefs(ctx, user.ID); err != nil || p != want {
			t.Errorf("read back %+v, %v; want %+v", p, err, want)
		}
		if err := tx.SetUserTimePrefs(ctx, user.ID, core.TimePrefs{Timezone: "UTC"}); err != nil {
			return err
		}
		if p, _ := tx.UserTimePrefs(ctx, user.ID); p.TimeFormat != "" || p.Timezone != "UTC" {
			t.Errorf("clearing the format left %+v", p)
		}
		if _, err := tx.UserTimePrefs(ctx, "01NOBODY00000000000000000"); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("an unknown user's prefs: %v, want not found", err)
		}
		if err := tx.SetUserTimePrefs(ctx, "01NOBODY00000000000000000", core.TimePrefs{}); !core.IsKind(err, core.KindNotFound) {
			t.Errorf("setting an unknown user's prefs: %v, want not found", err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
