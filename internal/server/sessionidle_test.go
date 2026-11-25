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

// The idle timeout is read from the tenant on every request, so a tenant that
// turns one on ends sessions already signed in, and use recorded through the
// store keeps a session alive.
func TestTenantIdleTimeoutEndsAnUnusedSessionAtOnce(t *testing.T) {
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
	minted, err := auth.MintSession(clk, auth.SessionInput{ActorID: "x", TenantID: tenant.ID, TTL: 12 * time.Hour})
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
	verifier := auth.NewSessionVerifier(server.NewStoreCredentials(st, core.SessionPolicy{TTL: core.DefaultSessionTTL}), clk)
	clk.Advance(2 * time.Hour)
	if _, err := verifier.Verify(pinned, minted.Session.Token); err != nil {
		t.Fatalf("with no idle timeout a two-hour-old session was refused: %v", err)
	}

	tenant.SessionIdleTimeout = core.Duration(time.Hour)
	if err := st.Unscoped(ctx, func(u store.UnscopedTx) error { return u.UpdateTenant(ctx, &tenant) }); err != nil {
		t.Fatal(err)
	}
	clk.Advance(50 * time.Minute)
	if _, err := verifier.Verify(pinned, minted.Session.Token); err != nil {
		t.Fatalf("a session used 50 minutes ago was refused under a one hour idle timeout: %v", err)
	}
	clk.Advance(61 * time.Minute)
	if _, err := verifier.Verify(pinned, minted.Session.Token); !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("a session unused for 61 minutes: err = %v, want unauthenticated", err)
	}
}
