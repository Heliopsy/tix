// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// settingRow returns the rendered line of one setting and nothing else, so an
// assertion about a row cannot pass on text that happens to be elsewhere on
// the screen.
func settingRow(t *testing.T, m Model, row int) string {
	t.Helper()
	for _, l := range SettingsView(m.settingsState()) {
		if l.Row == row && !l.Dim {
			return l.Text
		}
	}
	t.Fatalf("the settings screen renders no row for setting %d", row)
	return ""
}

// settingNote returns the dim note attached to one setting, which is where an
// override warning lands.
func settingNote(t *testing.T, m Model, row int) string {
	t.Helper()
	for _, l := range SettingsView(m.settingsState()) {
		if l.Row == row && l.Dim {
			return l.Text
		}
	}
	return ""
}

// sessionLine returns the session fact with the given label.
func sessionLine(t *testing.T, m Model, label string) string {
	t.Helper()
	for _, f := range SessionFacts(m.sessionInfo()) {
		if f.Label == label {
			return f.Value
		}
	}
	t.Fatalf("the session section has no %q fact", label)
	return ""
}

// settingsModel opens the settings screen on a model with fixed preferences
// and a recording writer, so nothing in these tests depends on the machine's
// own zone or configuration.
func settingsModel(t *testing.T, save PreferenceWriter) Model {
	t.Helper()
	m := New(Config{
		Environ: []string{"NO_COLOR=1"},
		Now:     func() time.Time { return time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC) },
		Prefs: Preferences{
			Keymap: "default", TimeFormat: output.TimeISO,
			Timezone: "UTC", Color: output.ColorNever,
		},
		SavePrefs: save,
		Session: SessionInfo{
			Target: "sqlite:///tmp/scratch.db", Tenant: "default",
			Version: "tix 0.5.0", ConfigFile: "/tmp/scratch/config.yaml",
		},
	})
	m.width, m.height = 120, 40
	return m.openSettings()
}

func TestEverySettingNamesTheConfigurationKeyItIsWrittenTo(t *testing.T) {
	t.Parallel()
	settings := SettingsFor(Preferences{})
	if len(settings) != SettingCount {
		t.Fatalf("the screen offers %d settings, SettingCount says %d", len(settings), SettingCount)
	}
	want := map[int]string{
		SettingKeymap:     KeyTUIKeymap,
		SettingTimeFormat: KeyOutputTimeFormat,
		SettingTimezone:   KeyOutputTimezone,
		SettingColor:      KeyOutputColor,
	}
	for i, set := range settings {
		if set.Key != want[i] {
			t.Errorf("setting %d writes to %q, want %q", i, set.Key, want[i])
		}
		if len(set.Options) < 2 {
			t.Errorf("setting %q offers %d options, so it cannot be changed", set.Key, len(set.Options))
		}
		if set.Label == "" {
			t.Errorf("setting %q has no label", set.Key)
		}
	}
}

func TestCyclingAValueWrapsAtBothEnds(t *testing.T) {
	t.Parallel()
	options := []string{"a", "b", "c"}
	cases := []struct {
		current string
		delta   int
		want    string
	}{
		{"a", 1, "b"},
		{"c", 1, "a"},
		{"a", -1, "c"},
		{"b", -1, "a"},
		{"unknown", 1, "b"},
	}
	for _, tc := range cases {
		if got := CycleValue(options, tc.current, tc.delta); got != tc.want {
			t.Errorf("CycleValue(%q, %d) = %q, want %q", tc.current, tc.delta, got, tc.want)
		}
	}
	if got := CycleValue(nil, "a", 1); got != "a" {
		t.Errorf("cycling an empty list changed the value to %q", got)
	}
}

func TestAConfiguredValueThisBuildDoesNotShipIsStillOffered(t *testing.T) {
	t.Parallel()
	set := SettingsFor(Preferences{Timezone: "Antarctica/Troll"})[SettingTimezone]
	found := false
	for _, o := range set.Options {
		if o == "Antarctica/Troll" {
			found = true
		}
	}
	if !found {
		t.Fatalf("a configured zone was cycled away rather than kept: %v", set.Options)
	}
}

