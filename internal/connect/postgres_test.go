package connect

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/thereisnotime/tix/internal/config"
	"github.com/thereisnotime/tix/internal/core"
)

// dsnEnv names the environment variable that points the suite at a database.
// Without it every postgres-backed test skips, so the suite still passes.
const dsnEnv = "TIX_TEST_POSTGRES_DSN"

var pgCounter atomic.Int64

// freshPostgresDSN creates a database of its own for one test.
func freshPostgresDSN(t *testing.T) string {
	t.Helper()
	base := strings.TrimSpace(os.Getenv(dsnEnv))
	if base == "" {
		t.Skipf("%s is not set", dsnEnv)
	}
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("opening the admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()

	name := fmt.Sprintf("tix_connect_%d_%d", os.Getpid(), pgCounter.Add(1))
	ctx := context.Background()
	if _, err := admin.ExecContext(ctx, "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)"); err != nil {
		t.Fatalf("dropping %q: %v", name, err)
	}
	if _, err := admin.ExecContext(ctx, "CREATE DATABASE "+name); err != nil {
		t.Fatalf("creating %q: %v", name, err)
	}
	t.Cleanup(func() {
		cleanup, err := sql.Open("pgx", base)
		if err != nil {
			return
		}
		defer func() { _ = cleanup.Close() }()
		_, _ = cleanup.ExecContext(context.Background(), "DROP DATABASE IF EXISTS "+name+" WITH (FORCE)")
	})

	at := strings.LastIndex(base, "/")
	rest := ""
	if q := strings.Index(base[at:], "?"); q >= 0 {
		rest = base[at+q:]
	}
	return base[:at+1] + name + rest
}

// dialPostgres opens the given postgres dsn as the named tenant.
func dialPostgres(t *testing.T, home, dsn, tenant string) (*Conn, error) {
	t.Helper()
	resolved := load(t, home, nil, func(o *config.Options) {
		flags := map[string]string{"database.dsn": dsn}
		if tenant != "" {
			flags["tenant"] = tenant
		}
		o.Flags = flags
	})
	return Dial(context.Background(), resolved, Overrides{Home: home, DB: dsn})
}

func TestDialOpensThePostgresEngine(t *testing.T) {
	dsn := freshPostgresDSN(t)
	home := t.TempDir()
	ctx := context.Background()

	conn, err := dialPostgres(t, home, dsn, "")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	if conn.Info.Target.Engine != EnginePostgres {
		t.Fatalf("engine = %q, want postgres", conn.Info.Target.Engine)
	}
	if conn.Store == nil || conn.Store.Dialect() != "postgres" {
		t.Fatalf("store dialect = %v, want postgres", conn.Store)
	}
	if conn.Info.SchemaVersion <= 0 {
		t.Fatalf("schema version = %d, want positive", conn.Info.SchemaVersion)
	}
	if _, err := conn.Service.CreateTask(conn.Context(ctx), core.CreateTaskInput{Title: "on postgres"}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	assertOnlyTask(t, conn, "on postgres")
}

func TestDialOpensTheSelectedTenantOnPostgres(t *testing.T) {
	dsn := freshPostgresDSN(t)
	home := t.TempDir()
	ctx := context.Background()

	first, err := dialPostgres(t, home, dsn, "")
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	if _, err := first.Service.CreateTenant(first.Context(ctx),
		core.CreateTenantInput{Key: "acme", Name: "Acme"}); err != nil {
		t.Fatalf("CreateTenant: %v", err)
	}
	if _, err := first.Service.CreateTask(first.Context(ctx), core.CreateTaskInput{Title: "default work"}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	acme, err := dialPostgres(t, home, dsn, "acme")
	if err != nil {
		t.Fatalf("Dial acme: %v", err)
	}
	defer func() { _ = acme.Close() }()
	if _, err := acme.Service.CreateProject(acme.Context(ctx),
		core.CreateProjectInput{Key: "acme", Name: "Acme"}); err != nil {
		t.Fatalf("CreateProject: %v", err)
	}
	if _, err := acme.Service.CreateTask(acme.Context(ctx), core.CreateTaskInput{Title: "acme work"}); err != nil {
		t.Fatalf("CreateTask: %v", err)
	}
	assertOnlyTask(t, acme, "acme work")
}

func TestDialRejectsAnUnknownTenantOnPostgres(t *testing.T) {
	dsn := freshPostgresDSN(t)
	home := t.TempDir()
	if _, err := dialPostgres(t, home, dsn, "nope"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("err = %v, want not found", err)
	}
}
