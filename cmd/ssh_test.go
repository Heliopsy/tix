// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
	gossh "golang.org/x/crypto/ssh"
)

func TestSSHRefusesTheZeroConfigurationStore(t *testing.T) {
	c := newCLI(t)
	got := c.run("ssh", "--demo", "--listen", "127.0.0.1:0")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d: %s", got.code, core.ExitUsage, got.err)
	}
	if !strings.Contains(got.err, "your own work") {
		t.Fatalf("stderr = %q, want the refusal to name the store it protects", got.err)
	}
}

// TestSSHRefusesTheDefaultStoreWithoutCreatingIt pins the difference between
// refusing a database and refusing it after having made one. Opening a target
// migrates it into existence, so a guard that ran after the open left behind
// the very file it was protecting, on exactly the machines that had never run
// tix before.
func TestSSHRefusesTheDefaultStoreWithoutCreatingIt(t *testing.T) {
	c := newCLI(t)
	got := c.run("ssh", "--demo", "--listen", "127.0.0.1:0")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d: %s", got.code, core.ExitUsage, got.err)
	}
	var found []string
	_ = filepath.WalkDir(c.home, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.Contains(d.Name(), ".db") {
			found = append(found, path)
		}
		return nil
	})
	if len(found) != 0 {
		t.Fatalf("the refusal left a database behind: %v", found)
	}
}

// TestSSHInEnrolledModeServesTheConfiguredTarget is the other half of the
// guard above. Serving enrolled keys is the point of the hosted listener, so
// the refusal that protects a demo from real work must not fire here.
func TestSSHInEnrolledModeServesTheConfiguredTarget(t *testing.T) {
	c := newCLI(t)
	got := c.run("ssh", "--listen", "0.0.0.0:0")
	if strings.Contains(got.err, "your own work") {
		t.Fatalf("stderr = %q, want the demo-only refusal to stay out of enrolled mode", got.err)
	}
	if !strings.Contains(got.err, "non-loopback") {
		t.Fatalf("stderr = %q, want the run to have reached the bind guard", got.err)
	}
}

func TestSSHRefusesANonLoopbackBindWithoutTheChoice(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "demo.db")
	got := c.run("--db", db, "ssh", "--listen", "0.0.0.0:0")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d: %s", got.code, core.ExitUsage, got.err)
	}
	if !strings.Contains(got.err, "non-loopback") {
		t.Fatalf("stderr = %q, want the bind guard", got.err)
	}
}

func TestSSHServesAndShutsDownOnASignal(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "demo.db")
	_, done := startSSH(t, c, "--db", db, "ssh", "--listen", "127.0.0.1:0")

	got := stopSSH(t, done)
	if got.code != core.ExitOK {
		t.Fatalf("ssh exited %d: %s", got.code, got.err)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(db), hostKeyName)); err != nil {
		t.Fatalf("the listener served without its host key beside the database: %v", err)
	}
}

func TestResolveHostKey(t *testing.T) {
	tests := []struct {
		name   string
		flag   string
		target connect.Target
		want   string
		fails  bool
	}{
		{
			name:   "beside a sqlite database",
			target: connect.Target{Engine: connect.EngineSQLite, Path: "/var/lib/tix/demo.db"},
			want:   filepath.Join("/var/lib/tix", hostKeyName),
		},
		{
			name:   "the flag wins",
			flag:   "/etc/tix/host_key",
			target: connect.Target{Engine: connect.EngineSQLite, Path: "/var/lib/tix/demo.db"},
			want:   "/etc/tix/host_key",
		},
		{
			name:   "postgres needs one naming",
			target: connect.Target{Engine: connect.EnginePostgres, DSN: "postgres://localhost/tix"},
			fails:  true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := resolveHostKey(tc.flag, tc.target, "--host-key")
			if tc.fails {
				if err == nil {
					t.Fatalf("resolveHostKey = %q, want a refusal", got)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("resolveHostKey = %q, %v; want %q", got, err, tc.want)
			}
		})
	}
}

