package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/connect"
	"github.com/heliopsy/tix/internal/core"
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
