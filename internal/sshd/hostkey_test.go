package sshd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestHostKeyIsGeneratedOnceAndReused(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys", "host_key")

	first, err := hostKey(path)
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

	second, err := hostKey(path)
	if err != nil {
		t.Fatalf("reloading: %v", err)
	}
	if string(first.PublicKey().Marshal()) != string(second.PublicKey().Marshal()) {
		t.Error("the host key changed on restart, which warns every returning visitor")
	}
}

func TestHostKeyRejections(t *testing.T) {
	dir := t.TempDir()
	loose := filepath.Join(dir, "loose")
	if _, err := hostKey(loose); err != nil {
		t.Fatalf("generating: %v", err)
	}
	if err := os.Chmod(loose, 0o644); err != nil {
		t.Fatalf("chmod: %v", err)
	}
	garbage := filepath.Join(dir, "garbage")
	if err := os.WriteFile(garbage, []byte("not a key"), 0o600); err != nil {
		t.Fatalf("writing: %v", err)
	}

	tests := []struct {
		name string
		path string
	}{
		{"no path", ""},
		{"world readable", loose},
		{"not a key", garbage},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := hostKey(tc.path); err == nil {
				t.Fatal("hostKey accepted it")
			}
		})
	}
}
