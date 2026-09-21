package bench

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	// The pgx driver creates and drops the throwaway bench database.
	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/postgres"
	"github.com/heliopsy/tix/internal/store/sqlite"
)

// engine is one database the suite measures.
type engine struct {
	name string
	open func(tb testing.TB) store.Store
}

// engines returns SQLite always and Postgres only when its DSN is set, which is
// the same gate the engine test suites use.
func engines(tb testing.TB) []engine {
	tb.Helper()
	only := strings.ToLower(strings.TrimSpace(os.Getenv(EngineEnv)))
	var out []engine
	if only == "" || only == "sqlite" {
		out = append(out, engine{name: "sqlite", open: openSQLite})
	}
	if only == "" || only == "postgres" {
		if strings.TrimSpace(os.Getenv(PostgresEnv)) != "" {
			out = append(out, engine{name: "postgres", open: openPostgres})
		} else {
			tb.Logf("%s is not set; the postgres run is skipped", PostgresEnv)
		}
	}
	if len(out) == 0 {
		tb.Skipf("%s=%q selects no available engine", EngineEnv, only)
	}
	return out
}

func openSQLite(tb testing.TB) store.Store {
	tb.Helper()
	dir, err := os.MkdirTemp("", "tix-bench-*")
	if err != nil {
		tb.Fatalf("creating the bench directory: %v", err)
	}
	st, err := sqlite.Open(filepath.Join(dir, "bench.db"), clock.New())
	if err != nil {
		tb.Fatalf("opening the bench sqlite store: %v", err)
	}
	if err := st.Migrate(context.Background()); err != nil {
		tb.Fatalf("migrating the bench sqlite store: %v", err)
	}
	return st
}

// pgDropper drops the throwaway database once the store is closed. TestMain
// runs it, because a fixture outlives the test that seeded it.
var (
	pgDropper func()
	pgDSN     string
)

// openPostgres gives the fixture a database of its own, the same isolation the
// engine suite uses, so bench rows never land in a shared schema.
func openPostgres(tb testing.TB) store.Store {
	tb.Helper()
	base := strings.TrimSpace(os.Getenv(PostgresEnv))
	name := fmt.Sprintf("tix_bench_%d", os.Getpid())

	admin, err := sql.Open("pgx", base)
	if err != nil {
		tb.Fatalf("opening the admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		tb.Fatalf("dropping %q: %v", name, err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		tb.Fatalf("creating %q: %v", name, err)
	}
	pgDropper = func() {
		cleaner, err := sql.Open("pgx", base)
		if err != nil {
			return
		}
		defer func() { _ = cleaner.Close() }()
		_, _ = cleaner.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	}

	pgDSN = replaceDatabase(base, name)
	st, err := postgres.Open(pgDSN, clock.New())
	if err != nil {
		tb.Fatalf("opening the bench postgres store: %v", err)
	}
	if err := st.Migrate(ctx); err != nil {
		tb.Fatalf("migrating the bench postgres store: %v", err)
	}
	return st
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

// cached holds one fixture per engine for the life of the process. Seeding the
// full profile takes minutes, so every benchmark shares it.
var (
	cacheMu sync.Mutex
	cache   = map[string]*Fixture{}
)

// fixtureFor seeds, or returns the already seeded, fixture for one engine.
func fixtureFor(tb testing.TB, e engine) *Fixture {
	tb.Helper()
	cacheMu.Lock()
	defer cacheMu.Unlock()
	if f, ok := cache[e.name]; ok {
		return f
	}

	spec := SpecFromEnv()
	st := e.open(tb)
	tb.Logf("seeding the %q fixture on %s: %d tasks, %d projects, %d tenants",
		spec.Name, e.name, spec.Tasks, spec.Projects, spec.Tenants)

	f, err := Seed(context.Background(), st, spec)
	if err != nil {
		_ = st.Close()
		tb.Fatalf("seeding the bench fixture on %s: %v", e.name, err)
	}
	analyze(tb, f)
	cache[e.name] = f
	return f
}

// analyze gives Postgres the statistics autovacuum would have collected in a
// running deployment. Without it the planner reads a freshly loaded table as
// empty and every measurement is of the wrong plan.
func analyze(tb testing.TB, f *Fixture) {
	tb.Helper()
	if f.Dialect != store.Postgres || pgDSN == "" {
		return
	}
	db, err := sql.Open("pgx", pgDSN)
	if err != nil {
		tb.Fatalf("opening the analyze connection: %v", err)
	}
	defer func() { _ = db.Close() }()
	if _, err := db.ExecContext(context.Background(), "ANALYZE"); err != nil {
		tb.Fatalf("analyzing the bench database: %v", err)
	}
}

// TestMain drops every seeded fixture, which matters on Postgres where the
// database outlives the process.
func TestMain(m *testing.M) {
	code := m.Run()
	cacheMu.Lock()
	for _, f := range cache {
		_ = f.Store.Close()
	}
	if pgDropper != nil {
		pgDropper()
	}
	cacheMu.Unlock()
	os.Exit(code)
}
