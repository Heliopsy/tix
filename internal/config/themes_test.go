// SPDX-License-Identifier: AGPL-3.0-or-later

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// TestThemesLoadFromTheConfigurationFile pins the themes block docs/theming.md
// documents being read at all: it used to be refused as an unknown key, so no
// configured palette ever reached a tenant.
func TestThemesLoadFromTheConfigurationFile(t *testing.T) {
	home := t.TempDir()
	cfg := write(t, filepath.Join(home, "config.yaml"), `
themes:
  acme:
    accent: "#7c3aed"
    accent_soft: "#f3eeff"
    accent_dark: "#c4b5fd"
    accent_soft_dark: "#2a2145"
  plain:
    accent: "#334155"
    accent_soft: "#eef2f6"
`)
	got := mustLoad(t, Options{Dir: t.TempDir(), Home: home,
		Environ: environ(map[string]string{"HOME": home, EnvConfigFile: cfg})})
	reg, err := got.Config.ThemeRegistry()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name string
		want core.Theme
	}{
		{"acme", core.Theme{Name: "acme", Accent: "#7c3aed", AccentSoft: "#f3eeff",
			AccentDark: "#c4b5fd", AccentSoftDark: "#2a2145"}},
		{"plain", core.Theme{Name: "plain", Accent: "#334155", AccentSoft: "#eef2f6",
			AccentDark: "#334155", AccentSoftDark: "#eef2f6"}},
	}
	for _, tc := range tests {
		if found, ok := reg.Lookup(tc.name); !ok || found != tc.want {
			t.Errorf("theme %s = %+v, %v; want %+v", tc.name, found, ok, tc.want)
		}
	}
}

func TestLoadFileRejectsUnknownThemeField(t *testing.T) {
	path := write(t, filepath.Join(t.TempDir(), "config.yaml"), "themes:\n  a:\n    acent: \"#000000\"\n")
	if _, err := LoadFile(path); !core.IsKind(err, core.KindInvalid) || !strings.Contains(err.Error(), "acent") {
		t.Fatalf("err = %v, want invalid naming the field", err)
	}
}

// TestSaveChangesKeepsTheThemesBlock pins a rewrite of the file, as tix ctx
// use makes, leaving the operator's palettes in place.
func TestSaveChangesKeepsTheThemesBlock(t *testing.T) {
	path := write(t, filepath.Join(t.TempDir(), "config.yaml"),
		"themes:\n  acme:\n    accent: \"#7c3aed\"\n    accent_soft: \"#f3eeff\"\n")
	existing, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	before := Defaults()
	after := Defaults()
	after.Tenant = "other"
	if err := SaveChanges(path, existing, &before, &after); err != nil {
		t.Fatal(err)
	}
	again, err := LoadFile(path)
	if err != nil {
		raw, _ := os.ReadFile(path)
		t.Fatalf("reloading %s: %v", raw, err)
	}
	if again.Themes["acme"].Accent != "#7c3aed" || again.Values["tenant"] != "other" {
		t.Errorf("after a save, themes = %+v and tenant = %q", again.Themes, again.Values["tenant"])
	}
}
