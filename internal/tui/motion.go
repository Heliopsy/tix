// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// The values the motion preference takes. They are the strings config writes
// to tui.motion; a test holds the two spellings against each other.
const (
	MotionOn  = "on"
	MotionOff = "off"
)

// MotionModes are the values the motion row steps through.
var MotionModes = []string{MotionOn, MotionOff}

// PulseInterval is half a pulse cycle: the selected row changes emphasis this
// often, so one whole cycle takes twice as long.
//
// The period is a bandwidth figure as much as a visual one. `tix ssh` renders
// server side and sends the frame down the wire, so every phase is a repaint
// on somebody's connection. At 800ms nothing faster than a slow breath is ever
// drawn, which is both what a selection deserves and a little over one frame a
// second; under about half a second the same thing reads as a blink and pulls
// attention away from the work.
const PulseInterval = 800 * time.Millisecond

// MotionIdleAfter is how long the interface goes without a keystroke before
// the pulse stops altogether. An idle session sends nothing: a public demo
// host holding twenty abandoned sessions must not be repainting all of them
// for ever.
//
// Forty-five seconds is longer than it takes to read a task through, so the
// motion does not stop while somebody is still looking at the screen, and it
// is short enough that an abandoned session goes quiet inside a minute.
// Stopping is not a failure state either way: the pulse settles on full
// emphasis, which is exactly what a reader with motion off already sees.
const MotionIdleAfter = 45 * time.Second

// MotionEnabled reports whether the pulse may run at all.
//
// Colour is the other half of the condition rather than a nicety. Without it
// every style in the theme is the same empty style, so a pulse would repaint
// frame after identical frame: NO_COLOR, TIX_NO_COLOR, output.color = never
// and a destination that is not a terminal all degrade to a static selection
// instead of to invisible motion on a busy wire.
func MotionEnabled(mode string, color bool) bool {
	if !color {
		return false
	}
	return !strings.EqualFold(strings.TrimSpace(mode), MotionOff)
}

// PulseRunning reports whether another phase of the pulse is due, given how
// long it has been since the reader last pressed a key. It is the whole of the
// idle pause: a false answer schedules nothing, and nothing then produces a
// frame until the next keystroke.
func PulseRunning(mode string, color bool, idle time.Duration) bool {
	return MotionEnabled(mode, color) && idle < MotionIdleAfter
}

// MotionDescription says what a motion mode does on this terminal, resolving
// "on" against whether this run draws in colour at all, so a reader whose
// terminal is getting none is told the selection will stay still rather than
// told the pulse is running.
func MotionDescription(mode string, color bool) string {
	if strings.EqualFold(strings.TrimSpace(mode), MotionOff) {
		return "off; the selected row is drawn the same in every frame"
	}
	if !color {
		return "on, but this terminal is getting no colour, so nothing pulses"
	}
	return "on; the selected row pulses, and rests after " + MotionIdleAfter.String() + " without a key"
}

// pulseMsg carries the next phase of the selection's pulse.
type pulseMsg struct{}

// pulse schedules one phase of the pulse. Nothing else produces it, so a model
// that stops returning this command stops producing frames.
func pulse() tea.Cmd {
	return tea.Tick(PulseInterval, func(time.Time) tea.Msg { return pulseMsg{} })
}
