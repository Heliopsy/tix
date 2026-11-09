// SPDX-License-Identifier: AGPL-3.0-or-later

package loadtest

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// LocalOptions describe the scratch server a local run starts.
type LocalOptions struct {
	// Tix is the tix binary to seed and serve with.
	Tix string
	// Dir is the scratch directory: database, XDG directories and nothing else.
	Dir string
	// DSN is a PostgreSQL database to use instead of SQLite in Dir. It must be
	// empty and disposable: the run writes tenants, users and tasks into it.
	DSN  string
	Seed Seed
	Logf func(string, ...any)
}

// Local is a running scratch server.
type Local struct {
	Base     string
	Accounts []Account
	Engine   string
	Seeded   string
	Probe    Probe

	cmd  *exec.Cmd
	done chan error
}

// loadPassword is every seeded account's password. The database is scratch.
const loadPassword = "tixload-scratch-password"

// StartLocal bootstraps tenants and accounts with the tix CLI, seeds content
// through a first server, then starts a fresh server to measure, so the
// seeding's own allocations are not part of the measurement.
func StartLocal(ctx context.Context, o LocalOptions) (*Local, error) {
	env := isolatedEnv(o.Dir)
	dsn, engine := filepath.Join(o.Dir, "tix.db"), "sqlite"
	if o.DSN != "" {
		dsn, engine = o.DSN, "postgres"
	}
	logf := o.Logf
	if logf == nil {
		logf = func(string, ...any) {}
	}
	cli := func(args ...string) ([]byte, error) {
		full := append([]string{"--db", dsn, "--no-discovery", "--log-level", "warn"}, args...)
		cmd := exec.CommandContext(ctx, o.Tix, full...)
		cmd.Env, cmd.Dir = env, o.Dir
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("tix %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
		}
		return out, nil
	}

	logf("bootstrapping %d tenants with %d people each (%s)", o.Seed.Tenants, o.Seed.UsersPerTenant, engine)
	accounts, err := bootstrap(cli, o.Seed)
	if err != nil {
		return nil, err
	}

	seeder, err := serve(ctx, o.Tix, dsn, o.Dir, env, false)
	if err != nil {
		return nil, err
	}
	shares := split(o.Seed.Tasks, o.Seed.Tenants)
	logf("seeding %d tasks with %d-%d KiB descriptions through %s", o.Seed.Tasks, o.Seed.MinBody>>10, o.Seed.MaxBody>>10, seeder.Base)
	for t := range o.Seed.Tenants {
		lead := accounts[t*o.Seed.UsersPerTenant]
		if _, err := SeedTenant(ctx, seeder.Base, lead, o.Seed, shares[t], uint64(t)+7, logf); err != nil {
			_ = seeder.Stop()
			return nil, fmt.Errorf("seeding tenant %s: %w", lead.Tenant, err)
		}
	}
	if err := seeder.Stop(); err != nil {
		return nil, fmt.Errorf("stopping the seeding server: %w", err)
	}

	l, err := serve(ctx, o.Tix, dsn, o.Dir, env, true)
	if err != nil {
		return nil, err
	}
	l.Accounts, l.Engine = accounts, engine
	l.Seeded = fmt.Sprintf("%d tenants, %d people, %d tasks, %d-%d KiB descriptions",
		o.Seed.Tenants, o.Seed.Tenants*o.Seed.UsersPerTenant, o.Seed.Tasks, o.Seed.MinBody>>10, o.Seed.MaxBody>>10)
	logf("measuring server pid %d at %s", l.cmd.Process.Pid, l.Base)
	return l, nil
}

// isolatedEnv drops every TIX_ variable and points the XDG directories into
// dir, so no configuration, context or default store of the caller's is reachable.
func isolatedEnv(dir string) []string {
	var env []string
	for _, kv := range os.Environ() {
		if strings.HasPrefix(kv, "TIX_") || strings.HasPrefix(kv, "XDG_") || strings.HasPrefix(kv, "GODEBUG=") {
			continue
		}
		env = append(env, kv)
	}
	for _, x := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
		env = append(env, fmt.Sprintf("XDG_%s_HOME=%s", x, filepath.Join(dir, strings.ToLower(x))))
	}
	return env
}

