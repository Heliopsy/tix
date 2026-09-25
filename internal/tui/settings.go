// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// Preferences are the display settings a terminal reader owns. They are the
// reader's own configuration keys rather than a store of their own: a browser
// has only a cookie to keep a preference in, but somebody at a terminal
// already has a configuration file, and a second place to write a timezone
// down would be a second place to disagree with the command line.
type Preferences struct {
	Keymap     string
	TimeFormat string
	Timezone   string
	Color      string
}

// PreferenceWriter persists a whole set of preferences. It is supplied by the
// command layer, which owns the configuration file; a nil writer is a session
// that has nowhere to write, such as a hosted SSH sandbox, and the screen says
// so rather than offering a choice that will not survive the run.
type PreferenceWriter func(Preferences) error

// SessionInfo answers "what am I connected to", which is the other half of
// what somebody opens a settings screen for. Every field is read-only here:
// changing a target from inside a session already pinned to it is a trap, and
// the command line already owns that decision.
type SessionInfo struct {
	Target     string
	Tenant     string
	Actor      string
	Version    string
	ConfigFile string
}

// Configuration keys each preference is written to. They are the CLI's own
// keys, so a format chosen here is the format `tix task ls` prints.
const (
	KeyTUIKeymap        = "tui.keymap"
	KeyOutputTimeFormat = "output.time_format"
	KeyOutputTimezone   = "output.timezone"
	KeyOutputColor      = "output.color"
)

// Setting is one preference the screen can change.
type Setting struct {
	Label   string
	Key     string
	Options []string
}

// Indexes of the settings the screen offers, in the order it offers them.
const (
	SettingKeymap = iota
	SettingTimeFormat
	SettingTimezone
	SettingColor
	SettingCount
)

// Timezones are the zones the screen cycles through. It is a selection rather
// than every name the zone database carries, for the same reason the browser
// offers a selection: six hundred entries cycled one key press at a time is
// not a control anybody can use. A zone already configured is added to the
// list, so a name chosen outside the interface is never cycled away and lost.
var Timezones = []string{
	"local", "UTC",
	"Pacific/Auckland", "Australia/Sydney", "Australia/Perth",
	"Asia/Tokyo", "Asia/Shanghai", "Asia/Singapore", "Asia/Bangkok",
	"Asia/Kolkata", "Asia/Karachi", "Asia/Dubai", "Asia/Jerusalem",
	"Africa/Cairo", "Africa/Johannesburg", "Africa/Lagos",
	"Europe/Moscow", "Europe/Sofia", "Europe/Berlin", "Europe/Madrid",
	"Europe/London", "America/Sao_Paulo", "America/New_York",
	"America/Chicago", "America/Denver", "America/Los_Angeles",
}

// SettingsFor describes the settings on offer, with the value currently in
// force folded into each list. A value this build does not ship is kept rather
// than dropped: cycling past an unknown scheme or zone would silently rewrite
// a configuration file the reader only meant to look at.
func SettingsFor(p Preferences) []Setting {
	return []Setting{
		{Label: "keys", Key: KeyTUIKeymap, Options: withCurrent(schemeOptions(), p.Keymap, SchemeDefault.String())},
		{Label: "time format", Key: KeyOutputTimeFormat, Options: withCurrent(output.TimeFormats, p.TimeFormat, output.TimeISO)},
		{Label: "timezone", Key: KeyOutputTimezone, Options: withCurrent(Timezones, p.Timezone, "local")},
		{Label: "colour", Key: KeyOutputColor, Options: withCurrent(output.ColorModes, p.Color, output.ColorAuto)},
	}
}

// String renders a scheme name.
func (s Scheme) String() string { return string(s) }

// schemeOptions lists the shipped keybinding schemes as option strings.
func schemeOptions() []string {
	out := make([]string, 0, len(Schemes()))
	for _, s := range Schemes() {
		out = append(out, string(s))
	}
	return out
}

// withCurrent returns the options with the value in force guaranteed present,
// treating an empty value as the given fallback.
func withCurrent(options []string, current, fallback string) []string {
	want := strings.TrimSpace(current)
	if want == "" {
		want = fallback
	}
	out := append([]string(nil), options...)
	for _, o := range out {
		if strings.EqualFold(o, want) {
			return out
		}
	}
	return append(out, want)
}