func TestTheTimezoneRowShowsTheExampleInThatZone(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.settingSel = SettingTimezone
	if row := settingRow(t, m, SettingTimezone); !strings.Contains(row, "2026-09-21 14:05") {
		t.Fatalf("the timezone row does not render the example in UTC: %q", row)
	}
	m.prefs.Timezone = "Asia/Tokyo"
	// The same instant, nine hours on. A row that renders the example through
	// anything but the configured zone cannot produce this.
	if row := settingRow(t, m, SettingTimezone); !strings.Contains(row, "2026-09-21 23:05") {
		t.Fatalf("the timezone row does not follow the zone: %q", row)
	}
}

func TestTheTimeFormatRowShowsTheExampleInThatLayout(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	if row := settingRow(t, m, SettingTimeFormat); !strings.Contains(row, "2026-09-21 14:05") {
		t.Fatalf("the iso row does not render an iso example: %q", row)
	}
	m.prefs.TimeFormat = output.TimeUS
	if row := settingRow(t, m, SettingTimeFormat); !strings.Contains(row, "09/21/2026 2:05 PM") {
		t.Fatalf("the time format row does not follow the layout: %q", row)
	}
}

func TestChangingTheTimeFormatRestylesEveryTimestamp(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.settingSel = SettingTimeFormat
	m, _ = m.reduce(pressKey("right"))
	if m.prefs.TimeFormat == output.TimeISO {
		t.Fatal("cycling the time format did not change it")
	}
	if got := m.timeStyle.Format(SettingsExample); got != "2026-09-21T14:05:09Z" {
		t.Fatalf("the model's time style is still the old layout: %q", got)
	}
}

func TestAChangedPreferenceIsWrittenDownAtOnce(t *testing.T) {
	t.Parallel()
	var saved []Preferences
	m := settingsModel(t, func(p Preferences) error {
		saved = append(saved, p)
		return nil
	})
	m, _ = m.reduce(pressKey("right"))
	if len(saved) != 1 {
		t.Fatalf("a change wrote %d times, want 1", len(saved))
	}
	if saved[0].Keymap != string(SchemeVim) {
		t.Fatalf("the writer was handed keymap %q", saved[0].Keymap)
	}
	if !strings.Contains(m.status, "/tmp/scratch/config.yaml") {
		t.Fatalf("the status does not say where the change went: %q", m.status)
	}
}

func TestASessionWithNowhereToWriteSaysTheChoiceIsTemporary(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m, _ = m.reduce(pressKey("right"))
	if m.scheme != SchemeVim {
		t.Fatalf("the change did not apply: %q", m.scheme)
	}
	if !strings.Contains(m.status, "for this session") {
		t.Fatalf("the status does not admit the change is temporary: %q", m.status)
	}
	lines := SettingsView(m.settingsState())
	if !containsLine(lines, "last until you quit") {
		t.Fatal("the screen does not warn that nothing can be written down")
	}
}

func TestAFailedSaveIsReportedRatherThanSwallowed(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, func(Preferences) error { return errors.New("read-only file system") })
	m, _ = m.reduce(pressKey("right"))
	if !strings.Contains(m.status, "not saved") || !strings.Contains(m.status, "read-only file system") {
		t.Fatalf("a failed write was not reported: %q", m.status)
	}
}

func TestAValueTheBuildCannotRenderIsRefusedAndNothingChanges(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	set := SettingsFor(m.prefs)[SettingTimezone]
	next := m.usePreferences(m.prefs.With(SettingTimezone, "Mars/Olympus"), set, "Mars/Olympus")
	if next.prefs.Timezone != "UTC" {
		t.Fatalf("an unresolvable zone was adopted: %q", next.prefs.Timezone)
	}
	if !strings.Contains(next.err, "Mars/Olympus") {
		t.Fatalf("the refusal does not name the value: %q", next.err)
	}
}