// bootstrap creates the tenants, one admin and some members in each, and a
// token for every person, the way an operator would from a shell.
func bootstrap(cli func(...string) ([]byte, error), s Seed) ([]Account, error) {
	var accounts []Account
	for t := range s.Tenants {
		key := fmt.Sprintf("load-%c", 'a'+t)
		if _, err := cli("tenant", "create", key, "--name", "Load "+strings.ToUpper(key[5:]), "-q"); err != nil {
			return nil, err
		}
		for u := range s.UsersPerTenant {
			handle := fmt.Sprintf("u%d", u+1)
			email := fmt.Sprintf("%s@%s.load.invalid", handle, key)
			role := "member"
			if u == 0 {
				role = "admin"
			}
			if _, err := cli("tenant", "user", "create", key, email, "--handle", handle, "--role", role, "--password", loadPassword, "-q"); err != nil {
				return nil, err
			}
			out, err := cli("--tenant", key, "token", "create", "load-"+handle, "--actor", handle, "--scope", "*", "--expires", "1d", "-o", "json")
			if err != nil {
				return nil, err
			}
			var tok struct {
				Token string `json:"token"`
			}
			if err := json.Unmarshal(out, &tok); err != nil || tok.Token == "" {
				return nil, fmt.Errorf("reading the token minted for %s: %v", email, err)
			}
			accounts = append(accounts, Account{Tenant: key, Email: email, Password: loadPassword, Token: tok.Token})
		}
	}
	return accounts, nil
}

func freePort() (int, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("finding a free port: %w", err)
	}
	defer func() { _ = l.Close() }()
	return l.Addr().(*net.TCPAddr).Port, nil
}

// serve starts `tix serve` on a free loopback port and waits until it is ready.
// With trace set, the Go runtime's gctrace lines are read for the live heap.
func serve(ctx context.Context, tix, dsn, dir string, env []string, trace bool) (*Local, error) {
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	addr := fmt.Sprintf("127.0.0.1:%d", port)
	cmd := exec.Command(tix, "serve", "--db", dsn, "--no-discovery", "--listen", addr, "--log-level", "warn")
	cmd.Env, cmd.Dir = env, dir
	l := &Local{Base: "http://" + addr, cmd: cmd, done: make(chan error, 1)}
	heap := &heapWatch{}
	if trace {
		cmd.Env = append(append([]string(nil), env...), "GODEBUG=gctrace=1")
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("piping the server's stderr: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting tix serve: %w", err)
	}
	go heap.consume(stderr)
	go func() { l.done <- cmd.Wait() }()
	l.Probe = Probe{PID: cmd.Process.Pid, heap: heap}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case err := <-l.done:
			l.done <- err
			return nil, fmt.Errorf("tix serve exited before it was ready: %v\n%s", err, heap.log())
		case <-ctx.Done():
			_ = l.Stop()
			return nil, ctx.Err()
		case <-time.After(100 * time.Millisecond):
		}
		if ready(ctx, l.Base) {
			return l, nil
		}
	}
	_ = l.Stop()
	return nil, fmt.Errorf("tix serve did not become ready in 30s:\n%s", heap.log())
}

func ready(ctx context.Context, base string) bool {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/readyz", nil)
	if err != nil {
		return false
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return false
	}
	_ = resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// Stop ends the server it started, by its own PID, and waits for it to exit.
func (l *Local) Stop() error {
	if l == nil || l.cmd == nil || l.cmd.Process == nil {
		return nil
	}
	select {
	case err := <-l.done:
		l.done <- err
		return nil
	default:
	}
	_ = l.cmd.Process.Signal(syscall.SIGTERM)
	select {
	case err := <-l.done:
		l.done <- err
		var exit *exec.ExitError
		if err != nil && !errors.As(err, &exit) {
			return err
		}
		return nil
	case <-time.After(15 * time.Second):
		_ = l.cmd.Process.Kill()
		err := <-l.done
		l.done <- err
		return fmt.Errorf("tix serve pid %d ignored SIGTERM for 15s and was killed", l.cmd.Process.Pid)
	}
}