// Value reads the preference at index i.
func (p Preferences) Value(i int) string {
	switch i {
	case SettingKeymap:
		return orDefault(p.Keymap, SchemeDefault.String())
	case SettingTimeFormat:
		return orDefault(p.TimeFormat, output.TimeISO)
	case SettingTimezone:
		return orDefault(p.Timezone, "local")
	case SettingColor:
		return orDefault(p.Color, output.ColorAuto)
	default:
		return ""
	}
}

// With returns a copy carrying value at index i.
func (p Preferences) With(i int, value string) Preferences {
	switch i {
	case SettingKeymap:
		p.Keymap = value
	case SettingTimeFormat:
		p.TimeFormat = value
	case SettingTimezone:
		p.Timezone = value
	case SettingColor:
		p.Color = value
	}
	return p
}

// orDefault substitutes a fallback for an unset value.
func orDefault(value, fallback string) string {
	if trimmed := strings.TrimSpace(value); trimmed != "" {
		return trimmed
	}
	return fallback
}

// CycleValue steps through options from the value in force, wrapping at both
// ends so a list can be walked in either direction without counting.
func CycleValue(options []string, current string, delta int) string {
	if len(options) == 0 {
		return current
	}
	at := 0
	for i, o := range options {
		if strings.EqualFold(o, strings.TrimSpace(current)) {
			at = i
			break
		}
	}
	next := (at + delta) % len(options)
	if next < 0 {
		next += len(options)
	}
	return options[next]
}

// SettingsExample is the instant every absolute layout is demonstrated with,
// so an example is the same on every machine and in every zone.
var SettingsExample = time.Date(2026, time.September, 21, 14, 5, 9, 0, time.UTC)

// OptionDescription says what choosing an option would mean, rendered by the
// thing that will actually render it. An example produced any other way can
// claim a layout the interface does not draw.
func OptionDescription(i int, value string, p Preferences, now time.Time, colorAuto bool) string {
	switch i {
	case SettingKeymap:
		return SchemeDescription(Scheme(strings.ToLower(value)))
	case SettingTimeFormat:
		return timeExample(value, p.Timezone, now)
	case SettingTimezone:
		return timeExample(p.TimeFormat, value, now)
	case SettingColor:
		return colorDescription(value, colorAuto)
	default:
		return ""
	}
}

// timeExample renders the sample instant under one format and zone, saying so
// when the pair is one this build cannot resolve rather than showing nothing.
func timeExample(format, zone string, now time.Time) string {
	style, err := output.NewTimeStyle(orDefault(format, output.TimeISO), orDefault(zone, "local"))
	if err != nil {
		return "not available on this machine"
	}
	if strings.EqualFold(strings.TrimSpace(format), output.TimeRelative) {
		return style.Format(now.Add(-18 * time.Minute))
	}
	return style.Format(SettingsExample)
}

// colorDescription says what a colour mode does here, resolving auto against
// this terminal so the reader is told what auto currently decides rather than
// what it decides in general.
func colorDescription(value string, colorAuto bool) string {
	switch strings.TrimSpace(value) {
	case output.ColorAlways:
		return "always, whatever the output is"
	case output.ColorNever:
		return "never; monochrome, as NO_COLOR asks for"
	default:
		if colorAuto {
			return "auto: this terminal accepts colour"
		}
		return "auto: this terminal is getting none"
	}
}

// OverrideNote warns that a layer above the file is supplying this value, so
// what is written here will not be what the next run uses. Saving a key the
// environment also sets, and then watching nothing change on restart, is the
// whole reason this line exists.
func OverrideNote(layer string) string {
	switch strings.TrimSpace(layer) {
	case "", "file", "default":
		return ""
	default:
		return "the " + strings.TrimSpace(layer) + " layer supplies this, and still wins after a restart"
	}
}

// SaveNote is what the status bar says after a preference changed.
func SaveNote(setting Setting, value, path string, err error, persistent bool) string {
	switch {
	case err != nil:
		return setting.Key + " = " + value + ", not saved: " + err.Error()
	case !persistent:
		return setting.Key + " = " + value + " for this session; nothing here can write a configuration file"
	case path == "":
		return setting.Key + " = " + value + ", saved"
	default:
		return setting.Key + " = " + value + ", saved to " + path
	}
}