func TestALayerAboveTheFileIsAnnouncedOnItsOwnRow(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.prefSources = Preferences{Timezone: "environment"}
	if note := settingNote(t, m, SettingTimezone); !strings.Contains(note, "environment layer supplies this") {
		t.Fatalf("the timezone row does not warn about the environment: %q", note)
	}
	if note := settingNote(t, m, SettingTimeFormat); note != "" {
		t.Fatalf("a row with no override carries a warning: %q", note)
	}
}

func TestOverrideNoteIsSilentForTheLayersAFileCanBeat(t *testing.T) {
	t.Parallel()
	for _, layer := range []string{"", "file", "default"} {
		if note := OverrideNote(layer); note != "" {
			t.Errorf("layer %q warned %q", layer, note)
		}
	}
	for _, layer := range []string{"flag", "environment", "dotenv"} {
		if note := OverrideNote(layer); note == "" {
			t.Errorf("layer %q did not warn", layer)
		}
	}
}

func TestTheSessionSectionAnswersWhatThisRunIsConnectedTo(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.tenantKey = "acme"
	m.actor = &core.Actor{Handle: "nadia"}
	if got := sessionLine(t, m, "target"); got != "sqlite:///tmp/scratch.db" {
		t.Errorf("target = %q", got)
	}
	if got := sessionLine(t, m, "tenant"); got != "acme" {
		t.Errorf("tenant = %q, want the tenant this session switched to", got)
	}
	if got := sessionLine(t, m, "actor"); got != "@nadia" {
		t.Errorf("actor = %q", got)
	}
	if got := sessionLine(t, m, "version"); got != "tix 0.5.0" {
		t.Errorf("version = %q", got)
	}
	if got := sessionLine(t, m, "config"); got != "/tmp/scratch/config.yaml" {
		t.Errorf("config = %q", got)
	}
}

func TestAFactThisSessionWasNeverToldSaysSoRatherThanRenderingBlank(t *testing.T) {
	t.Parallel()
	for _, f := range SessionFacts(SessionInfo{}) {
		if strings.TrimSpace(f.Value) == "" {
			t.Errorf("%q renders blank, which reads as a value that failed to load", f.Label)
		}
	}
}

func TestChangingTheColourModeRedrawsTheFrame(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.autoColor = true
	m.settingSel = SettingColor
	set := SettingsFor(m.prefs)[SettingColor]
	m = m.usePreferences(m.prefs.With(SettingColor, output.ColorAlways), set, output.ColorAlways)
	if !m.theme.Color {
		t.Fatal("asking for colour always left the theme monochrome")
	}
	m = m.usePreferences(m.prefs.With(SettingColor, output.ColorNever), set, output.ColorNever)
	if m.theme.Color {
		t.Fatal("asking for no colour left the theme coloured")
	}
}

func TestColorForResolvesAutoAgainstTheProbe(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode string
		auto bool
		want bool
	}{
		{output.ColorAuto, true, true},
		{output.ColorAuto, false, false},
		{output.ColorAlways, false, true},
		{output.ColorNever, true, false},
		{"", true, true},
		{"nonsense", true, true},
	}
	for _, tc := range cases {
		if got := ColorFor(tc.mode, tc.auto); got != tc.want {
			t.Errorf("ColorFor(%q, %v) = %v, want %v", tc.mode, tc.auto, got, tc.want)
		}
	}
}

func TestTheSettingsFooterDescribesSettingsRatherThanColumns(t *testing.T) {
	t.Parallel()
	short := DefaultKeyMap().ShortHelp(viewSettings, ActionContext{})
	var descs []string
	for _, e := range short {
		descs = append(descs, e.Desc)
	}
	joined := strings.Join(descs, ", ")
	if !strings.Contains(joined, "next value") || !strings.Contains(joined, "previous value") {
		t.Fatalf("the settings footer does not offer the value keys: %q", joined)
	}
	if strings.Contains(joined, "column") {
		t.Fatalf("the settings footer promises columns on a screen with none: %q", joined)
	}
}

