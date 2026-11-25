// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

// A session in use is renewed through the store by the window its kind gets
// from the tenant's policy, up to that kind's maximum lifetime.
func TestUseRenewsASessionByItsKindUpToItsLimit(t *testing.T) {
	policy := core.SessionPolicy{TTL: core.Duration(time.Hour), RememberTTL: core.Duration(4 * time.Hour),
		MaxLifetime: core.Duration(3 * time.Hour), RememberMaxLifetime: core.Duration(10 * time.Hour)}
	tests := []struct {
		name       string
		remembered bool
		every      time.Duration
		alive      time.Duration
		limit      time.Duration
	}{
		{"ordinary", false, 50 * time.Minute, 150 * time.Minute, 3 * time.Hour},
		{"remembered", true, 3 * time.Hour, 9 * time.Hour, 10 * time.Hour},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			clk := clock.NewFakeAt()
			st, err := sqlite.Open(sqlitetest.Path(t), clk)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = st.Close() })
			if err := st.Migrate(ctx); err != nil {
				t.Fatal(err)
			}
			tenant := core.Tenant{Key: "acme", Name: "Acme"}
			if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
				t.Fatal(err)
			}
			scope := core.TenantScope{TenantID: tenant.ID}
			window, _ := policy.Renewal(tt.remembered)
			minted, err := auth.MintSession(clk, auth.SessionInput{ActorID: "x", TenantID: tenant.ID, TTL: window.D()})
			if err != nil {
				t.Fatal(err)
			}
			if err := st.Update(ctx, scope, func(tx store.Tx) error {
				actor := &core.Actor{Kind: core.ActorUser, Handle: "pat"}
				if err := tx.CreateActor(ctx, actor); err != nil {
					return err
				}
				if err := tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleMember}); err != nil {
					return err
				}
				return tx.CreateSession(ctx, actor.ID, minted.Stored.TokenHash, minted.Session.ExpiresAt,
					core.SessionOrigin{Remembered: tt.remembered})
			}); err != nil {
				t.Fatal(err)
			}

			pinned := core.WithTenant(ctx, scope)
			verifier := auth.NewSessionVerifier(server.NewStoreCredentials(st, policy), clk)
			var last *core.Actor
			for used := tt.every; used <= tt.alive; used += tt.every {
				clk.Advance(tt.every)
				if last, err = verifier.Verify(pinned, minted.Session.Token); err != nil {
					t.Fatalf("a session in use was refused %v after sign-in: %v", used, err)
				}
			}
			if want := minted.Stored.CreatedAt.Add(tt.limit); last.Term == nil || !last.Term.ExpiresAt.Equal(want) ||
				last.Term.Remembered != tt.remembered {
				t.Errorf("term = %+v; want it capped at %v, remembered %v", last.Term, want, tt.remembered)
			}
			clk.Advance(tt.limit - tt.alive + time.Minute)
			if _, err := verifier.Verify(pinned, minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
				t.Fatalf("a session past its maximum lifetime: err = %v, want unauthenticated", err)
			}
		})
	}
}

// A tenant that shortened its session lifetime renews by its own, shorter,
// window, and a server that turned renewal off leaves the expiry fixed.
func TestTenantLifetimeIsTheWindowAndZeroLimitIsNoRenewal(t *testing.T) {
	ctx := context.Background()
	clk := clock.NewFakeAt()
	st, err := sqlite.Open(sqlitetest.Path(t), clk)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatal(err)
	}
	tenant := core.Tenant{Key: "acme", Name: "Acme", SessionTTL: core.Duration(30 * time.Minute)}
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
		t.Fatal(err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	minted, err := auth.MintSession(clk, auth.SessionInput{ActorID: "x", TenantID: tenant.ID, TTL: 30 * time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Update(ctx, scope, func(tx store.Tx) error {
		actor := &core.Actor{Kind: core.ActorUser, Handle: "pat"}
		if err := tx.CreateActor(ctx, actor); err != nil {
			return err
		}
		if err := tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleMember}); err != nil {
			return err
		}
		return tx.CreateSession(ctx, actor.ID, minted.Stored.TokenHash, minted.Session.ExpiresAt, core.SessionOrigin{})
	}); err != nil {
		t.Fatal(err)
	}
	pinned := core.WithTenant(ctx, scope)
	server12h := core.SessionPolicy{TTL: core.DefaultSessionTTL, MaxLifetime: core.DefaultSessionMaxLifetime}
	clk.Advance(10 * time.Minute)
	actor, err := auth.NewSessionVerifier(server.NewStoreCredentials(st, server12h), clk).Verify(pinned, minted.Session.Token)
	if err != nil {
		t.Fatal(err)
	}
	if want := clk.Now().Add(30 * time.Minute); !actor.Term.ExpiresAt.Equal(want) {
		t.Errorf("renewed to %v, want the tenant's 30m window, %v", actor.Term.ExpiresAt, want)
	}

	fixed := auth.NewSessionVerifier(server.NewStoreCredentials(st, core.SessionPolicy{TTL: core.DefaultSessionTTL}), clk)
	clk.Advance(25 * time.Minute)
	if actor, err = fixed.Verify(pinned, minted.Session.Token); err != nil || actor.Term.Renewed {
		t.Fatalf("with no maximum lifetime: term %+v, err %v; want accepted and not renewed", actor.Term, err)
	}
	clk.Advance(20 * time.Minute)
	if _, err := fixed.Verify(pinned, minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("with renewal off a session past its expiry: err = %v, want unauthenticated", err)
	}
}
