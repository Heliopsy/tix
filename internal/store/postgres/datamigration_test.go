// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/migrations"
)

// restrictedDB is a database owned by a login role that is neither a
// superuser nor BYPASSRLS, which is what production gives tix and what the
// test container's own role is not. admin reaches the same database as the
// superuser, for arranging and reading rows past the policies.
type restrictedDB struct {
	dsn   string
	admin *sql.DB
	role  string
}

// newRestrictedDB creates a role and a database it owns, both dropped when the
// test ends. The role may not create roles either, so it never gets tix_app
// and every transaction runs as the table owner under FORCE ROW LEVEL SECURITY.
func newRestrictedDB(t *testing.T) restrictedDB {
	t.Helper()
	base := requireDSN(t)
	ctx := context.Background()
	n := dbCounter.Add(1)
	role := fmt.Sprintf("tix_owner_%d_%d", os.Getpid(), n)
	name := fmt.Sprintf("tix_test_owner_%d_%d", os.Getpid(), n)

	root, err := sql.Open(driverName, base)
	if err != nil {
		t.Fatalf("opening the admin connection: %v", err)
	}
	defer func() { _ = root.Close() }()
	for _, stmt := range []string{
		"DROP DATABASE IF EXISTS " + name + " WITH (FORCE)",
		"DROP ROLE IF EXISTS " + role,
		"CREATE ROLE " + role + " LOGIN NOSUPERUSER NOBYPASSRLS NOCREATEROLE PASSWORD 'owner'",
		"CREATE DATABASE " + name + " OWNER " + role,
	} {
		if _, err := root.ExecContext(ctx, stmt); err != nil {
			t.Fatalf("%s: %v", stmt, err)
		}
	}
	t.Cleanup(func() {
		cleaner, err := sql.Open(driverName, base)
		if err != nil {
			return
		}
		defer func() { _ = cleaner.Close() }()
		_, _ = cleaner.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
		_, _ = cleaner.ExecContext(context.Background(), "DROP ROLE IF EXISTS "+role)
	})

	adminDSN := replaceDatabase(base, name)
	admin, err := sql.Open(driverName, adminDSN)
	if err != nil {
		t.Fatalf("opening the admin connection to %s: %v", name, err)
	}
	t.Cleanup(func() { _ = admin.Close() })
	u, err := url.Parse(adminDSN)
	if err != nil {
		t.Fatalf("parsing the dsn: %v", err)
	}
	u.User = url.UserPassword(role, "owner")
	return restrictedDB{dsn: u.String(), admin: admin, role: role}
}

// requireRestricted fails unless the store's login role is subject to the
// policies, so the test cannot pass by running as a role that bypasses them.
func requireRestricted(t *testing.T, s *Store) {
	t.Helper()
	var super, bypass bool
	if err := s.db.QueryRowContext(context.Background(),
		`SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname = current_user`).Scan(&super, &bypass); err != nil {
		t.Fatalf("reading the login role: %v", err)
	}
	if super || bypass {
		t.Fatalf("the login role is superuser=%v bypassrls=%v; the test needs one the policies apply to", super, bypass)
	}
}

// legacyPerson is an account as 0.45 wrote it: a users row and an actor of the
// same identifier, with nothing in actors.user_id.
type legacyPerson struct {
	email, account string
	role           core.Role
	globalAdmin    bool
}

