package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

// dsnEnv names the environment variable that points the suite at a database.
// Without it every database-backed test skips, so the suite still passes.
const dsnEnv = "TIX_TEST_POSTGRES_DSN"

var dbCounter atomic.Int64

type fixture struct {
	store    *Store
	clock    *clock.Fake
	tenant   core.Tenant
	scope    core.TenantScope
	actor    core.Actor
	workflow core.Workflow
	project  core.Project
}

func requireDSN(t *testing.T) string {
	t.Helper()
	dsn := strings.TrimSpace(os.Getenv(dsnEnv))
	if dsn == "" {
		t.Skipf("%s is not set", dsnEnv)
	}
	return dsn
}

// freshDSN creates a database of its own for one test, so listings that count
// every row are not disturbed by another test running beside them.
func freshDSN(t *testing.T) string {
	t.Helper()
	base := requireDSN(t)
	admin, err := sql.Open(driverName, base)
	if err != nil {
		t.Fatalf("opening the admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()

	name := fmt.Sprintf("tix_test_%d_%d", os.Getpid(), dbCounter.Add(1))
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		t.Fatalf("dropping %q: %v", name, err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("creating %q: %v", name, err)
	}
	t.Cleanup(func() {
		cleaner, err := sql.Open(driverName, base)
		if err != nil {
			return
		}
		defer func() { _ = cleaner.Close() }()
		_, _ = cleaner.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})
	return replaceDatabase(base, name)
}

func replaceDatabase(dsn, name string) string {
	head, query, _ := strings.Cut(dsn, "?")
	slash := strings.LastIndex(head, "/")
	out := head[:slash+1] + name
	if query != "" {
		out += "?" + query
	}
	return out
}

func newStore(t *testing.T) (*Store, *clock.Fake) {
	t.Helper()
	clk := clock.NewFakeAt()
	s := openStore(t, freshDSN(t), clk)
	if err := s.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return s, clk
}

func openStore(t *testing.T, dsn string, clk clock.Clock) *Store {
	t.Helper()
	s, err := Open(dsn, clk)
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func seed(t *testing.T, s *Store, clk *clock.Fake, key string) fixture {
	t.Helper()
	ctx := context.Background()

	f := fixture{store: s, clock: clk}
	f.tenant = core.Tenant{Key: key, Name: key}
	if err := s.Unscoped(ctx, func(u store.UnscopedTx) error {
		return u.CreateTenant(ctx, &f.tenant)
	}); err != nil {
		t.Fatalf("creating tenant %q: %v", key, err)
	}
	f.scope = core.TenantScope{TenantID: f.tenant.ID}

	f.workflow = core.Workflow{
		Key:  "default",
		Name: "Default",
		Definition: core.WorkflowDefinition{
			Initial: "todo",
			States: []core.State{
				{Key: "todo", Label: "To do"},
				{Key: "doing", Label: "Doing"},
				{Key: "done", Label: "Done", Terminal: true},
			},
			Transitions: []core.Transition{{From: "todo", To: "doing"}, {From: "doing", To: "done"}},
		},
	}
	f.actor = core.Actor{Kind: core.ActorAgent, Handle: "worker", DisplayName: "Worker"}
	f.project = core.Project{Key: "alpha", Name: "Alpha"}

	err := s.Update(ctx, f.scope, func(tx store.Tx) error {
		if err := tx.PutWorkflow(ctx, &f.workflow); err != nil {
			return err
		}
		if err := tx.CreateActor(ctx, &f.actor); err != nil {
			return err
		}
		f.project.WorkflowID = f.workflow.ID
		return tx.CreateProject(ctx, &f.project)
	})
	if err != nil {
		t.Fatalf("seeding tenant %q: %v", key, err)
	}
	return f
}

func (f fixture) newTask(t *testing.T, title string, priority core.Priority) core.Task {
	t.Helper()
	ctx := context.Background()
	task := core.Task{
		ProjectID:      f.project.ID,
		Title:          title,
		Status:         "todo",
		Priority:       priority,
		CreatorActorID: f.actor.ID,
	}
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.CreateTask(ctx, &task)
	}); err != nil {
		t.Fatalf("creating task %q: %v", title, err)
	}
	return task
}

func TestOpenRejectsAnEmptyDSN(t *testing.T) {
	if _, err := Open("   ", nil); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("empty dsn = %v, want invalid", err)
	}
}

func TestRedactHidesThePassword(t *testing.T) {
	got := redact("postgres://tix:secret@127.0.0.1:55432/tix?sslmode=disable")
	if strings.Contains(got, "secret") {
		t.Fatalf("redacted dsn still carries the password: %s", got)
	}
	if !strings.Contains(got, "tix@") {
		t.Fatalf("redacted dsn lost the user: %s", got)
	}
	if got := redact("::not a url::"); got != "::not a url::" {
		t.Fatalf("unparseable dsn = %q", got)
	}
}

