package migrations

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	_ "modernc.org/sqlite"
)

func open(t *testing.T) *sql.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "tix.db") + "?_pragma=foreign_keys(1)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestAllMigrationsAreWellFormed(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatalf("All() error = %v", err)
	}
	if len(all) == 0 {
		t.Fatal("no migrations found")
	}
	for i, m := range all {
		if m.Version <= 0 {
			t.Errorf("migration %d has version %d", i, m.Version)
		}
		if strings.TrimSpace(m.SQL) == "" {
			t.Errorf("migration %d is empty", m.Version)
		}
		if i > 0 && all[i-1].Version >= m.Version {
			t.Errorf("migrations are not in ascending order at index %d", i)
		}
	}
}

func TestRunAppliesSchema(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	applied, err := Run(ctx, db)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if applied == 0 {
		t.Fatal("Run() applied no migrations")
	}

	latest, err := Latest()
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	got, err := Current(ctx, db)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got != latest {
		t.Errorf("schema version = %d, want %d", got, latest)
	}
}

func TestRunIsIdempotent(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("first Run() error = %v", err)
	}
	applied, err := Run(ctx, db)
	if err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	if applied != 0 {
		t.Errorf("second Run() applied %d migrations, want 0", applied)
	}
}

// Every table the query builder treats as tenant-scoped must actually have the
// column, or the builder would generate SQL referencing a column that does not
// exist.
func TestScopedTablesHaveTenantColumn(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	scoped := []string{
		"actors", "sessions", "api_tokens", "workflows", "projects", "field_defs",
		"tasks", "task_deps", "tags", "task_tags", "comments", "artifacts",
		"events", "audit_entries", "webhook_endpoints", "webhook_deliveries",
		"retention_policies", "external_refs", "sync_sources", "tenant_members",
	}
	for _, table := range scoped {
		var n int
		err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM pragma_table_info(?) WHERE name = 'tenant_id'`, table,
		).Scan(&n)
		if err != nil {
			t.Fatalf("inspecting %q: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %q has no tenant_id column", table)
		}
	}
}

func TestExpectedTablesExist(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	want := []string{
		"tenants", "tenant_domains", "tenant_members", "actors", "users", "sessions",
		"api_tokens", "workflows", "projects", "field_defs", "tasks", "task_deps",
		"tags", "task_tags", "comments", "artifacts", "events", "audit_entries",
		"webhook_endpoints", "webhook_deliveries", "retention_policies",
		"external_refs", "sync_sources", "schema_migrations",
	}
	for _, table := range want {
		var n int
		if err := db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&n); err != nil {
			t.Fatalf("checking %q: %v", table, err)
		}
		if n != 1 {
			t.Errorf("table %q was not created", table)
		}
	}
}

func TestForeignKeysAreEnforced(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	_, err := db.ExecContext(ctx,
		`INSERT INTO tenant_domains (id, tenant_id, hostname, cert_mode, created_at)
		 VALUES ('d1', 'does-not-exist', 'example.com', 'none', '2026-01-01T00:00:00Z')`)
	if err == nil {
		t.Error("inserting a domain for a missing tenant should violate the foreign key")
	}
}

func TestTaskConstraints(t *testing.T) {
	db := open(t)
	ctx := context.Background()
	if _, err := Run(ctx, db); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	seed(t, db)

	t.Run("priority range", func(t *testing.T) {
		_, err := db.ExecContext(ctx, insertTask, "bad", "t1", "p1", 99, "todo", 9, "a1")
		if err == nil {
			t.Error("a priority outside 1-5 should be rejected")
		}
	})

	t.Run("seq unique per project", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, insertTask, "k1", "t1", "p1", 1, "todo", 3, "a1"); err != nil {
			t.Fatalf("first insert: %v", err)
		}
		if _, err := db.ExecContext(ctx, insertTask, "k2", "t1", "p1", 1, "todo", 3, "a1"); err == nil {
			t.Error("two tasks in one project must not share a sequence number")
		}
	})

	t.Run("dependency on self rejected", func(t *testing.T) {
		if _, err := db.ExecContext(ctx, insertTask, "s1", "t1", "p1", 20, "todo", 3, "a1"); err != nil {
			t.Fatalf("insert: %v", err)
		}
		_, err := db.ExecContext(ctx,
			`INSERT INTO task_deps (tenant_id, task_id, depends_on, created_at)
			 VALUES ('t1','s1','s1','2026-01-01T00:00:00Z')`)
		if err == nil {
			t.Error("a task depending on itself should be rejected")
		}
	})
}

const insertTask = `
INSERT INTO tasks (id, tenant_id, project_id, seq, status, priority, creator_actor_id,
                   title, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, 'title', '2026-01-01T00:00:00Z', '2026-01-01T00:00:00Z')`

func seed(t *testing.T, db *sql.DB) {
	t.Helper()
	ctx := context.Background()
	stmts := []string{
		`INSERT INTO tenants (id, key, name, created_at, updated_at)
		 VALUES ('t1','acme','Acme','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`,
		`INSERT INTO actors (id, tenant_id, kind, handle, created_at)
		 VALUES ('a1','t1','user','alice','2026-01-01T00:00:00Z')`,
		`INSERT INTO workflows (id, tenant_id, key, name, definition, created_at, updated_at)
		 VALUES ('w1','t1','default','Default','{}','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`,
		`INSERT INTO projects (id, tenant_id, key, name, workflow_id, created_at, updated_at)
		 VALUES ('p1','t1','infra','Infra','w1','2026-01-01T00:00:00Z','2026-01-01T00:00:00Z')`,
	}
	for _, s := range stmts {
		if _, err := db.ExecContext(ctx, s); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s)
		}
	}
}

