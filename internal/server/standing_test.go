// SPDX-License-Identifier: AGPL-3.0-or-later

package server_test

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/store/sqlite/sqlitetest"
)

// A disabled account has sessions and tokens in every tenant it belongs to,
// and a transaction can revoke them in one tenant only, so the lookups behind
// every request read the disabled state themselves. Disabling the actor in
// its tenant alone has the same effect there.
func TestCredentialsOfADisabledPersonStopAtOnce(t *testing.T) {
	cases := []struct {
		name    string
		disable func(ctx context.Context, tx store.Tx, user *core.User, actor *core.Actor) error
	}{
		{"account disabled", func(ctx context.Context, tx store.Tx, user *core.User, _ *core.Actor) error {
			now := time.Now()
			user.DisabledAt = &now
			return tx.UpdateUser(ctx, user, "")
		}},
		{"actor disabled in its tenant", func(ctx context.Context, tx store.Tx, _ *core.User, actor *core.Actor) error {
			now := time.Now()
			actor.DisabledAt = &now
			return tx.UpdateActor(ctx, actor)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			st, err := sqlite.Open(sqlitetest.Path(t), clock.NewFakeAt())
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
			user := &core.User{Email: "pat@example.com"}
			actor := &core.Actor{Kind: core.ActorUser, Handle: "pat"}
			token := &core.APIToken{Name: "pat", Scopes: []core.Scope{core.ScopeTaskRead}}
			if err := st.Update(ctx, scope, func(tx store.Tx) error {
				if err := tx.CreateUser(ctx, user, ""); err != nil {
					return err
				}
				actor.UserID = user.ID
				if err := tx.CreateActor(ctx, actor); err != nil {
					return err
				}
				if err := tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleMember}); err != nil {
					return err
				}
				if err := tx.CreateSession(ctx, actor.ID, "session-hash", time.Now().Add(time.Hour), core.SessionOrigin{}); err != nil {
					return err
				}
				token.ActorID = actor.ID
				return tx.CreateToken(ctx, token, "token-hash")
			}); err != nil {
				t.Fatal(err)
			}

			creds := server.NewStoreCredentials(st)
			pinned := core.WithTenant(ctx, scope)
			if s, err := creds.SessionByHash(pinned, "session-hash"); err != nil || s == nil {
				t.Fatalf("session before disabling = %+v, %v; want it found", s, err)
			}
			if err := st.Update(ctx, scope, func(tx store.Tx) error { return tc.disable(ctx, tx, user, actor) }); err != nil {
				t.Fatal(err)
			}
			if s, err := creds.SessionByHash(pinned, "session-hash"); err != nil || s != nil {
				t.Errorf("session after disabling = %+v, %v; want none", s, err)
			}
			if tok, err := creds.TokenByHash(pinned, "token-hash"); err != nil || tok != nil {
				t.Errorf("token after disabling = %+v, %v; want none", tok, err)
			}
		})
	}
}