func TestTranslateRewritesThePortableSchema(t *testing.T) {
	all, err := migrationsAll()
	if err != nil {
		t.Fatalf("reading migrations: %v", err)
	}
	stmts, err := translate(all)
	if err != nil {
		t.Fatalf("translating: %v", err)
	}
	joined := strings.Join(stmts, "\n")

	if strings.Contains(strings.ToUpper(joined), "AUTOINCREMENT") {
		t.Fatal("translated schema still contains AUTOINCREMENT")
	}
	if strings.Contains(joined, "TEXT NOT NULL,\n  updated_at TEXT") {
		t.Fatal("translated schema still stores timestamps as text")
	}
	for _, want := range []string{
		"created_at TIMESTAMPTZ NOT NULL",
		"locked_until TIMESTAMPTZ",
		"builtin BOOLEAN NOT NULL DEFAULT FALSE",
		"active BOOLEAN NOT NULL DEFAULT TRUE",
		"custom_fields JSONB NOT NULL DEFAULT '{}'",
		"seq BIGSERIAL NOT NULL",
		"PARTITION BY RANGE (occurred_at)",
		"PRIMARY KEY (seq, occurred_at)",
		"UNIQUE (id, occurred_at)",
		"blob BYTEA",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("translated schema is missing %q", want)
		}
	}
}

func TestAdjustmentsCoverEveryScopedTable(t *testing.T) {
	stmts := strings.Join(adjustments(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)), "\n")
	for _, table := range sqlb.ScopedTables() {
		if !strings.Contains(stmts, "ALTER TABLE "+table+" ENABLE ROW LEVEL SECURITY") {
			t.Fatalf("no row-level security for %q", table)
		}
		if !strings.Contains(stmts, "ALTER TABLE "+table+" FORCE ROW LEVEL SECURITY") {
			t.Fatalf("row-level security is not forced for %q", table)
		}
		if !strings.Contains(stmts, "CREATE POLICY "+isolationPolicy+" ON "+table) {
			t.Fatalf("no isolation policy for %q", table)
		}
	}
	if !strings.Contains(stmts, "search_tsv") {
		t.Fatal("no search vector on tasks")
	}
	if !strings.Contains(stmts, partitionName("events", time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC))) {
		t.Fatal("next month's event partition is missing")
	}
	if !strings.Contains(stmts, "events_default") {
		t.Fatal("no default partition for events")
	}
}

func TestSplitTopLevelIgnoresNestedCommas(t *testing.T) {
	parts := splitTopLevel("a INTEGER, b TEXT CHECK (b IN ('x','y')), c TEXT")
	if len(parts) != 3 {
		t.Fatalf("parts = %d (%q), want 3", len(parts), parts)
	}
	if !strings.Contains(parts[1], "'x','y'") {
		t.Fatalf("nested list was split: %q", parts[1])
	}
}

func TestMigrateOnFreshDatabaseAndRerun(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	v, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema version: %v", err)
	}
	if v < 1 {
		t.Fatalf("schema version = %d, want at least 1", v)
	}
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	again, err := s.SchemaVersion(ctx)
	if err != nil {
		t.Fatalf("schema version after rerun: %v", err)
	}
	if again != v {
		t.Fatalf("schema version changed on rerun: %d then %d", v, again)
	}
	if s.Dialect() != store.Postgres {
		t.Fatalf("dialect = %q", s.Dialect())
	}
	if err := s.Health(ctx); err != nil {
		t.Fatalf("health: %v", err)
	}
}

func TestEveryPortableTableExistsAfterMigration(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	for _, table := range append(sqlb.ScopedTables(), "tenants", "tenant_domains", "users") {
		var n int
		if err := s.db.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM information_schema.tables WHERE table_name = $1`, table).Scan(&n); err != nil {
			t.Fatalf("looking for %q: %v", table, err)
		}
		if n == 0 {
			t.Fatalf("table %q is missing after migration", table)
		}
	}
}

func TestTransactionsRefuseAnEmptyScope(t *testing.T) {
	ctx := context.Background()
	s, _ := newStore(t)

	if _, err := s.Begin(ctx, core.TenantScope{}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unscoped Begin = %v, want invalid", err)
	}
	if err := s.View(ctx, core.TenantScope{}, func(store.Tx) error { return nil }); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unscoped View = %v, want invalid", err)
	}
	if err := s.Update(ctx, core.TenantScope{}, func(store.Tx) error { return nil }); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("unscoped Update = %v, want invalid", err)
	}
}

func TestTenantSettingDoesNotLeakOntoTheNextTransaction(t *testing.T) {
	ctx := context.Background()
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")

	if err := s.View(ctx, f.scope, func(txn store.Tx) error {
		var setting string
		return txn.(*tx).ex.QueryRowContext(ctx,
			`SELECT current_setting($1, true)`, tenantSetting).Scan(&setting)
	}); err != nil {
		t.Fatalf("reading the tenant setting: %v", err)
	}

	var leaked sql.NullString
	if err := s.db.QueryRowContext(ctx, `SELECT current_setting($1, true)`, tenantSetting).Scan(&leaked); err != nil {
		t.Fatalf("reading the setting outside a transaction: %v", err)
	}
	if leaked.Valid && leaked.String != "" {
		t.Fatalf("tenant setting survived the transaction: %q", leaked.String)
	}
}
