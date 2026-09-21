package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/cmd"
	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
)

// cliResult captures one CLI invocation's outcome.
type cliResult struct {
	code int
	out  string
	err  string
}

// cliEnv is an isolated environment for driving cmd.Run, mirroring the setup
// cmd's own tests use so a --db-scoped invocation never touches a real
// installation's configuration.
type cliEnv struct {
	t    *testing.T
	home string
}

// newCLIEnv prepares a throwaway home directory with no configuration.
func newCLIEnv(t *testing.T) *cliEnv {
	t.Helper()
	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, ".git"), 0o700); err != nil {
		t.Fatalf("preparing the isolated home: %v", err)
	}
	return &cliEnv{t: t, home: home}
}

func (e *cliEnv) environ() []string {
	return []string{
		"HOME=" + e.home,
		"XDG_DATA_HOME=" + filepath.Join(e.home, "data"),
		"XDG_CONFIG_HOME=" + filepath.Join(e.home, "conf"),
		"NO_COLOR=1",
	}
}

// run executes one invocation synchronously and returns once it exits. It is
// not for "tix serve", which blocks until signalled; see (*cliEnv).runAsync.
func (e *cliEnv) run(args ...string) cliResult {
	e.t.Helper()
	var out, errb bytes.Buffer
	code := cmd.Run(args, strings.NewReader(""), &out, &errb, e.environ(), e.home)
	return cliResult{code: code, out: out.String(), err: errb.String()}
}

func (e *cliEnv) mustRun(args ...string) cliResult {
	e.t.Helper()
	got := e.run(args...)
	if got.code != core.ExitOK {
		e.t.Fatalf("tix %s exited %d\nstdout: %s\nstderr: %s", strings.Join(args, " "), got.code, got.out, got.err)
	}
	return got
}

// runAsync starts a long-running invocation, such as "serve", on a goroutine
// and delivers its result on the returned channel once it exits.
func (e *cliEnv) runAsync(args ...string) <-chan cliResult {
	e.t.Helper()
	done := make(chan cliResult, 1)
	go func() {
		var out, errb bytes.Buffer
		code := cmd.Run(args, strings.NewReader(""), &out, &errb, e.environ(), e.home)
		done <- cliResult{code: code, out: out.String(), err: errb.String()}
	}()
	return done
}

// freeLoopbackAddr reserves a loopback port nothing is listening on and
// releases it immediately, so "tix serve" binds an OS-assigned port without
// this test ever hard-coding one. There is an inherent, small TOCTOU window
// between the release and tix's own bind; that risk is accepted here exactly
// as it already is in cmd/target_test.go's freePort.
func freeLoopbackAddr(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserving a port: %v", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		t.Fatalf("releasing the reserved port: %v", err)
	}
	return addr
}

// servedProcess is one running "tix serve" invocation, driven in-process via
// cmd.Run. Stopping it is idempotent: the eventual t.Cleanup call and any
// earlier, explicit call to stop from the test body are the same sync.Once,
// so a stray extra SIGINT is never sent once the process has already exited.
// A stray SIGINT with nothing left registered to catch it falls back to the
// OS default disposition and kills the whole test binary, which is exactly
// the failure this guards against.
type servedProcess struct {
	base string
	done <-chan cliResult
	stop func()
}

// startServe boots "tix serve" on an OS-assigned port and blocks until it
// answers healthy. args must not include --listen.
func (e *cliEnv) startServe(t *testing.T, args ...string) *servedProcess {
	t.Helper()
	addr := freeLoopbackAddr(t)
	done := e.runAsync(append(append([]string{}, args...), "--listen", addr)...)

	var once sync.Once
	stop := func() {
		once.Do(func() {
			_ = syscall.Kill(os.Getpid(), syscall.SIGINT)
			select {
			case got := <-done:
				if got.code != core.ExitOK {
					t.Errorf("tix serve exited %d\nstdout: %s\nstderr: %s", got.code, got.out, got.err)
				}
			case <-time.After(30 * time.Second):
				t.Error("tix serve did not shut down within 30s of SIGINT")
			}
		})
	}
	t.Cleanup(stop)

	base := "http://" + addr
	waitForHealthy(t, base, 30*time.Second)
	return &servedProcess{base: base, done: done, stop: stop}
}

// waitForHealthy polls until the server answers or the deadline passes,
// which is what lets the test start issuing --server commands the instant
// the listener is actually accepting rather than racing its startup.
func waitForHealthy(t *testing.T, base string, within time.Duration) {
	t.Helper()
	deadline := time.Now().Add(within)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(base + "/healthz")
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return
			}
			lastErr = nil
		} else {
			lastErr = err
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("server at %s never became healthy within %s: %v", base, within, lastErr)
}

