package cmd

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// pgDSNEnv names the environment variable that points the suite at a database.
// Without it every postgres-backed test skips, so the suite still passes.
const pgDSNEnv = "TIX_TEST_POSTGRES_DSN"

var pgCounter atomic.Int64

// freshPostgresDSN creates a database of its own for one test.
func freshPostgresDSN(t *testing.T) string {
	t.Helper()
	base := strings.TrimSpace(os.Getenv(pgDSNEnv))
	if base == "" {
		t.Skipf("%s is not set", pgDSNEnv)
	}
	admin, err := sql.Open("pgx", base)
	if err != nil {
		t.Fatalf("opening the admin connection: %v", err)
	}
	defer func() { _ = admin.Close() }()

	name := fmt.Sprintf("tix_cmd_%d_%d", os.Getpid(), pgCounter.Add(1))
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

func TestTenantFlagSelectsTheTenant(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "create", "acme", "Acme")
	c.mustRun("tenant", "create", "globex", "Globex")
	c.mustRun("--tenant", "acme", "project", "create", "acme", "Acme")
	c.mustRun("--tenant", "globex", "project", "create", "globex", "Globex")
	c.mustRun("--tenant", "acme", "task", "add", "acme work")
	c.mustRun("--tenant", "globex", "task", "add", "globex work")

	acme := c.mustRun("--tenant", "acme", "task", "ls", "-o", "json").out
	if !strings.Contains(acme, "acme work") || strings.Contains(acme, "globex work") {
		t.Fatalf("acme listing = %s", acme)
	}
	globex := c.mustRun("--tenant", "globex", "task", "ls", "-o", "json").out
	if !strings.Contains(globex, "globex work") || strings.Contains(globex, "acme work") {
		t.Fatalf("globex listing = %s", globex)
	}
	if def := c.mustRun("task", "ls", "-o", "json").out; strings.Contains(def, "work") {
		t.Fatalf("default listing = %s", def)
	}
}

func TestTenantFromTheEnvironmentIsHonoured(t *testing.T) {
	c := newCLI(t)
	c.mustRun("tenant", "create", "acme", "Acme")
	c.mustRun("--tenant", "acme", "project", "create", "acme", "Acme")

	if got := runWithEnv(c, "TIX_TENANT=acme", "task", "add", "acme work"); got.code != core.ExitOK {
		t.Fatalf("task add exited %d: %s", got.code, got.err)
	}
	if got := c.mustRun("--tenant", "default", "task", "ls", "-o", "json").out; strings.Contains(got, "acme work") {
		t.Fatalf("default listing = %s", got)
	}
}

// runWithEnv runs one invocation with extra environment variables set.
func runWithEnv(c *cli, env string, args ...string) result {
	var out, errb bytes.Buffer
	code := Run(args, strings.NewReader(""), &out, &errb, append(c.environ(), env), c.home)
	return result{code: code, out: out.String(), err: errb.String()}
}

func TestUnknownTenantIsReported(t *testing.T) {
	c := newCLI(t)
	got := c.run("--tenant", "nope", "task", "ls")
	if got.code != core.ExitNotFound {
		t.Fatalf("exit = %d, want %d", got.code, core.ExitNotFound)
	}
	if !strings.Contains(got.err, "nope") {
		t.Fatalf("stderr = %q, want the tenant key named", got.err)
	}
}

func TestPostgresDSNOpensPostgres(t *testing.T) {
	dsn := freshPostgresDSN(t)
	c := newCLI(t)
	if got := c.mustRun("--db", dsn, "-v", "task", "add", "on postgres"); !strings.Contains(got.out, "on postgres") {
		t.Fatalf("stdout = %q", got.out)
	}
	listing := c.mustRun("--db", dsn, "task", "ls", "-o", "json").out
	if !strings.Contains(listing, "on postgres") {
		t.Fatalf("listing = %s", listing)
	}
	if doctor := c.mustRun("--db", dsn, "doctor", "-o", "json").out; !strings.Contains(doctor, "postgres") {
		t.Fatalf("doctor = %s", doctor)
	}
}

func TestServeOpensTheEngineTheDSNNames(t *testing.T) {
	c := newCLI(t)
	c.mustRun("task", "add", "served from sqlite")
	assertServes(t, c)
}

func TestServeOpensPostgres(t *testing.T) {
	dsn := freshPostgresDSN(t)
	c := newCLI(t)
	c.mustRun("--db", dsn, "task", "add", "served from postgres")
	assertServes(t, c, "--db", dsn)
}

// assertServes starts the serve command, probes it and stops it with a signal.
func assertServes(t *testing.T, c *cli, target ...string) {
	t.Helper()
	addr := freePort(t)
	done := make(chan result, 1)
	go func() { done <- c.run(append(append([]string{}, target...), "serve", "--listen", addr)...) }()

	base := "http://" + addr
	waitFor(t, base+"/healthz")
	for _, path := range []string{"/healthz", "/readyz"} {
		resp, err := http.Get(base + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("GET %s = %d, want 200", path, resp.StatusCode)
		}
	}
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("signalling serve: %v", err)
	}
	select {
	case got := <-done:
		if got.code != core.ExitOK {
			t.Fatalf("serve exited %d: %s", got.code, got.err)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("serve did not shut down")
	}
}

// freePort returns a loopback address nothing is listening on.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("releasing the port: %v", err)
	}
	return addr
}

// waitFor blocks until the url answers or the deadline passes.
func waitFor(t *testing.T, url string) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("%s never answered", url)
}