// TestAnUpgradeFromBefore046LinksEveryAccountUnderRowLevelSecurity is the
// production failure: on a NOBYPASSRLS owner role the actor_user_link and
// actor_display_name_from_account backfills applied to no rows, so every
// account created before 0.46.0 could not sign in, was missing from the user
// list and lost its global admin standing. It rewinds to 0034, writes the rows
// as 0.45 had them, migrates forward through the real runner, and uses them.
func TestAnUpgradeFromBefore046LinksEveryAccountUnderRowLevelSecurity(t *testing.T) {
	ctx := context.Background()
	db := newRestrictedDB(t)
	clk := clock.NewFakeAt()
	s := openStore(t, db.dsn, clk)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	requireRestricted(t, s)
	if s.appRole {
		t.Fatal("the restricted role entered tix_app; the test means to run as the owner")
	}

	hasher := auth.NewHasherWithParams(auth.TestParams())
	hash, err := hasher.Hash(t.Context(), "correct horse battery")
	if err != nil {
		t.Fatal(err)
	}
	tenant := core.Tenant{Key: "acme", Name: "Acme"}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error { return u.CreateTenant(ctx, &tenant) }); err != nil {
		t.Fatalf("creating the tenant: %v", err)
	}
	scope := core.TenantScope{TenantID: tenant.ID}
	people := []legacyPerson{
		{email: "root@example.com", account: "01JROOT0000000000000000000", role: core.RoleAdmin, globalAdmin: true},
		{email: "ann@example.com", account: "01JANN00000000000000000000", role: core.RoleMember},
	}
	if err := s.Update(ctx, scope, func(tx store.Tx) error {
		for _, p := range people {
			if err := tx.CreateUser(ctx, &core.User{ID: p.account, Email: p.email, DisplayName: "Account " + p.email}, hash); err != nil {
				return err
			}
			a := &core.Actor{ID: p.account, UserID: p.account, Kind: core.ActorUser, Handle: p.email[:3], DisplayName: "Old name"}
			if err := tx.CreateActor(ctx, a); err != nil {
				return err
			}
			if err := tx.AddMember(ctx, &core.Membership{ActorID: a.ID, Role: p.role}); err != nil {
				return err
			}
			if p.globalAdmin {
				if err := tx.SetUserGlobalAdmin(ctx, p.account, true); err != nil {
					return err
				}
			}
		}
		return nil
	}); err != nil {
		t.Fatalf("seeding the accounts: %v", err)
	}

	rewindTo(t, s, 34)
	if _, err := db.admin.ExecContext(ctx, `UPDATE actors SET user_id = NULL WHERE kind = 'user'`); err != nil {
		t.Fatalf("unlinking the actors as 0.45 left them: %v", err)
	}

	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating forward from 0034: %v", err)
	}
	if latest, err := s.SchemaVersion(ctx); err != nil || latest != lastVersion(t) {
		t.Fatalf("schema version = %d, %v; want %d", latest, err, lastVersion(t))
	}

	for _, p := range people {
		var linked sql.NullString
		var name string
		if err := db.admin.QueryRowContext(ctx, `SELECT user_id, display_name FROM actors WHERE id = $1`, p.account).
			Scan(&linked, &name); err != nil {
			t.Fatalf("reading the actor of %s: %v", p.email, err)
		}
		if linked.String != p.account {
			t.Errorf("actor of %s links %q after the upgrade, want its account %q", p.email, linked.String, p.account)
		}
		if want := "Account " + p.email; name != want {
			t.Errorf("actor of %s is called %q after the upgrade, want the account's %q", p.email, name, want)
		}
	}

	l := service.New(s, service.WithClock(clk), service.WithHasher(hasher))
	actors := map[string]*core.Actor{}
	for _, p := range people {
		res, err := l.Login(core.WithTenant(ctx, scope), core.LoginInput{
			Email: p.email, Password: "correct horse battery", Surface: core.SurfaceWeb,
		})
		if err != nil || res.Session == nil {
			t.Fatalf("signing in %s after the upgrade = %+v, %v; want a session", p.email, res, err)
		}
		actors[p.email] = &core.Actor{ID: res.Session.ActorID, TenantID: tenant.ID, Kind: core.ActorUser,
			Role: p.role, Scopes: []core.Scope{core.ScopeAll}}
	}

	root := core.WithActor(ctx, actors["root@example.com"])
	listed, _, err := l.ListUsers(root, core.UserFilter{Page: core.Page{Limit: 50}})
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	var emails []string
	for _, u := range listed {
		emails = append(emails, u.Email)
	}
	for _, p := range people {
		if !slices.Contains(emails, p.email) {
			t.Errorf("ListUsers = %v after the upgrade, want %s listed", emails, p.email)
		}
	}

	who, err := l.WhoAmI(root)
	if err != nil || !who.GlobalAdmin {
		t.Fatalf("WhoAmI of the global admin after the upgrade = %+v, %v; want global admin standing", who, err)
	}
	if who, err := l.WhoAmI(core.WithActor(ctx, actors["ann@example.com"])); err != nil || who.GlobalAdmin {
		t.Fatalf("WhoAmI of a member = %+v, %v; want no global admin standing", who, err)
	}
}

func lastVersion(t *testing.T) int {
	t.Helper()
	all, err := migrations.All()
	if err != nil {
		t.Fatal(err)
	}
	return all[len(all)-1].Version
}

// TestEveryForcedTableAdmitsTheMigrationRunner reads the live catalogue, not
// sqlb.ScopedTables(), for the same reason TestEveryTenantTableIsIsolated does:
// a table that is forced but carries no migration policy is one a future data
// migration would silently skip on PostgreSQL alone.
func TestEveryForcedTableAdmitsTheMigrationRunner(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)
	forced := 0
	for _, v := range tenantTables(t, s) {
		if v.partition || !v.forced {
			continue
		}
		forced++
		var cmd, using, check string
		err := s.db.QueryRowContext(ctx,
			`SELECT cmd, qual, with_check FROM pg_policies WHERE tablename = $1 AND policyname = $2`,
			v.name, migratingPolicy).Scan(&cmd, &using, &check)
		if err != nil {
			t.Errorf("forced table %q has no %s policy: %v", v.name, migratingPolicy, err)
			continue
		}
		for _, expr := range []string{using, check} {
			if cmd != "ALL" || !strings.Contains(expr, migratingSetting) || !strings.Contains(expr, appRoleName) {
				t.Errorf("table %q: %s policy is %s USING %q CHECK %q; want ALL, gated on %s and refusing %s",
					v.name, migratingPolicy, cmd, using, check, migratingSetting, appRoleName)
			}
		}
	}
	if forced == 0 {
		t.Fatal("no forced table in the catalogue")
	}
}