func TestSplit(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want int
	}{
		{"two statements", "CREATE TABLE a (x TEXT); CREATE TABLE b (y TEXT);", 2},
		{"trailing statement without semicolon", "SELECT 1; SELECT 2", 2},
		{"comments are stripped", "-- a comment\nSELECT 1;", 1},
		{"semicolon inside a string literal", "INSERT INTO a VALUES ('x;y');", 1},
		{"empty input", "", 0},
		{"only comments", "-- nothing here\n-- still nothing\n", 0},
		{"blank statements ignored", "SELECT 1;;;SELECT 2;", 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Split(tt.in); len(got) != tt.want {
				t.Errorf("Split() = %d statements %q, want %d", len(got), got, tt.want)
			}
		})
	}
}

func TestAllFromRejectsMalformedNames(t *testing.T) {
	tests := []struct {
		name  string
		files fstest.MapFS
	}{
		{"no version prefix", fstest.MapFS{"init.sql": &fstest.MapFile{Data: []byte("SELECT 1;")}}},
		{"non-numeric version", fstest.MapFS{"abc_init.sql": &fstest.MapFile{Data: []byte("SELECT 1;")}}},
		{
			"duplicate version",
			fstest.MapFS{
				"0001_a.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
				"0001_b.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := allFrom(tt.files); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

func TestAllFromIgnoresNonSQL(t *testing.T) {
	fsys := fstest.MapFS{
		"0001_init.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
		"migrations.go": &fstest.MapFile{Data: []byte("package migrations")},
		"README.md":     &fstest.MapFile{Data: []byte("notes")},
	}
	got, err := allFrom(fsys)
	if err != nil {
		t.Fatalf("allFrom() error = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("allFrom() returned %d migrations, want 1", len(got))
	}
}

func TestAllFromSortsByVersion(t *testing.T) {
	fsys := fstest.MapFS{
		"0010_ten.sql": &fstest.MapFile{Data: []byte("SELECT 10;")},
		"0002_two.sql": &fstest.MapFile{Data: []byte("SELECT 2;")},
		"0001_one.sql": &fstest.MapFile{Data: []byte("SELECT 1;")},
	}
	got, err := allFrom(fsys)
	if err != nil {
		t.Fatalf("allFrom() error = %v", err)
	}
	want := []int{1, 2, 10}
	for i, m := range got {
		if m.Version != want[i] {
			t.Errorf("position %d = version %d, want %d", i, m.Version, want[i])
		}
	}
}

func TestAllFromEmpty(t *testing.T) {
	got, err := allFrom(fstest.MapFS{})
	if err != nil {
		t.Fatalf("allFrom() error = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("allFrom() = %d migrations, want 0", len(got))
	}
}

// A failing migration must leave no partial schema behind, so the next attempt
// starts from a known state rather than a half-applied one.
func TestFailedMigrationRollsBack(t *testing.T) {
	db := open(t)
	ctx := context.Background()

	bad := Migration{Version: 999, Name: "bad", SQL: `
		CREATE TABLE should_not_survive (x TEXT);
		THIS IS NOT VALID SQL;
	`}
	if err := apply(ctx, db, bad); err == nil {
		t.Fatal("applying invalid SQL should fail")
	}

	var n int
	if err := db.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='should_not_survive'`,
	).Scan(&n); err != nil {
		t.Fatalf("checking rollback: %v", err)
	}
	if n != 0 {
		t.Error("a failed migration left a table behind; it must roll back atomically")
	}

	if v, err := Current(ctx, db); err != nil || v == 999 {
		t.Errorf("failed migration recorded its version: v=%d err=%v", v, err)
	}
}

func TestCurrentOnEmptyDatabase(t *testing.T) {
	db := open(t)
	got, err := Current(context.Background(), db)
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}
	if got != 0 {
		t.Errorf("Current() = %d on an empty database, want 0", got)
	}
}

func TestLatest(t *testing.T) {
	got, err := Latest()
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if got < 1 {
		t.Errorf("Latest() = %d, want at least 1", got)
	}
}