func TestSSHOptionsTakeConfiguredKeysAndFlagsBeatThem(t *testing.T) {
	cfg := config.SSH{
		Listen:             "127.0.0.1:2300",
		HostKey:            "/etc/tix/host_key",
		AllowPublic:        true,
		TenantTTL:          core.Duration(2 * time.Hour),
		ReapInterval:       core.Duration(5 * time.Minute),
		MaxTenants:         11,
		MaxTasks:           12,
		LeaseTTL:           core.Duration(90 * time.Second),
		RatePerHour:        13,
		RateBurst:          14,
		IdleTimeout:        core.Duration(7 * time.Minute),
		KeepaliveInterval:  core.Duration(15 * time.Second),
		KeepaliveMaxMissed: 4,
		MaxSessionsPerKey:  5,
		MaxSessions:        6,
		Demo:               true,
	}
	tests := []struct {
		name  string
		args  []string
		check func(*testing.T, sshOptions)
	}{
		{
			name: "no flag leaves the configured value",
			check: func(t *testing.T, o sshOptions) {
				if o.listen != cfg.Listen || o.hostKey != cfg.HostKey || !o.allowPublic {
					t.Fatalf("options = %+v, want the configured listener", o)
				}
				if o.idleTimeout != 7*time.Minute || o.keepaliveInterval != 15*time.Second {
					t.Fatalf("timings = %v/%v, want the configured ones", o.idleTimeout, o.keepaliveInterval)
				}
				if o.maxTenants != 11 || o.maxTasks != 12 || o.ratePerHour != 13 || o.rateBurst != 14 {
					t.Fatalf("limits = %+v, want the configured ones", o)
				}
				if o.keepaliveMaxMissed != 4 || o.maxSessionsPerKey != 5 || o.maxSessions != 6 {
					t.Fatalf("caps = %+v, want the configured ones", o)
				}
				if o.tenantTTL != 2*time.Hour || o.reapInterval != 5*time.Minute ||
					o.leaseTTL != 90*time.Second {
					t.Fatalf("windows = %+v, want the configured ones", o)
				}
			},
		},
		{
			name: "a flag beats the configured value",
			args: []string{"--idle-timeout", "9m", "--max-sessions", "42", "--listen", "127.0.0.1:2400"},
			check: func(t *testing.T, o sshOptions) {
				if o.idleTimeout != 9*time.Minute || o.maxSessions != 42 || o.listen != "127.0.0.1:2400" {
					t.Fatalf("options = %+v, want the flags to win", o)
				}
				if o.keepaliveInterval != 15*time.Second || o.maxSessionsPerKey != 5 {
					t.Fatalf("options = %+v, want the untouched keys to stay configured", o)
				}
			},
		},
		{
			name: "an explicit flag equal to its default still wins",
			args: []string{"--allow-public=false", "--demo=false"},
			check: func(t *testing.T, o sshOptions) {
				if o.allowPublic {
					t.Fatal("allow-public=false lost to the configured true")
				}
				if o.demo {
					t.Fatal("demo=false lost to the configured true")
				}
			},
		},
		{
			name: "the demo key is read like every other",
			check: func(t *testing.T, o sshOptions) {
				if !o.demo {
					t.Fatal("ssh.demo was configured and the listener ignored it")
				}
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var o sshOptions
			cmd := &cobra.Command{Use: "ssh"}
			registerSSHFlags(cmd, &o)
			if err := cmd.Flags().Parse(tc.args); err != nil {
				t.Fatalf("parsing %v: %v", tc.args, err)
			}
			tc.check(t, o.fromConfig(cmd, cfg))
		})
	}
}

func TestSSHTakesItsListenAddressFromAConfigurationFile(t *testing.T) {
	c := newCLI(t)
	db := filepath.Join(t.TempDir(), "demo.db")
	writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"),
		"ssh:\n  listen: 0.0.0.0:2222\n")

	got := c.run("--db", db, "ssh")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d: %s", got.code, core.ExitUsage, got.err)
	}
	if !strings.Contains(got.err, "0.0.0.0:2222") {
		t.Fatalf("stderr = %q, want the bind guard to name the configured address", got.err)
	}
}

