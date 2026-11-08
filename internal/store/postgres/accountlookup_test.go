// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// seedPersonInTwo writes one account with a member actor in each of two
// tenants, and an actor with no membership in a third.
func seedPersonInTwo(t *testing.T, s *Store, f1, f2, f3 fixture) core.User {
	t.Helper()
	ctx := context.Background()
	user := core.User{Email: "pat-" + f1.scope.TenantID + "@example.com"}
	for i, f := range []fixture{f1, f2, f3} {
		if err := s.Update(ctx, f.scope, func(tx store.Tx) error {
			if i == 0 {
				if err := tx.CreateUser(ctx, &user, ""); err != nil {
					return err
				}
			}
			actor := core.Actor{Kind: core.ActorUser, Handle: "pat", UserID: user.ID}
			if err := tx.CreateActor(ctx, &actor); err != nil {
				return err
			}
			if i == 2 {
				return nil
			}
			return tx.AddMember(ctx, &core.Membership{ActorID: actor.ID, Role: core.RoleMember})
		}); err != nil {
			t.Fatalf("seeding pat in %s: %v", f.tenant.Key, err)
		}
	}
	return user
}

// rawMembershipRows runs the unscoped membership query with no flag raised,
// which is what the isolation policies alone admit.
func rawMembershipRows(t *testing.T, tr *tx, userID string) int {
	t.Helper()
	q, args := sqlb.AccountMembershipsQuery(dialect, userID)
	rows, err := tr.ex.QueryContext(context.Background(), q, args...)
	if err != nil {
		t.Fatalf("reading memberships without the flag: %v", err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		n++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading memberships without the flag: %v", err)
	}
	return n
}

// TestAccountLookupPolicyIsNarrow pins what the membership listing rests on:
// it sees an account's memberships in every tenant, the flag that admits it
// is closed by default and does not outlive its transaction, and it widens
// reads and nothing else.
func TestAccountLookupPolicyIsNarrow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	acme, globex, initech := seed(t, s, clk, "acme"), seed(t, s, clk, "globex"), seed(t, s, clk, "initech")
	pat := seedPersonInTwo(t, s, acme, globex, initech)

	t.Run("closed by default", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawMembershipRows(t, u.(*tx), pat.ID); n != 0 {
				t.Fatalf("an unscoped read with no flag returned %d rows, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("one statement sees every tenant the account is a member of", func(t *testing.T) {
		var found []core.AccountMembership
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			var err error
			found, err = u.ListMemberships(ctx, pat.ID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if len(found) != 2 || found[0].TenantKey != "acme" || found[1].TenantKey != "globex" {
			t.Fatalf("memberships = %+v, want acme and globex and not the tenant with no membership", found)
		}
	})

	t.Run("the flag does not outlive its transaction", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			_, err := u.ListMemberships(ctx, pat.ID)
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawMembershipRows(t, u.(*tx), pat.ID); n != 0 {
				t.Fatalf("a later transaction still read %d rows, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("the flag admits no write", func(t *testing.T) {
		var affected int64
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			tr := u.(*tx)
			if err := tr.accountLookup(ctx, true); err != nil {
				return err
			}
			res, err := tr.ex.ExecContext(ctx, `UPDATE tenant_members SET role = 'admin' WHERE tenant_id = $1`,
				globex.tenant.ID)
			if err != nil {
				return err
			}
			affected, err = res.RowsAffected()
			return err
		}); err != nil {
			t.Fatalf("attempting an update under the flag: %v", err)
		}
		if affected != 0 {
			t.Fatalf("an update under the flag reached %d rows, want 0", affected)
		}
	})

	t.Run("the policies are SELECT alone", func(t *testing.T) {
		rows, err := s.db.QueryContext(ctx,
			`SELECT tablename, cmd FROM pg_policies WHERE policyname = $1 ORDER BY tablename`, accountLookupPolicy)
		if err != nil {
			t.Fatal(err)
		}
		defer func() { _ = rows.Close() }()
		var got []string
		for rows.Next() {
			var table, cmd string
			if err := rows.Scan(&table, &cmd); err != nil {
				t.Fatal(err)
			}
			got = append(got, table+":"+cmd)
		}
		if len(got) != 3 || got[0] != "actors:SELECT" || got[1] != "tenant_domains:SELECT" ||
			got[2] != "tenant_members:SELECT" {
			t.Fatalf("account lookup policies = %v, want SELECT on actors, tenant_domains and tenant_members", got)
		}
	})
}