// SettingsLine is one rendered line of the settings screen, carrying how it
// should be drawn rather than the drawing itself, so the layout is testable
// without a terminal.
type SettingsLine struct {
	Text string
	// Row is the setting the line belongs to, or -1 for a line that is not
	// part of any setting and cannot be selected.
	Row     int
	Heading bool
	Dim     bool
}

// SettingsState is everything the settings screen renders from.
type SettingsState struct {
	Prefs   Preferences
	Sources Preferences
	Session SessionInfo
	// Selected indexes the setting the cursor is on.
	Selected int
	// Persistent reports whether a change can be written down.
	Persistent bool
	// ColorAuto is what the auto colour mode decides for this run.
	ColorAuto bool
	Now       time.Time
}

// sourceOf names the layer that supplied the setting at index i.
func (s SettingsState) sourceOf(i int) string {
	switch i {
	case SettingKeymap:
		return s.Sources.Keymap
	case SettingTimeFormat:
		return s.Sources.TimeFormat
	case SettingTimezone:
		return s.Sources.Timezone
	case SettingColor:
		return s.Sources.Color
	default:
		return ""
	}
}

// SettingsView renders the whole settings screen.
func SettingsView(s SettingsState) []SettingsLine {
	lines := []SettingsLine{{Text: "display", Row: -1, Heading: true}, {Text: "", Row: -1}}
	settings := SettingsFor(s.Prefs)
	for i, set := range settings {
		value := s.Prefs.Value(i)
		// The value is bracketed with plain angles rather than the arrow glyph
		// the cursor uses, so nothing but the cursor draws a cursor.
		label := "  " + SelectionMarker(i == s.Selected) + pad(set.Label, 13) + pad("< "+value+" >", 24) +
			OptionDescription(i, value, s.Prefs, s.Now, s.ColorAuto)
		lines = append(lines, SettingsLine{Text: label, Row: i})
		if note := OverrideNote(s.sourceOf(i)); note != "" {
			lines = append(lines, SettingsLine{Text: "      " + set.Key + ": " + note, Row: i, Dim: true})
		}
	}
	lines = append(lines, SettingsLine{Text: "", Row: -1})
	lines = append(lines, SettingsLine{Text: "  " + persistenceNote(s), Row: -1, Dim: true})
	// The preview is always drawn rather than only under the keys row: its
	// height decides where the screen can scroll to, and a body that grows and
	// shrinks with the cursor moves the session facts out from under it.
	lines = append(lines, SettingsLine{Text: "", Row: -1})
	lines = append(lines, keyPreview(s.Prefs.Keymap)...)
	lines = append(lines, SettingsLine{Text: "", Row: -1})
	lines = append(lines, SettingsLine{Text: "session", Row: -1, Heading: true})
	lines = append(lines, SettingsLine{Text: "", Row: -1})
	for _, f := range SessionFacts(s.Session) {
		lines = append(lines, SettingsLine{Text: "  " + pad(f.Label, 13) + f.Value, Row: -1})
	}
	lines = append(lines, SettingsLine{Text: "", Row: -1})
	for _, l := range elsewhereNote() {
		lines = append(lines, SettingsLine{Text: "  " + l, Row: -1, Dim: true})
	}
	return lines
}

// persistenceNote says where a change goes, which is the one thing a reader
// has to know before touching any of these.
func persistenceNote(s SettingsState) string {
	if !s.Persistent {
		return "This session cannot write a configuration file, so these last until you quit."
	}
	if s.Session.ConfigFile == "" {
		return "Every change is written to your configuration file at once."
	}
	return "Every change is written to " + s.Session.ConfigFile + " at once."
}

// keyPreview shows what the scheme in force binds, for the actions people
// reach for most often. A name this build does not ship previews the default
// bindings, which are the ones such a session is actually running.
func keyPreview(scheme string) []SettingsLine {
	parsed, _ := ParseScheme(scheme)
	out := []SettingsLine{{Text: "  the " + string(parsed) + " keys bind:", Row: -1, Dim: true}}
	preview := KeyMapFor(parsed)
	for _, action := range []string{"Up", "Down", "New", "EditTitle", "Comment", "Filter", "Back"} {
		b, ok := preview.Binding(action)
		if !ok {
			continue
		}
		out = append(out, SettingsLine{Text: "    " + pad(strings.Join(b.Keys(), ", "), 22) + b.Help().Desc, Row: -1})
	}
	return out
}