// TestTheHTTPTrustedProxyListDoesNotGrantSSHTrust pins the two keys apart. The
// HTTP reverse proxy and the L4 SSH proxy need not be the same host, so a list
// written for one must not silently arm the other.
//
// Both directions are asserted, because the first alone passes over a listener
// that reads no header from anybody: the same dial is refused once
// ssh.trusted_proxies names the peer, which is what proves the discriminator
// means anything.
func TestTheHTTPTrustedProxyListDoesNotGrantSSHTrust(t *testing.T) {
	tests := []struct {
		name    string
		config  string
		refused bool
	}{
		{
			name:   "server.trusted_proxies alone",
			config: "server:\n  trusted_proxies: [127.0.0.1]\n",
		},
		{
			name:    "ssh.trusted_proxies names the peer",
			config:  "ssh:\n  trusted_proxies: [127.0.0.1]\n",
			refused: true,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			db := filepath.Join(t.TempDir(), "tix.db")
			writeFile(t, filepath.Join(c.home, "conf", "tix", "config.yaml"), tc.config)
			addr, done := startSSH(t, c, "--db", db, "ssh", "--listen", "127.0.0.1:0")

			// A plain SSH client sends no PROXY header. A listener that trusts
			// this peer closes it before the version exchange; one that does
			// not serves it like any other client.
			err := dialSSH(t, addr)
			switch {
			case tc.refused && err == nil:
				t.Error("the dial was served, so no proxy header was demanded of a listed peer")
			case tc.refused && errors.Is(err, syscall.ECONNREFUSED):
				t.Errorf("nothing was listening, so the refusal says nothing about proxy trust: %v", err)
			case tc.refused && strings.Contains(err.Error(), "unable to authenticate"):
				t.Errorf("the dial reached authentication, so no proxy header was demanded of a "+
					"listed peer: %v", err)
			case !tc.refused && err != nil:
				t.Errorf("the dial was refused, so a proxy header was demanded of a peer only "+
					"server.trusted_proxies names: %v", err)
			}

			if got := stopSSH(t, done); got.code != core.ExitOK {
				t.Fatalf("ssh exited %d: %s", got.code, got.err)
			}
		})
	}
}

// announcement is the line tix ssh prints once its listener is bound.
const announcement = "tix ssh listening on "

// announcer is a command's stdout that hands over the bound address the
// moment tix ssh announces it.
type announcer struct {
	mu    sync.Mutex
	buf   bytes.Buffer
	bound chan string
	once  sync.Once
}

func (a *announcer) Write(p []byte) (int, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	n, err := a.buf.Write(p)
	_, rest, found := strings.Cut(a.buf.String(), announcement)
	if addr, _, complete := strings.Cut(rest, "\n"); found && complete {
		a.once.Do(func() { a.bound <- addr })
	}
	return n, err
}

func (a *announcer) String() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.buf.String()
}

// startSSH runs tix in the background and returns once it has announced the
// address it bound, with the channel its result arrives on.
//
// The announcement is the readiness signal because it is printed after the
// interrupt handler is installed and the port is bound, and nothing before it
// is both. The host key these tests used to wait for is written earlier than
// either, so a signal sent on seeing it could land before the handler and a
// dial could land before the bind.
func startSSH(t *testing.T, c *cli, args ...string) (string, <-chan result) {
	t.Helper()
	out := &announcer{bound: make(chan string, 1)}
	done := make(chan result, 1)
	go func() {
		var errb bytes.Buffer
		code := Run(args, strings.NewReader(""), out, &errb, c.environ(), c.home)
		done <- result{code: code, out: out.String(), err: errb.String()}
	}()
	select {
	case addr := <-out.bound:
		return addr, done
	case got := <-done:
		t.Fatalf("tix %s exited %d before it bound: %s%s",
			strings.Join(args, " "), got.code, got.out, got.err)
		return "", nil
	}
}

// stopSSH interrupts a command startSSH started and waits for it to exit,
// which stopAndRead explains is safe. It sets no deadline of its own: a
// listener that ignores the signal hangs the test binary, whose timeout then
// reports the stuck goroutine rather than a wall clock that ran out.
func stopSSH(t *testing.T, done <-chan result) result {
	t.Helper()
	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("signalling: %v", err)
	}
	return <-done
}

// dialSSH authenticates against addr with a key nobody enrolled.
func dialSSH(t *testing.T, addr string) error {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}
	signer, err := gossh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatalf("signer: %v", err)
	}
	client, err := gossh.Dial("tcp", addr, &gossh.ClientConfig{
		User:            "tix",
		Auth:            []gossh.AuthMethod{gossh.PublicKeys(signer)},
		HostKeyCallback: gossh.InsecureIgnoreHostKey(), // #nosec G106 -- the test generated this host key
		Timeout:         30 * time.Second,
	})
	if err != nil {
		return err
	}
	return client.Close()
}