func TestTheSettingsScreenFitsAShortTerminal(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.width, m.height = 80, 24
	layout := LayoutFor(m.width, m.height, 0)
	lines := m.settingsLines(layout)
	if len(lines) > layout.BodyHeight {
		t.Fatalf("the settings body drew %d lines into a body of %d", len(lines), layout.BodyHeight)
	}
	if !strings.Contains(strings.Join(lines, "\n"), "more") {
		t.Fatal("a body that does not fit says nothing about what is off screen")
	}
}

func TestTheSettingsCursorStaysOnEverySetting(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	for i := range SettingCount {
		if m.settingSel != i {
			t.Fatalf("walking down landed on %d, want %d", m.settingSel, i)
		}
		for j := range SettingCount {
			row := settingRow(t, m, j)
			marked := strings.HasPrefix(row, "  "+SelectionMarker(true))
			if marked != (j == i) {
				t.Fatalf("with setting %d selected, row %d marked = %v: %q", i, j, marked, row)
			}
		}
		m, _ = m.reduce(pressKey("down"))
	}
	if m.settingSel != SettingCount-1 {
		t.Fatalf("walking past the last setting moved to %d", m.settingSel)
	}
	m, _ = m.reduce(pressKey("up"))
	if m.settingSel != SettingCount-2 {
		t.Fatalf("walking up landed on %d", m.settingSel)
	}
}

func TestTheSettingsScreenPointsAtTheCommandLineForConfiguration(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	lines := SettingsView(m.settingsState())
	if !containsLine(lines, "tix config show --sources") {
		t.Fatal("the screen does not say where the rest of the configuration lives")
	}
}

// containsLine reports whether any rendered line carries want.
func containsLine(lines []SettingsLine, want string) bool {
	for _, l := range lines {
		if strings.Contains(l.Text, want) {
			return true
		}
	}
	return false
}

func TestTheSessionFactsAreReachableOnATerminalTooShortToShowThem(t *testing.T) {
	t.Parallel()
	m := settingsModel(t, nil)
	m.width, m.height = 100, 24
	lines, rows := m.settingsBody()
	if rows >= len(lines) {
		t.Fatalf("the whole screen fits in %d rows, so this test proves nothing", rows)
	}
	// The last line of the screen is the last thing anybody has to be able to
	// reach; walking down must eventually put it in the window.
	for range len(lines) + SettingCount {
		m.settingSel, m.settingsOff = m.moveSettings(1)
	}
	window, offset := SettingsWindow(lines, m.settingsOff, rows)
	if offset+len(window) != len(lines) {
		t.Fatalf("walking down stopped at line %d of %d", offset+len(window), len(lines))
	}
	if !containsLine(window, "tix config show --sources") {
		t.Fatal("the bottom of the screen is unreachable")
	}
	for range len(lines) + SettingCount {
		m.settingSel, m.settingsOff = m.moveSettings(-1)
	}
	if m.settingsOff != 0 || m.settingSel != 0 {
		t.Fatalf("walking back up landed on setting %d at offset %d", m.settingSel, m.settingsOff)
	}
}

func TestMoveSettingsScrollsOnlyOnceTheCursorHasNowhereToGo(t *testing.T) {
	t.Parallel()
	lines := SettingsView(SettingsState{Now: time.Now()})
	rows := len(lines)
	sel, off := MoveSettings(lines, 0, 0, 1, rows)
	if sel != 1 || off != 0 {
		t.Fatalf("a move with room left gave setting %d at offset %d, want 1 at 0", sel, off)
	}
	sel, off = MoveSettings(lines, SettingCount-1, 0, 1, rows)
	if sel != SettingCount-1 {
		t.Fatalf("a move past the last setting moved the cursor to %d", sel)
	}
	if off != 0 {
		t.Fatalf("a screen that already fits scrolled to %d", off)
	}
}
