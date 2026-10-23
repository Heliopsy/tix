// SPDX-License-Identifier: AGPL-3.0-or-later

package sshd

import (
	"bytes"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// quietLog is a logger for the cases that are not asserting on what was logged.
func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestHostKeyIsGeneratedOnceAndReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "host_key")

	first, err := hostKey(path, quietLog())
	if err != nil {
		t.Fatalf("generating: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != hostKeyPerm {
		t.Errorf("host key written as %04o, want %04o", got, hostKeyPerm)
	}

	second, err := hostKey(path, quietLog())
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if string(first.PublicKey().Marshal()) != string(second.PublicKey().Marshal()) {
		t.Error("the host key changed on restart, which warns every returning visitor")
	}
}

func TestHostKeyRejections(t *testing.T) {
	dir := t.TempDir()
	garbage := filepath.Join(dir, "garbage")
	if err := os.WriteFile(garbage, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"no path", ""},
		{"not a key", garbage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := hostKey(tc.path, quietLog()); err == nil {
				t.Fatal("hostKey accepted it")
			}
		})
	}
}

// TestAWidenedHostKeyIsNarrowedRatherThanRefused covers the state a Kubernetes
// volume mount puts the key in. An fsGroup with the default
// fsGroupChangePolicy of Always adds group-rw to every file on the volume, so
// the 0600 the listener wrote itself comes back as 0660 at the next mount. The
// listener used to refuse to start, which read as the upgrade having broken it
// because the first restart after the key exists is usually a rollout.
func TestAWidenedHostKeyIsNarrowedRatherThanRefused(t *testing.T) {
	for _, mode := range []os.FileMode{0o660, 0o644, 0o666, 0o700} {
		t.Run(mode.String(), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "host_key")
			first, err := hostKey(path, quietLog())
			if err != nil {
				t.Fatalf("generating: %v", err)
			}
			if err := os.Chmod(path, mode); err != nil {
				t.Fatalf("chmod: %v", err)
			}

			var logged bytes.Buffer
			log := slog.New(slog.NewTextHandler(&logged, nil))
			second, err := hostKey(path, log)
			if err != nil {
				t.Fatalf("reloading a %04o key: %v, want it narrowed and accepted", mode.Perm(), err)
			}
			if string(first.PublicKey().Marshal()) != string(second.PublicKey().Marshal()) {
				t.Error("the host key changed, which warns every returning visitor")
			}

			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			if got := info.Mode().Perm(); got != hostKeyPerm {
				t.Errorf("host key left as %04o, want %04o", got, hostKeyPerm)
			}
			// Narrowing silently would hide a deployment that keeps widening
			// it, and the exposure that already happened is not undone by the
			// chmod, so it has to be said out loud.
			if !strings.Contains(logged.String(), "narrowed") {
				t.Errorf("nothing was logged about narrowing the key:\n%s", logged.String())
			}
		})
	}
}

// TestAHostKeyThatCannotBeNarrowedIsRefused is the other half: a mode this
// process cannot fix means the file is not ours, and that is the case an
// operator has to look at rather than have papered over.
func TestAHostKeyThatCannotBeNarrowedIsRefused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "host_key")
	if _, err := hostKey(path, quietLog()); err != nil {
		t.Fatalf("generating: %v", err)
	}
	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}

	t.Run("the chmod is refused", func(t *testing.T) {
		restore := chmodHostKey
		chmodHostKey = func(string, os.FileMode) error { return os.ErrPermission }
		t.Cleanup(func() { chmodHostKey = restore })

		if _, err := hostKey(path, quietLog()); err == nil {
			t.Fatal("a host key that could not be narrowed was accepted")
		}
	})

	t.Run("the chmod reports success and changes nothing", func(t *testing.T) {
		restore := chmodHostKey
		chmodHostKey = func(string, os.FileMode) error { return nil }
		t.Cleanup(func() { chmodHostKey = restore })

		if _, err := hostKey(path, quietLog()); err == nil {
			t.Fatal("a host key still readable after the chmod was accepted")
		}
	})
}