// Fact is one read-only line of the session section.
type Fact struct {
	Label string
	Value string
}

// SessionFacts states what this run is connected to. A field the caller could
// not supply says so, because a blank value reads as one that failed to load.
func SessionFacts(info SessionInfo) []Fact {
	return []Fact{
		{Label: "target", Value: orUnset(info.Target)},
		{Label: "tenant", Value: orUnset(info.Tenant)},
		{Label: "actor", Value: orUnset(info.Actor)},
		{Label: "version", Value: orUnset(info.Version)},
		{Label: "config", Value: orNoFile(info.ConfigFile)},
	}
}

// orUnset names a fact this session was never told.
func orUnset(value string) string {
	if strings.TrimSpace(value) == "" {
		return "not named by this session"
	}
	return value
}

// orNoFile names a configuration file that does not exist yet, which is not
// the same as one nobody told this session about.
func orNoFile(path string) string {
	if strings.TrimSpace(path) == "" {
		return "none; a change here creates one"
	}
	return path
}

// elsewhereNote points at the command line for everything this screen does
// not offer. The target, the token, the log and the retention windows are
// deployment configuration rather than reading preferences, and editing a DSN
// inside a session already connected through it would apply to nothing.
func elsewhereNote() []string {
	return []string{
		"Only what changes how this interface reads is here. The target, the",
		"credentials, the log and the retention windows are configuration, not",
		"preferences, and a session already connected cannot act on a new one.",
		"",
		"tix config show --sources   every key, and the layer it came from",
		"tix tenant use KEY          write this session's tenant down",
	}
}

// SettingsLineIndex is the line the cursor sits on, which is what a scrolling
// window has to keep on screen.
func SettingsLineIndex(lines []SettingsLine, row int) int {
	for i, l := range lines {
		if l.Row == row {
			return i
		}
	}
	return 0
}

// MoveSettings applies one step of the cursor. The cursor walks the settings
// while there is another one to reach; at either end, and whenever the selected
// setting has been scrolled out of the window, the same key scrolls the body
// instead. Without that the session facts below the settings were unreachable
// on a terminal too short to show them: four settings meant four presses and
// nothing moved after the fourth.
func MoveSettings(lines []SettingsLine, sel, off, delta, rows int) (int, int) {
	limit := max(0, len(lines)-rows)
	at := SettingsLineIndex(lines, sel)
	offScreen := at < off || at >= off+rows
	next := sel + delta
	if offScreen || next < 0 || next >= SettingCount {
		return sel, clamp(off+delta, 0, limit)
	}
	return next, ScrollWindow(off, SettingsLineIndex(lines, next), rows, len(lines))
}

// SettingsWindow is the slice of rendered lines a body of the given height
// shows, with the offset it settled on.
func SettingsWindow(lines []SettingsLine, off, rows int) ([]SettingsLine, int) {
	if rows <= 0 || len(lines) == 0 {
		return nil, 0
	}
	off = clamp(off, 0, max(0, len(lines)-rows))
	end := min(off+rows, len(lines))
	return lines[off:end], off
}

// ApplyPreference resolves a new preference value into the renderers it
// governs, refusing a value the build cannot render rather than adopting it.
// It is the whole effect of a change, kept out of the model so the rule can be
// tested without a terminal.
func ApplyPreference(p Preferences, overrides map[string]string) (KeyMap, output.TimeStyle, error) {
	parsed, err := ParseScheme(p.Keymap)
	if err != nil {
		return DefaultKeyMap(), output.TimeStyle{}, err
	}
	keys, err := KeyMapFrom(parsed, overrides)
	if err != nil {
		return DefaultKeyMap(), output.TimeStyle{}, err
	}
	style, err := output.NewTimeStyle(orDefault(p.TimeFormat, output.TimeISO), orDefault(p.Timezone, "local"))
	if err != nil {
		return keys, output.TimeStyle{}, core.Invalid("%s", err.Error())
	}
	return keys, style, nil
}

// ColorFor resolves a colour mode against what this run's terminal would take
// on its own. An unknown mode reads as auto, which is what the configuration
// layer refuses before a run ever starts.
func ColorFor(mode string, auto bool) bool {
	parsed, ok := output.ParseMode(mode)
	if !ok {
		return auto
	}
	switch parsed {
	case output.ModeAlways:
		return true
	case output.ModeNever:
		return false
	default:
		return auto
	}
}
