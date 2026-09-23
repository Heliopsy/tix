// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
	"github.com/heliopsy/tix/internal/testenv"
)

// requireAppRole skips when the database granted no unprivileged role, because
// the login role then bypasses row-level security and the policies below would
// prove nothing.
func requireAppRole(t *testing.T, s *Store) {
	t.Helper()
	if s.appRole {
		return
	}
	testenv.Skip(t, testenv.Capability{
		Name: "postgres-app-role",
		Why:  "the " + appRoleName + " role is unavailable, so the login role bypasses row-level security",
		How:  "point " + testenv.PostgresEnv + " at a database whose user may CREATE ROLE (just pg-up does)",
	})
}

// TestRowLevelSecurityRefusesACrossTenantRead issues statements with no tenant
// predicate at all, which is what a bug in the query builder would produce. The
// policies still confine the transaction to its own tenant.
func TestRowLevelSecurityRefusesACrossTenantRead(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	one := seed(t, s, clk, "one")
	two := seed(t, s, clk, "two")
	one.newTask(t, "mine", core.PriorityNormal)
	two.newTask(t, "theirs", core.PriorityNormal)
	two.newTask(t, "theirs as well", core.PriorityNormal)

	if err := s.View(ctx, one.scope, func(txn store.Tx) error {
		ex := txn.(*tx).ex
		var n int
		if err := ex.QueryRowContext(ctx, "SELECT COUNT(*) FROM tasks").Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("an unscoped count saw %d tasks, want only this tenant's 1", n)
		}
		var title string
		err := ex.QueryRowContext(ctx, "SELECT title FROM tasks WHERE tenant_id = $1", two.tenant.ID).Scan(&title)
		if err == nil {
			t.Fatalf("an explicit cross-tenant read returned %q", title)
		}
		return nil
	}); err != nil {
		t.Fatalf("bypassing the tenant predicate: %v", err)
	}
}

// TestRowLevelSecurityRefusesACrossTenantWrite proves the WITH CHECK half of the
// policies: a row that would land in another tenant is rejected outright.
func TestRowLevelSecurityRefusesACrossTenantWrite(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	one := seed(t, s, clk, "one")
	two := seed(t, s, clk, "two")

	txn, err := s.Begin(ctx, one.scope)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	_, err = txn.(*tx).ex.ExecContext(ctx,
		`INSERT INTO tags (id, tenant_id, name, color) VALUES ($1, $2, $3, '')`,
		"leaked-tag", two.tenant.ID, "smuggled")
	if err == nil {
		t.Fatal("a row was written into another tenant")
	}
	if !core.IsKind(mapErr(err, "writing across tenants"), core.KindPrecondition) {
		t.Fatalf("cross-tenant write = %v, want a precondition failure", err)
	}
	if err := txn.Rollback(); err != nil {
		t.Fatalf("rollback: %v", err)
	}

	if err := s.View(ctx, two.scope, func(tx store.Tx) error {
		tags, err := tx.ListTags(ctx)
		if err != nil {
			return err
		}
		if len(tags) != 0 {
			t.Fatalf("the other tenant gained %d tags", len(tags))
		}
		return nil
	}); err != nil {
		t.Fatalf("verifying the other tenant: %v", err)
	}
}

func TestPoliciesExistForEveryScopedTable(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	for _, table := range sqlb.ScopedTables() {
		var enabled, forced bool
		if err := s.db.QueryRowContext(ctx,
			`SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE relname = $1`,
			table).Scan(&enabled, &forced); err != nil {
			t.Fatalf("reading the security flags of %q: %v", table, err)
		}
		if !enabled || !forced {
			t.Fatalf("table %q has row-level security enabled=%v forced=%v", table, enabled, forced)
		}
		var policies int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pg_policies WHERE tablename = $1 AND policyname = $2`,
			table, isolationPolicy).Scan(&policies); err != nil {
			t.Fatalf("reading the policies of %q: %v", table, err)
		}
		if policies != 1 {
			t.Fatalf("table %q has %d isolation policies, want 1", table, policies)
		}
	}
}