// TestCLIEndToEndOverAServer drives an actual "tix" process image, via
// cmd.Run rather than a subprocess, through the exact sequence a human
// verified by hand: mint a token, serve a scratch database on an
// OS-assigned port, then read and write through --server and --token. It
// asserts a remote write lands in the underlying database and a direct
// database write is visible through the remote read, which is the one
// end-to-end path nothing else in this suite drives through the CLI binary
// itself.
//
// This test is deliberately not parallel: it sends SIGINT to this process to
// stop "tix serve", exactly as cmd/target_test.go's own serve tests do, and
// two such signals racing between parallel subtests would stop the wrong
// server.
func TestCLIEndToEndOverAServer(t *testing.T) {
	env := newCLIEnv(t)
	dbPath := filepath.Join(env.home, "data", "scratch.db")

	issue := env.mustRun("--db", dbPath, "token", "create", "ci",
		"--scope", "task:read", "--scope", "task:write", "-o", "json")
	var issued struct {
		Token string `json:"token"`
	}
	if err := json.Unmarshal([]byte(issue.out), &issued); err != nil || issued.Token == "" {
		t.Fatalf("token create -o json = %q, want a decodable token: %v", issue.out, err)
	}

	sp := env.startServe(t, "--db", dbPath, "serve",
		"--no-lease-sweeper", "--no-webhook-dispatcher", "--no-retention-pruner")

	// A remote read against a database with no tasks yet.
	listing := env.mustRun("--server", sp.base, "--token", issued.Token, "task", "ls", "-o", "json")
	if strings.Contains(listing.out, "written remotely") {
		t.Fatalf("a fresh database already shows a task before any write: %s", listing.out)
	}

	// A remote write.
	added := env.mustRun("--server", sp.base, "--token", issued.Token, "task", "add", "written remotely")
	if !strings.Contains(added.out, "written remotely") {
		t.Fatalf("task add stdout = %q, want the title", added.out)
	}

	// The remote write must be visible straight in the database: stop the
	// server first, since the sqlite connection this test opens directly and
	// the one inside the running server would otherwise both hold the file.
	sp.stop()
	assertTaskTitleInDB(t, dbPath, "written remotely")

	// A direct database write, read back once a fresh server is brought up
	// over the same file. This is the "vice versa" half of the round trip:
	// the CLI over --server sees what a direct writer left.
	writeTaskDirectlyToDB(t, dbPath, "written directly to the database")

	sp2 := env.startServe(t, "--db", dbPath, "serve",
		"--no-lease-sweeper", "--no-webhook-dispatcher", "--no-retention-pruner")

	listing2 := env.mustRun("--server", sp2.base, "--token", issued.Token, "task", "ls", "-o", "json")
	if !strings.Contains(listing2.out, "written directly to the database") {
		t.Fatalf("remote listing = %s, want the directly-written task visible over --server", listing2.out)
	}
	if !strings.Contains(listing2.out, "written remotely") {
		t.Fatalf("remote listing = %s, want the earlier remote write still visible", listing2.out)
	}
}

// directService opens dbPath on its own connection, straight at the store,
// with no server and no CLI in between: the same shape as a hand-verified
// "read the database yourself" check.
func directService(t *testing.T, dbPath string) (core.Service, context.Context) {
	t.Helper()
	clk := clock.New()
	st := openStore(t, dbPath, clk)
	svc := service.New(st,
		service.WithClock(clk),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())))
	tenant, err := svc.EnsureDefaults(context.Background())
	if err != nil {
		t.Fatalf("resolving the default tenant in %q: %v", dbPath, err)
	}
	bootstrapCtx := core.WithSource(core.WithActor(context.Background(), core.SystemActor(tenant.ID)), core.SourceCLI)

	// CreateTask records a creator_actor_id that must reference a persisted
	// actor row; the synthetic SystemActor used for bootstrap is never
	// persisted, so a real user is minted for this connection to write as,
	// the same way the cross-transport harness does for its direct writer.
	handle := "e2e-direct-" + randSuffix()
	user, err := svc.CreateUser(bootstrapCtx, core.CreateUserInput{
		Email: handle + "@example.test", Handle: handle, Role: core.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("creating the direct-write actor in %q: %v", dbPath, err)
	}
	direct := &core.Actor{
		ID: user.ID, TenantID: tenant.ID, Kind: core.ActorUser,
		Handle: handle, Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll},
	}
	ctx := core.WithSource(core.WithActor(context.Background(), direct), core.SourceCLI)
	return svc, ctx
}

// assertTaskTitleInDB fails the test unless some task in dbPath carries the
// given title, proving a write reached the database itself rather than only
// the responding process's memory.
func assertTaskTitleInDB(t *testing.T, dbPath, title string) {
	t.Helper()
	svc, ctx := directService(t, dbPath)
	page, err := svc.ListTasks(ctx, core.TaskFilter{})
	if err != nil {
		t.Fatalf("listing tasks directly in %q: %v", dbPath, err)
	}
	for _, tsk := range page.Tasks {
		if tsk.Title == title {
			return
		}
	}
	t.Fatalf("no task titled %q found directly in %q after a remote write", title, dbPath)
}

// writeTaskDirectlyToDB creates a task straight against the store, with no
// server and no CLI involved, the way a direct-to-database caller would.
func writeTaskDirectlyToDB(t *testing.T, dbPath, title string) {
	t.Helper()
	svc, ctx := directService(t, dbPath)
	if _, err := svc.CreateTask(ctx, core.CreateTaskInput{ProjectRef: service.DefaultProjectKey, Title: title}); err != nil {
		t.Fatalf("writing a task directly to %q: %v", dbPath, err)
	}
}
