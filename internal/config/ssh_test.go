package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

func TestSSHKeysAreRegisteredWithTheirVariables(t *testing.T) {
	want := map[string]string{
		"ssh.listen":               "TIX_SSH_LISTEN",
		"ssh.host_key":             "TIX_SSH_HOST_KEY",
		"ssh.allow_public":         "TIX_SSH_ALLOW_PUBLIC",
		"ssh.demo":                 "TIX_SSH_DEMO",
		"ssh.tenant_ttl":           "TIX_SSH_TENANT_TTL",
		"ssh.reap_interval":        "TIX_SSH_REAP_INTERVAL",
		"ssh.max_tenants":          "TIX_SSH_MAX_TENANTS",
		"ssh.max_tasks":            "TIX_SSH_MAX_TASKS",
		"ssh.lease_ttl":            "TIX_SSH_LEASE_TTL",
		"ssh.rate_per_hour":        "TIX_SSH_RATE_PER_HOUR",
		"ssh.rate_burst":           "TIX_SSH_RATE_BURST",
		"ssh.idle_timeout":         "TIX_SSH_IDLE_TIMEOUT",
		"ssh.keepalive_interval":   "TIX_SSH_KEEPALIVE_INTERVAL",
		"ssh.keepalive_max_missed": "TIX_SSH_KEEPALIVE_MAX_MISSED",
		"ssh.max_sessions_per_key": "TIX_SSH_MAX_SESSIONS_PER_KEY",
		"ssh.max_sessions":         "TIX_SSH_MAX_SESSIONS",
	}
	for path, env := range want {
		key, ok := Lookup(path)
		if !ok {
			t.Errorf("key %q is not registered", path)
			continue
		}
		if key.Env != env {
			t.Errorf("key %q generates %q, want %q", path, key.Env, env)
		}
	}
}

func TestSSHDurationsAreDurationsNotStrings(t *testing.T) {
	cfg := Defaults()
	for _, path := range []string{
		"ssh.tenant_ttl", "ssh.reap_interval", "ssh.lease_ttl",
		"ssh.idle_timeout", "ssh.keepalive_interval",
	} {
		key, ok := Lookup(path)
		if !ok {
			t.Fatalf("key %q is not registered", path)
		}
		if err := key.Set(&cfg, "90s"); err != nil {
			t.Fatalf("setting %q: %v", path, err)
		}
		if got := key.Get(&cfg); got != "1m30s" {
			t.Errorf("%q round-tripped 90s as %q, want a duration", path, got)
		}
		if err := key.Set(&cfg, "120"); err == nil {
			t.Errorf("%q accepted a bare number, so it is not a duration", path)
		}
	}
}

func TestSSHPrecedenceRunsFileToEnvToFlag(t *testing.T) {
	dir := t.TempDir()
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"),
		"ssh:\n  listen: 127.0.0.1:2301\n  idle_timeout: 4m\n  max_sessions: 7\n")
	env := map[string]string{"HOME": home, EnvConfigFile: cfg}

	fromFile := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})
	if fromFile.Config.SSH.Listen != "127.0.0.1:2301" ||
		fromFile.Config.SSH.IdleTimeout != core.Duration(4*time.Minute) ||
		fromFile.Config.SSH.MaxSessions != 7 {
		t.Fatalf("file layer = %+v", fromFile.Config.SSH)
	}

	env["TIX_SSH_IDLE_TIMEOUT"] = "5m"
	fromEnv := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})
	if fromEnv.Config.SSH.IdleTimeout != core.Duration(5*time.Minute) {
		t.Fatalf("idle_timeout = %v, want the environment to beat the file", fromEnv.Config.SSH.IdleTimeout)
	}
	if fromEnv.Source("ssh.idle_timeout") != LayerEnv {
		t.Fatalf("source = %q, want %q", fromEnv.Source("ssh.idle_timeout"), LayerEnv)
	}
	if fromEnv.Config.SSH.Listen != "127.0.0.1:2301" {
		t.Fatalf("listen = %q, want the file value left alone", fromEnv.Config.SSH.Listen)
	}

	fromFlag := mustLoad(t, Options{
		Dir: dir, Home: home, Environ: environ(env),
		Flags: map[string]string{"ssh.idle_timeout": "6m"},
	})
	if fromFlag.Config.SSH.IdleTimeout != core.Duration(6*time.Minute) {
		t.Fatalf("idle_timeout = %v, want the flag layer to win", fromFlag.Config.SSH.IdleTimeout)
	}
	if fromFlag.Source("ssh.idle_timeout") != LayerFlag {
		t.Fatalf("source = %q, want %q", fromFlag.Source("ssh.idle_timeout"), LayerFlag)
	}
}

func TestSSHValidationRefusesValuesThatCannotBeUsed(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Config)
		key    string
	}{
		{"a negative window", func(c *Config) { c.SSH.TenantTTL = core.Duration(-time.Hour) }, "ssh.tenant_ttl"},
		{"a negative keepalive", func(c *Config) { c.SSH.KeepaliveInterval = core.Duration(-time.Second) }, "ssh.keepalive_interval"},
		{"a negative idle timeout", func(c *Config) { c.SSH.IdleTimeout = core.Duration(-time.Minute) }, "ssh.idle_timeout"},
		{"a negative cap", func(c *Config) { c.SSH.MaxTenants = -1 }, "ssh.max_tenants"},
		{"a negative miss count", func(c *Config) { c.SSH.KeepaliveMaxMissed = -2 }, "ssh.keepalive_max_missed"},
		{"an unreachable per-key cap", func(c *Config) {
			c.SSH.MaxSessions, c.SSH.MaxSessionsPerKey = 4, 5
		}, "ssh.max_sessions_per_key"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Defaults()
			tc.mutate(&cfg)
			err := Validate(&cfg, map[string]Layer{tc.key: LayerFile})
			if err == nil {
				t.Fatal("Validate accepted a value that cannot be used")
			}
			if !strings.Contains(err.Error(), tc.key) {
				t.Fatalf("error = %q, want it to name %q", err, tc.key)
			}
		})
	}
}

func TestSSHDefaultsAreUsable(t *testing.T) {
	cfg := Defaults()
	if err := Validate(&cfg, nil); err != nil {
		t.Fatalf("the shipped defaults do not validate: %v", err)
	}
	if cfg.SSH.KeepaliveInterval >= cfg.SSH.IdleTimeout {
		t.Fatal("the keepalive is no quicker than the idle timeout, so it notices nothing new")
	}
	if cfg.SSH.MaxSessionsPerKey > cfg.SSH.MaxSessions {
		t.Fatal("one key may hold more sessions than the listener")
	}
}
