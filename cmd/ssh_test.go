package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
	"github.com/spf13/cobra"
)

func TestSSHRefusesTheZeroConfigurationStore(t *testing.T) {
	c := newCLI(t)
	got := c.run("ssh", "--listen", "127.0.0.1:0")
	if got.code != core.ExitUsage {
		t.Fatalf("exit = %d, want %d: %s", got.code, core.ExitUsage, got.err)
	}
	if !strings.Contains(got.err, "zero-configuration store") {
		t.Fatalf("stderr = %q, want the refusal to name the store it protects", got.err)
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
	addr := freePort(t)

	done := make(chan result, 1)
	go func() { done <- c.run("--db", db, "ssh", "--listen", addr) }()

	deadline := time.Now().Add(30 * time.Second)
	hostKey := filepath.Join(filepath.Dir(db), hostKeyName)
	for {
		if _, err := os.Stat(hostKey); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the listener never wrote its host key")
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
		t.Fatalf("signalling: %v", err)
	}
	select {
	case got := <-done:
		if got.code != core.ExitOK {
			t.Fatalf("ssh exited %d: %s", got.code, got.err)
		}
		if !strings.Contains(got.out, "listening on") {
			t.Fatalf("stdout = %q, want the bound address", got.out)
		}
	case <-time.After(30 * time.Second):
		t.Fatal("the listener did not shut down")
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
			got, err := resolveHostKey(tc.flag, tc.target)
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
			args: []string{"--allow-public=false"},
			check: func(t *testing.T, o sshOptions) {
				if o.allowPublic {
					t.Fatal("allow-public=false lost to the configured true")
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
