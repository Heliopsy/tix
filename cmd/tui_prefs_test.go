// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/output"
	"github.com/heliopsy/tix/internal/tui"
)

// prefsGlobals builds the command layer against an isolated home, which is
// what the settings screen's writer edits.
func prefsGlobals(t *testing.T) (*globals, string) {
	t.Helper()
	c := newCLI(t)
	g := &globals{environ: c.environ(), dir: c.home}
	return g, filepath.Join(c.home, "conf", config.RelativeConfigPath)
}

func TestTheSettingsScreenWritesItsChoicesToTheConfigurationFile(t *testing.T) {
	g, path := prefsGlobals(t)
	save := g.savePreferences()
	err := save(tui.Preferences{
		Keymap: "vim", TimeFormat: output.TimeUS,
		Timezone: "Asia/Tokyo", Color: output.ColorNever,
	})
	if err != nil {
		t.Fatalf("saving preferences: %v", err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the writer created no configuration file at %s: %v", path, err)
	}
	file, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("the written file does not parse: %v\n%s", err, raw)
	}
	want := map[string]string{
		"tui.keymap":         "vim",
		"output.time_format": output.TimeUS,
		"output.timezone":    "Asia/Tokyo",
		"output.color":       output.ColorNever,
	}
	for key, value := range want {
		if got := file.Values[key]; got != value {
			t.Errorf("%s = %q in the written file, want %q\n%s", key, got, value, raw)
		}
	}
}

func TestSavingAPreferenceDoesNotPinEveryOtherKey(t *testing.T) {
	g, path := prefsGlobals(t)
	if err := g.savePreferences()(tui.Preferences{
		Keymap: "helix", TimeFormat: output.TimeISO, Timezone: "local", Color: output.ColorAuto,
	}); err != nil {
		t.Fatalf("saving preferences: %v", err)
	}
	file, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the written file: %v", err)
	}
	// Only the key that moved off its default is written. A writer that saved
	// a whole Config would pin the DSN the environment was supplying, which is
	// the defect `tix tenant use` already had.
	for _, key := range []string{"database.dsn", "server.listen", "auth.mode", "retention.audit"} {
		if value, pinned := file.Values[key]; pinned {
			t.Errorf("saving a display preference pinned %s = %q", key, value)
		}
	}
	if file.Values["tui.keymap"] != "helix" {
		t.Errorf("tui.keymap = %q", file.Values["tui.keymap"])
	}
}

func TestSavingAPreferenceKeepsWhatTheFileAlreadyHeld(t *testing.T) {
	g, path := prefsGlobals(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	existing := "tenant: acme\nproject: infra\nlog:\n    level: debug\n"
	if err := os.WriteFile(path, []byte(existing), 0o600); err != nil {
		t.Fatalf("seeding a config: %v", err)
	}
	if err := g.savePreferences()(tui.Preferences{
		Keymap: "nano", TimeFormat: output.TimeShort, Timezone: "UTC", Color: output.ColorAlways,
	}); err != nil {
		t.Fatalf("saving preferences: %v", err)
	}
	file, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the written file: %v", err)
	}
	for key, value := range map[string]string{
		"tenant": "acme", "project": "infra", "log.level": "debug",
	} {
		if got := file.Values[key]; got != value {
			t.Errorf("%s = %q after saving preferences, want %q", key, got, value)
		}
	}
	if file.Values["tui.keymap"] != "nano" {
		t.Errorf("tui.keymap = %q", file.Values["tui.keymap"])
	}
}

func TestTheSettingsScreenIsToldWhichLayerSuppliedEachPreference(t *testing.T) {
	c := newCLI(t)
	environ := append(c.environ(), "TIX_OUTPUT_TIMEZONE=Asia/Tokyo")
	g := &globals{environ: environ, dir: c.home}
	resolved, err := g.resolve()
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	sources := preferenceSources(resolved)
	if sources.Timezone != string(config.LayerEnv) {
		t.Errorf("timezone source = %q, want %q", sources.Timezone, config.LayerEnv)
	}
	if sources.TimeFormat != string(config.LayerDefault) {
		t.Errorf("time format source = %q, want %q", sources.TimeFormat, config.LayerDefault)
	}
}

func TestTheSessionSectionNeverCarriesASecret(t *testing.T) {
	c := newCLI(t)
	environ := append(c.environ(), "TIX_DATABASE_DSN=postgres://tix:hunter2@db.internal:5432/tix")
	g := &globals{environ: environ, dir: c.home}
	resolved, err := g.resolve()
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	target := redactedTarget(resolved)
	if strings.Contains(target, "hunter2") {
		t.Fatalf("the settings screen would draw a password on screen: %q", target)
	}
	if target == "" {
		t.Fatal("the settings screen would name no target at all")
	}
}

func TestTheSettingsScreenNamesThisBuildWithoutOverflowingItsRow(t *testing.T) {
	got := shortVersion()
	if !strings.HasPrefix(got, "tix ") {
		t.Errorf("version = %q", got)
	}
	if len(got) > 48 {
		t.Errorf("version line is %d characters, which runs off a settings row: %q", len(got), got)
	}
}
