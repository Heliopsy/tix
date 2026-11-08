// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// rawCredentialRows runs the unscoped credential query with no flag raised.
func rawCredentialRows(t *testing.T, tr *tx, hash string) int {
	t.Helper()
	q, args := sqlb.CredentialTenantQuery(dialect, hash)
	rows, err := tr.ex.QueryContext(context.Background(), q, args...)
	if err != nil {
		t.Fatalf("reading a credential without the flag: %v", err)
	}
	defer func() { _ = rows.Close() }()
	n := 0
	for rows.Next() {
		n++
	}
	return n
}

// TestCredentialLookupPolicyIsNarrow pins what the shared host rests on: the
// tenant of a session or token is found by its hash from outside any tenant,
// the flag admitting it is closed by default and does not outlive its
// transaction, it widens reads and nothing else, and the credential itself
// still resolves only inside its own tenant.
func TestCredentialLookupPolicyIsNarrow(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	acme, globex := seed(t, s, clk, "acme"), seed(t, s, clk, "globex")
	if err := s.Update(ctx, globex.scope, func(tx store.Tx) error {
		if err := tx.CreateSession(ctx, globex.actor.ID, "globex-session", clk.Now().Add(time.Hour),
			core.SessionOrigin{Method: core.LoginMethodSwitch, SecondFactor: true}); err != nil {
			return err
		}
		return tx.CreateToken(ctx, &core.APIToken{ActorID: globex.actor.ID, Name: "t", Scopes: []core.Scope{core.ScopeAll}},
			"globex-token")
	}); err != nil {
		t.Fatal(err)
	}

	t.Run("closed by default", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawCredentialRows(t, u.(*tx), "globex-session"); n != 0 {
				t.Fatalf("an unscoped read with no flag returned %d rows, want 0", n)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("a session and a token name their tenant", func(t *testing.T) {
		for _, hash := range []string{"globex-session", "globex-token"} {
			var got string
			if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
				var err error
				got, err = u.CredentialTenant(ctx, hash)
				return err
			}); err != nil || got != globex.tenant.ID {
				t.Fatalf("CredentialTenant(%s) = %q, %v; want globex", hash, got, err)
			}
		}
		if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
			_, err := u.CredentialTenant(ctx, "nobody")
			return err
		}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("an unknown hash = %v, want not found", err)
		}
	})

	t.Run("the flag does not outlive its transaction", func(t *testing.T) {
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			_, err := u.CredentialTenant(ctx, "globex-session")
			return err
		}); err != nil {
			t.Fatal(err)
		}
		if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
			if n := rawCredentialRows(t, u.(*tx), "globex-session"); n != 0 {
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
			if err := tr.credentialLookup(ctx, true); err != nil {
				return err
			}
			res, err := tr.ex.ExecContext(ctx, `DELETE FROM sessions WHERE token_hash = $1`, "globex-session")
			if err != nil {
				return err
			}
			affected, err = res.RowsAffected()
			return err
		}); err != nil {
			t.Fatalf("attempting a delete under the flag: %v", err)
		}
		if affected != 0 {
			t.Fatalf("a delete under the flag reached %d rows, want 0", affected)
		}
	})

	t.Run("the credential still resolves only in its own tenant", func(t *testing.T) {
		if err := s.View(ctx, acme.scope, func(tx store.Tx) error {
			_, _, err := tx.GetSessionByHash(ctx, "globex-session")
			return err
		}); !core.IsKind(err, core.KindNotFound) {
			t.Fatalf("globex's session read in acme = %v, want not found", err)
		}
		var origin core.SessionOrigin
		if err := s.View(ctx, globex.scope, func(tx store.Tx) error {
			var err error
			origin, err = tx.GetSessionOrigin(ctx, "globex-session")
			return err
		}); err != nil || origin.Method != core.LoginMethodSwitch || !origin.SecondFactor {
			t.Fatalf("globex's session origin = %+v, %v; want switch with a second factor", origin, err)
		}
	})

	t.Run("the policies are SELECT alone", func(t *testing.T) {
		rows, err := s.db.QueryContext(ctx,
			`SELECT tablename, cmd FROM pg_policies WHERE policyname = $1 ORDER BY tablename`, credentialLookupPolicy)
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
		if len(got) != 2 || got[0] != "api_tokens:SELECT" || got[1] != "sessions:SELECT" {
			t.Fatalf("credential lookup policies = %v, want SELECT on api_tokens and sessions", got)
		}
	})
}

// TestSharedHostLookupsOnPostgres reads a host's shared flag, a tenant's own
// address and a membership's address through the policies that admit them.
func TestSharedHostLookupsOnPostgres(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	acme, globex, initech := seed(t, s, clk, "acme"), seed(t, s, clk, "globex"), seed(t, s, clk, "initech")
	pat := seedPersonInTwo(t, s, acme, globex, initech)
	if err := s.Update(ctx, acme.scope, func(tx store.Tx) error {
		if err := tx.AddDomain(ctx, &core.Domain{Hostname: "acme.example.test"}); err != nil {
			return err
		}
		return tx.SetDomainShared(ctx, "acme.example.test", true)
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Update(ctx, globex.scope, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &core.Domain{Hostname: "globex.example.test"})
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.ViewUnscoped(ctx, func(u store.UnscopedTx) error {
		tenant, shared, err := u.ResolveHost(ctx, "acme.example.test")
		if err != nil || tenant.ID != acme.tenant.ID || !shared {
			t.Errorf("ResolveHost(acme) = %+v, %v, %v; want acme, shared", tenant, shared, err)
		}
		if _, shared, err = u.ResolveHost(ctx, "globex.example.test"); err != nil || shared {
			t.Errorf("ResolveHost(globex) shared = %v, %v; want bound", shared, err)
		}
		for id, want := range map[string]string{acme.tenant.ID: "", globex.tenant.ID: "globex.example.test"} {
			if got, err := u.TenantAddress(ctx, id); err != nil || got != want {
				t.Errorf("TenantAddress(%s) = %q, %v; want %q", id, got, err, want)
			}
		}
		found, err := u.ListMemberships(ctx, pat.ID)
		if err != nil || len(found) != 2 || found[0].Host != "" || found[1].Host != "globex.example.test" {
			t.Errorf("memberships = %+v, %v; want acme with no address and globex at its own", found, err)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}