// TestADataStepReachesEveryTenantAsARestrictedOwner is the generic form of the
// regression above: any data statement a migration carries, run by the real
// runner as an owner the policies apply to, reaches every tenant's rows, as
// it does on SQLite. Before the migration flag it affected none.
func TestADataStepReachesEveryTenantAsARestrictedOwner(t *testing.T) {
	ctx := context.Background()
	db := newRestrictedDB(t)
	clk := clock.NewFakeAt()
	s := openStore(t, db.dsn, clk)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	requireRestricted(t, s)
	seed(t, s, clk, "one")
	seed(t, s, clk, "two")

	probe := migrations.Migration{Version: lastVersion(t) + 1, Name: "probe", SQL: `
UPDATE projects SET name = 'probed';
INSERT INTO tags (id, tenant_id, name, color) SELECT 'tag-' || id, id, 'probe', '' FROM tenants;
DELETE FROM actors WHERE kind = 'agent';`}
	if err := applyMigration(ctx, s.db, probe, clk); err != nil {
		t.Fatalf("applying the probe migration: %v", err)
	}
	for _, q := range []struct{ what, sql string }{
		{"renamed projects", `SELECT COUNT(*) FROM projects WHERE name = 'probed'`},
		{"inserted tags", `SELECT COUNT(*) FROM tags WHERE name = 'probe'`},
	} {
		var n int
		if err := db.admin.QueryRowContext(ctx, q.sql).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 2 {
			t.Errorf("%s = %d after the probe, want one in each of the 2 tenants", q.what, n)
		}
	}
	var agents int
	if err := db.admin.QueryRowContext(ctx, `SELECT COUNT(*) FROM actors WHERE kind = 'agent'`).Scan(&agents); err != nil {
		t.Fatal(err)
	}
	if agents != 0 {
		t.Errorf("%d agents survived the probe's DELETE, want 0", agents)
	}
	var left sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT current_setting($1, true)`, migratingSetting).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left.String == migratingOn {
		t.Errorf("%s is still %q after the migration committed; it must be transaction-local", migratingSetting, left.String)
	}
}

// TestTheMigrationFlagWidensNoRuntimeTransaction sets the flag inside a
// tenant's own transaction, which nothing outside the runner does, and still
// sees only that tenant: the policy refuses tix_app.
func TestTheMigrationFlagWidensNoRuntimeTransaction(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	requireAppRole(t, s)
	one := seed(t, s, clk, "one")
	seed(t, s, clk, "two")
	if err := s.View(ctx, one.scope, func(txn store.Tx) error {
		ex := txn.(*tx).ex
		if _, err := ex.ExecContext(ctx, `SELECT set_config($1, $2, true)`, migratingSetting, migratingOn); err != nil {
			return err
		}
		var n int
		if err := ex.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects`).Scan(&n); err != nil {
			return err
		}
		if n != 1 {
			t.Fatalf("a runtime transaction with %s set saw %d projects, want its own 1", migratingSetting, n)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

// TestOnlyTheRunnerSetsTheMigrationFlag keeps the flag out of every runtime
// path: no source outside schema.go names it.
func TestOnlyTheRunnerSetsTheMigrationFlag(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			if filepath.Base(path) == "schema.go" && filepath.Base(filepath.Dir(path)) == "postgres" {
				return nil
			}
			body, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			if strings.Contains(string(body), migratingSetting) || strings.Contains(string(body), "migratingSetting") {
				t.Errorf("%s names %s; only the migration runner in schema.go may set it", path, migratingSetting)
			}
			return nil
		})
		if err != nil {
			t.Fatalf("walking %s: %v", dir, err)
		}
	}
}

// TestRepairTaskNumberCountersReachesEveryTenantAsARestrictedOwner runs the
// counter repair the way production does: as an owner the forced policies
// apply to, with no tenant set. Without the migration flag it would update no
// project and leave every imported tenant refusing new tasks.
func TestRepairTaskNumberCountersReachesEveryTenantAsARestrictedOwner(t *testing.T) {
	ctx := context.Background()
	db := newRestrictedDB(t)
	clk := clock.NewFakeAt()
	s := openStore(t, db.dsn, clk)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	requireRestricted(t, s)
	want := map[string]int64{}
	for key, highest := range map[string]int64{"one": 95, "two": 868} {
		f := seed(t, s, clk, key)
		f.numberedTask(t, f.project.ID, highest, true)
		want[f.project.ID] = highest
	}

	rewindTo(t, s, migrationNamed(t, "repair_task_number_counters")-1)
	if _, err := db.admin.ExecContext(ctx, `UPDATE projects SET seq_counter = 0`); err != nil {
		t.Fatalf("winding the counters back as an import left them: %v", err)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrating forward: %v", err)
	}
	for id, highest := range want {
		if got := seqCounter(t, db.admin, id); got != highest {
			t.Errorf("counter of project %s after the repair = %d, want %d", id, got, highest)
		}
	}
}
