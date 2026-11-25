// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/auth"
)

func TestPasswordHashingKeysResolve(t *testing.T) {
	home := t.TempDir()
	dir := t.TempDir()
	write(t, filepath.Join(dir, ".tix.yaml"), "auth:\n  argon2_memory_kib: 65536\n  argon2_time: 3\n")
	env := map[string]string{"HOME": home, "TIX_AUTH_MAX_CONCURRENT_HASHES": "3", "TIX_AUTH_ARGON2_THREADS": "2"}
	got := mustLoad(t, Options{Dir: dir, Home: home, Environ: environ(env)})

	if got.Config.Auth.MaxConcurrentHashes != 3 {
		t.Fatalf("max_concurrent_hashes = %d, want 3", got.Config.Auth.MaxConcurrentHashes)
	}
	if src := got.Source("auth.max_concurrent_hashes"); src != LayerEnv {
		t.Fatalf("max_concurrent_hashes came from %q, want %q", src, LayerEnv)
	}
	p := got.Config.Auth.PasswordParams()
	want := auth.DefaultParams()
	want.Memory, want.Time, want.Threads = 65536, 3, 2
	if p != want {
		t.Fatalf("PasswordParams = %+v, want %+v", p, want)
	}
}

func TestPasswordHashingDefaults(t *testing.T) {
	cfg := Defaults()
	if cfg.Auth.MaxConcurrentHashes != 0 {
		t.Fatalf("max_concurrent_hashes defaults to %d, want 0 (sized from memory)", cfg.Auth.MaxConcurrentHashes)
	}
	if got := cfg.Auth.PasswordParams(); got != auth.DefaultParams() {
		t.Fatalf("default PasswordParams = %+v, want %+v", got, auth.DefaultParams())
	}
}

func TestPasswordHashingValidation(t *testing.T) {
	tests := []struct {
		name string
		edit func(*Auth)
		key  string
	}{
		{"negative concurrency", func(a *Auth) { a.MaxConcurrentHashes = -1 }, "auth.max_concurrent_hashes"},
		{"negative memory", func(a *Auth) { a.Argon2MemoryKiB = -1 }, "auth.argon2_memory_kib"},
		{"threads past a byte", func(a *Auth) { a.Argon2Threads = 256 }, "auth.argon2_threads"},
		{"memory below the floor", func(a *Auth) { a.Argon2MemoryKiB = 4096 }, "auth.argon2_memory_kib"},
		{"memory times passes below the floor", func(a *Auth) { a.Argon2MemoryKiB = 8192; a.Argon2Time = 1 },
			"auth.argon2_memory_kib"},
		{"one pass of the default memory", func(a *Auth) { a.Argon2Time = 1 }, ""},
		{"more memory", func(a *Auth) { a.Argon2MemoryKiB = 131072 }, ""},
		{"a fixed concurrency", func(a *Auth) { a.MaxConcurrentHashes = 1 }, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Defaults()
			tt.edit(&cfg.Auth)
			err := Validate(&cfg, map[string]Layer{})
			if tt.key == "" {
				if err != nil {
					t.Fatalf("a valid setting was refused: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.key) {
				t.Fatalf("Validate = %v, want a refusal naming %s", err, tt.key)
			}
		})
	}
}
