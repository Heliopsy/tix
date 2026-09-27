// SPDX-License-Identifier: AGPL-3.0-or-later

package cmd

import (
	"os"
	"path/filepath"
	"slices"
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

// TestTheMotionPreferenceReachesTheInterface is the probe the shadow table
// stands for: tui.motion is resolved, and the value the command hands the
// terminal interface is read off the preferences struct rather than off the
// configuration behind it. A key nothing carries through resolves fine and
// changes nothing, which is how two keys in this repo did nothing at all.
func TestTheMotionPreferenceReachesTheInterface(t *testing.T) {
	cases := []struct {
		name    string
		file    string
		environ []string
		want    string
	}{
		// The shipped value spelled out rather than read back from the
		// constant, which would pass whatever the constant said.
		{"nothing configured", "", nil, tui.MotionOn},
		{"the file layer", "tui:\n  motion: off\n", nil, tui.MotionOff},
		{"the environment beats the file", "tui:\n  motion: on\n",
			[]string{"TIX_TUI_MOTION=off"}, tui.MotionOff},
		{"the dotenv layer beats the file", "tui:\n  motion: on\n", nil, tui.MotionOff},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newCLI(t)
			if tc.file != "" {
				writeUserConfig(t, c, tc.file)
			}
			if tc.name == "the dotenv layer beats the file" {
				path := filepath.Join(c.home, ".env")
				if err := os.WriteFile(path, []byte("TIX_TUI_MOTION=off\n"), 0o600); err != nil {
					t.Fatalf("writing %s: %v", path, err)
				}
			}
			g := &globals{environ: append(c.environ(), tc.environ...), dir: c.home}
			resolved, err := g.resolve()
			if err != nil {
				t.Fatalf("resolving: %v", err)
			}
			if got := tuiPreferences(resolved, "default").Motion; got != tc.want {
				t.Errorf("the interface is handed motion = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestTheSettingsScreenWritesTheMotionPreferenceDown(t *testing.T) {
	g, path := prefsGlobals(t)
	if err := g.savePreferences()(tui.Preferences{
		Keymap: "default", TimeFormat: output.TimeISO, Timezone: "UTC",
		Color: output.ColorAuto, Motion: tui.MotionOff,
	}); err != nil {
		t.Fatalf("saving preferences: %v", err)
	}
	file, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("loading the written file: %v", err)
	}
	if got := file.Values["tui.motion"]; got != tui.MotionOff {
		t.Errorf("tui.motion = %q in the written file, want %q", got, tui.MotionOff)
	}
}

// TestTheSettingsScreenIsToldWhichLayerSuppliedTheMotionPreference keeps the
// fifth row working like the other four: a value the environment supplies
// carries the note saying so.
func TestTheSettingsScreenIsToldWhichLayerSuppliedTheMotionPreference(t *testing.T) {
	c := newCLI(t)
	g := &globals{environ: append(c.environ(), "TIX_TUI_MOTION=off"), dir: c.home}
	resolved, err := g.resolve()
	if err != nil {
		t.Fatalf("resolving: %v", err)
	}
	if got := preferenceSources(resolved).Motion; got != string(config.LayerEnv) {
		t.Errorf("motion source = %q, want %q", got, config.LayerEnv)
	}
}

// TestTheMotionValuesAreSpelledTheSameOnBothSides holds the two copies of the
// enumeration against each other. internal/config cannot import internal/tui,
// so the strings are written down twice and only this layer sees both.
func TestTheMotionValuesAreSpelledTheSameOnBothSides(t *testing.T) {
	want := []string{tui.MotionOn, tui.MotionOff}
	if got := config.TUIMotions; !slices.Equal(got, want) {
		t.Errorf("config accepts %v for tui.motion, the interface offers %v", got, want)
	}
	if config.DefaultTUIMotion != tui.MotionOn {
		t.Errorf("the shipped tui.motion is %q, want the interface's %q",
			config.DefaultTUIMotion, tui.MotionOn)
	}
}

// TestAnUnknownMotionValueIsRefusedAtStartup keeps the key from accepting
// anything that is not one of its two values. An enumeration nothing validates
// takes "yes" and then quietly behaves as though it were on.
func TestAnUnknownMotionValueIsRefusedAtStartup(t *testing.T) {
	c := newCLI(t)
	writeUserConfig(t, c, "tui:\n  motion: sometimes\n")
	g := &globals{environ: c.environ(), dir: c.home}
	_, err := g.resolve()
	if err == nil {
		t.Fatal("tui.motion accepted a value that is not one it offers")
	}
	if !strings.Contains(err.Error(), "tui.motion") {
		t.Errorf("the refusal does not name the key: %v", err)
	}
}
