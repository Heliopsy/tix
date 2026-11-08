// SPDX-License-Identifier: AGPL-3.0-or-later

package sync

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfinedNameKeepsAPathInsideItsDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "acme")
	cases := []struct {
		name string
		want string
		ok   bool
	}{
		{"rows.csv", "rows.csv", true},
		{"sub/rows.csv", filepath.Join("sub", "rows.csv"), true},
		{filepath.Join(dir, "rows.csv"), "rows.csv", true},
		{"../beta/rows.csv", "", false},
		{"sub/../../rows.csv", "", false},
		{"/etc/passwd", "", false},
		{filepath.Join(dir, "..", "beta", "rows.csv"), "", false},
		{dir, "", false},
		{".", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, err := ConfinedName(dir, tc.name)
		if (err == nil) != tc.ok || got != tc.want {
			t.Errorf("ConfinedName(%q) = %q, %v; want %q, ok %v", tc.name, got, err, tc.want, tc.ok)
		}
	}
}

func TestReadConfinedRefusesWhatIsNotARegularFileInside(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "acme")
	if err := os.MkdirAll(filepath.Join(dir, "sub"), 0o700); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(root, "secret.txt")
	for path, body := range map[string]string{filepath.Join(dir, "ok.csv"): "a\n", outside: "secret"} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(outside, filepath.Join(dir, "out.csv")); err != nil {
		t.Skipf("no symbolic links here: %v", err)
	}
	if raw, err := ReadConfined(dir, "ok.csv"); err != nil || string(raw) != "a\n" {
		t.Fatalf("ReadConfined(ok.csv) = %q, %v", raw, err)
	}
	for _, name := range []string{"out.csv", "sub", "missing.csv", outside} {
		if raw, err := ReadConfined(dir, name); err == nil {
			t.Errorf("ReadConfined(%q) read %q", name, raw)
		}
		if err := CheckConfined(dir, name); err == nil {
			t.Errorf("CheckConfined(%q) passed", name)
		}
	}
	if err := CheckConfined(dir, "ok.csv"); err != nil {
		t.Errorf("CheckConfined(ok.csv) = %v", err)
	}
}
