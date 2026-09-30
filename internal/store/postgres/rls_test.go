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

// tenantTable is one table of the live schema as pg_class and pg_attribute
// describe it, which is the only description of it that cannot be out of date.
type tenantTable struct {
	name        string
	hasTenant   bool
	rls, forced bool
	partition   bool
	parent      string
}

// tenantTables reads every ordinary and partitioned table in the schema, along
// with whether it carries a tenant column and how row-level security stands on
// it. Nothing here is taken from Go: the catalogue is the subject.
func tenantTables(t *testing.T, s *Store) []tenantTable {
	t.Helper()
	rows, err := s.db.QueryContext(context.Background(), `
		SELECT c.relname,
		       EXISTS (SELECT 1 FROM pg_attribute a
		                WHERE a.attrelid = c.oid AND a.attname = $1 AND NOT a.attisdropped),
		       c.relrowsecurity, c.relforcerowsecurity, c.relispartition,
		       COALESCE((SELECT p.relname FROM pg_inherits i
		                   JOIN pg_class p ON p.oid = i.inhparent
		                  WHERE i.inhrelid = c.oid), '')
		  FROM pg_class c
		  JOIN pg_namespace n ON n.oid = c.relnamespace
		 WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p')
		 ORDER BY c.relname`, sqlb.TenantColumn)
	if err != nil {
		t.Fatalf("reading the schema catalogue: %v", err)
	}
	defer func() { _ = rows.Close() }()

	var out []tenantTable
	for rows.Next() {
		var v tenantTable
		if err := rows.Scan(&v.name, &v.hasTenant, &v.rls, &v.forced, &v.partition, &v.parent); err != nil {
			t.Fatalf("scanning the schema catalogue: %v", err)
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("reading the schema catalogue: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("the schema catalogue is empty")
	}
	return out
}

// TestEveryTenantTableIsIsolated derives its subject from pg_attribute rather
// than from sqlb.ScopedTables(), because a guard that walks the same list the
// policies are emitted from cannot see a table dropped from that list: the
// table loses its policy and its assertion in one edit. Removing "sessions"
// from ScopedTables() used to leave the whole store tree green while sessions
// lost row-level security entirely.
//
// A table carrying a tenant column must be scoped, listed, RLS-enabled, forced,
// and carry the isolation policy. A table without one must be named in the
// builder's exemption allow-list, which records a reason for each entry.
//
// Partitions of events and audit_entries are excluded from the per-table
// assertions and checked differently. PostgreSQL applies the partitioned
// parent's policies to rows reached through the parent, which is the only way
// the store reaches them: nothing queries a partition by name. Their own
// relrowsecurity is therefore not the control that matters, and asserting it
// would assert something the engine does not use. What does matter is that the
// parent is protected, so each partition is required to descend from a table
// this same test has just held to the full standard.
func TestEveryTenantTableIsIsolated(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	exempt := sqlb.UnscopedTables()
	listed := map[string]bool{}
	for _, table := range sqlb.ScopedTables() {
		listed[table] = true
	}

	all := tenantTables(t, s)
	guarded := map[string]bool{}
	var partitions []tenantTable

	for _, v := range all {
		if v.partition {
			partitions = append(partitions, v)
			continue
		}
		reason, isExempt := exempt[v.name]

		if !v.hasTenant {
			if !isExempt {
				t.Errorf("table %q has no %s column and no entry in the unscoped allow-list",
					v.name, sqlb.TenantColumn)
			}
			if listed[v.name] {
				t.Errorf("table %q is in ScopedTables() but has no %s column to write a policy over",
					v.name, sqlb.TenantColumn)
			}
			continue
		}

		if isExempt {
			t.Errorf("table %q carries a %s column but is exempted from tenant scoping, with the reason %q",
				v.name, sqlb.TenantColumn, reason)
		}
		if !sqlb.IsScoped(v.name) {
			t.Errorf("table %q carries a %s column but IsScoped reports it unscoped", v.name, sqlb.TenantColumn)
		}
		if !listed[v.name] {
			t.Errorf("table %q carries a %s column but is absent from ScopedTables(), so no isolation policy was emitted for it",
				v.name, sqlb.TenantColumn)
		}
		if !v.rls || !v.forced {
			t.Errorf("table %q carries a %s column with row-level security enabled=%v forced=%v",
				v.name, sqlb.TenantColumn, v.rls, v.forced)
		}
		var policies int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pg_policies WHERE tablename = $1 AND policyname = $2`,
			v.name, isolationPolicy).Scan(&policies); err != nil {
			t.Fatalf("reading the policies of %q: %v", v.name, err)
		}
		if policies != 1 {
			t.Errorf("table %q has %d isolation policies, want 1", v.name, policies)
		}
		guarded[v.name] = true
	}

	for _, v := range partitions {
		if !guarded[v.parent] {
			t.Errorf("partition %q descends from %q, which this test did not hold to the isolation standard",
				v.name, v.parent)
		}
	}

	for table, reason := range exempt {
		if !hasTable(all, table) {
			t.Errorf("the unscoped allow-list exempts %q (%s), which the schema does not contain", table, reason)
		}
	}
	for table := range listed {
		if !hasTable(all, table) {
			t.Errorf("ScopedTables() names %q, which the schema does not contain", table)
		}
	}
}

func hasTable(all []tenantTable, name string) bool {
	for _, v := range all {
		if v.name == name {
			return true
		}
	}
	return false
}

// TestHostResolutionSurvivesRowLevelSecurity is the guard for the one thing the
// tenant_domains fix could break on this engine alone. tenant_domains is forced
// like every other tenant-owned table, and ResolveDomain must read it in an
// unscoped transaction where tix.tenant_id is the empty string. Without the
// SELECT-only lookup policy the read returns no rows and host-based tenant
// resolution stops, quietly, while every SQLite test keeps passing.
func TestHostResolutionSurvivesRowLevelSecurity(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)

	one := seed(t, s, clk, "resolve")
	host := "guard-" + one.scope.TenantID + ".example"
	if err := s.Update(ctx, one.scope, func(txn store.Tx) error {
		return txn.AddDomain(ctx, &core.Domain{Hostname: host})
	}); err != nil {
		t.Fatalf("adding the domain: %v", err)
	}

	var got *core.Tenant
	if err := s.Unscoped(ctx, func(txn store.UnscopedTx) error {
		v, err := txn.ResolveDomain(ctx, host)
		got = v
		return err
	}); err != nil {
		t.Fatalf("resolving %q across tenants: %v", host, err)
	}
	if got == nil || got.ID != one.scope.TenantID {
		t.Fatalf("resolved %q to %+v, want tenant %q", host, got, one.scope.TenantID)
	}
}

// TestHostResolutionSurvivesAReadOnlyTransaction is the same guard for the door
// the request path now uses. Raising the lookup flag is a set_config call, and a
// read-only transaction is the one place it could be refused as a write; if it
// were, host resolution would stop on PostgreSQL while every SQLite test kept
// passing.
func TestHostResolutionSurvivesAReadOnlyTransaction(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)

	one := seed(t, s, clk, "readonly")
	host := "readonly-" + one.scope.TenantID + ".example"
	if err := s.Update(ctx, one.scope, func(txn store.Tx) error {
		return txn.AddDomain(ctx, &core.Domain{Hostname: host})
	}); err != nil {
		t.Fatalf("adding the domain: %v", err)
	}

	var got *core.Tenant
	if err := s.ViewUnscoped(ctx, func(txn store.UnscopedTx) error {
		v, err := txn.ResolveDomain(ctx, host)
		got = v
		return err
	}); err != nil {
		t.Fatalf("resolving %q in a read-only transaction: %v", host, err)
	}
	if got == nil || got.ID != one.scope.TenantID {
		t.Fatalf("resolved %q to %+v, want tenant %q", host, got, one.scope.TenantID)
	}
}

// TestDomainLookupPolicyIsReadOnlyAndLowered holds the escape hatch to its
// stated shape: it admits a SELECT and nothing else, and it is not left raised
// for the rest of the transaction.
func TestDomainLookupPolicyIsReadOnlyAndLowered(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)

	var cmd string
	if err := s.db.QueryRowContext(ctx,
		`SELECT cmd FROM pg_policies WHERE tablename = 'tenant_domains' AND policyname = $1`,
		domainAuthPolicy).Scan(&cmd); err != nil {
		t.Fatalf("reading the domain lookup policy: %v", err)
	}
	if cmd != "SELECT" {
		t.Fatalf("the domain lookup policy applies to %q, want SELECT alone", cmd)
	}

	one := seed(t, s, clk, "lowered")
	host := "lowered-" + one.scope.TenantID + ".example"
	if err := s.Update(ctx, one.scope, func(txn store.Tx) error {
		return txn.AddDomain(ctx, &core.Domain{Hostname: host})
	}); err != nil {
		t.Fatalf("adding the domain: %v", err)
	}

	if err := s.Unscoped(ctx, func(txn store.UnscopedTx) error {
		if _, err := txn.ResolveDomain(ctx, host); err != nil {
			return err
		}
		var raised string
		if err := txn.(*tx).ex.QueryRowContext(ctx,
			`SELECT current_setting($1, true)`, domainAuthSetting).Scan(&raised); err != nil {
			t.Fatalf("reading the lookup flag: %v", err)
		}
		if raised == domainAuthOn {
			t.Fatal("the domain lookup flag is still raised after the lookup returned")
		}
		return nil
	}); err != nil {
		t.Fatalf("resolving %q: %v", host, err)
	}
}

// TestTheServerRegistryHasNoIsolationPolicy is the mirror of the guard above,
// and it is not pedantry.
//
// A server serves every tenant, so `servers` has no tenant column and there is
// no predicate an isolation policy could be written over. Somebody adding it to
// ScopedTables() in good faith would get FORCE ROW LEVEL SECURITY with a policy
// comparing a column that does not exist, or none at all, and a forced table
// with no policy is readable by nobody: the registrar could not write its own
// row and the status report would list none. That failure appears on PostgreSQL
// only, so SQLite would keep passing while the deployment went blind.
func TestTheServerRegistryHasNoIsolationPolicy(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	var enabled, forced bool
	if err := s.db.QueryRowContext(ctx,
		`SELECT relrowsecurity, relforcerowsecurity FROM pg_class WHERE relname = 'servers'`,
	).Scan(&enabled, &forced); err != nil {
		t.Fatalf("reading the security flags of servers: %v", err)
	}
	if enabled || forced {
		t.Fatalf("servers has row-level security enabled=%v forced=%v; it is installation state "+
			"with no tenant column, so a policy over it cannot be written", enabled, forced)
	}

	var policies int
	if err := s.db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM pg_policies WHERE tablename = 'servers'`).Scan(&policies); err != nil {
		t.Fatalf("reading the policies of servers: %v", err)
	}
	if policies != 0 {
		t.Fatalf("servers carries %d policies, want none", policies)
	}

	for _, table := range sqlb.ScopedTables() {
		if table == "servers" {
			t.Fatal("servers is listed as tenant-scoped; it has no tenant column to scope by")
		}
	}
}
