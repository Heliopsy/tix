// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"io"
	"slices"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/output"
)

// animatedModel is a board on a client whose terminal takes colour, with a
// clock the test moves, because the idle pause is measured against it.
func animatedModel(t *testing.T) (Model, *clock.Fake) {
	t.Helper()
	clk := clock.NewFakeAt()
	m := New(Config{
		Access: fullAccess(), Actor: fullActor(),
		Environ:  []string{"TERM=xterm-256color"},
		Renderer: NewRenderer([]string{"TERM=xterm-256color"}, io.Discard),
		Color:    boolPtr(true),
		Now:      clk.Now,
	})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks: []core.Task{
			task("a", "todo", 1, core.PriorityNormal),
			task("b", "todo", 2, core.PriorityNormal),
		},
	})
	if !m.theme.Color {
		t.Fatal("the harness built a colourless theme, so nothing here could pulse")
	}
	return m, clk
}

func boolPtr(v bool) *bool { return &v }

// frames drives the pulse chain the way the runtime does, returning the frame
// each delivered phase renders and stopping when the model asks for no further
// one. The clock moves one interval per phase, so the idle pause arrives after
// the same number of phases a real session would take.
//
// A frame is what View renders: the runtime draws after every message it
// receives, so a message the model never asks for is a frame nobody is sent.
func frames(m Model, limit int) (Model, []string) {
	var out []string
	for len(out) < limit {
		m.now = advanced(m.now, PulseInterval)
		next, cmd := m.reduce(pulseMsg{})
		m = next
		out = append(out, m.View())
		if cmd == nil {
			return m, out
		}
	}
	return m, out
}

// hasPulse reports whether a command asks for a phase of the pulse. The models
// here are built without a service, so every other command an interface would
// batch is nil and running this executes the tick and nothing else.
func hasPulse(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	switch msg := cmd().(type) {
	case tea.BatchMsg:
		for _, member := range msg {
			if member == nil {
				continue
			}
			if _, ok := member().(pulseMsg); ok {
				return true
			}
		}
		return false
	case pulseMsg:
		return true
	default:
		return false
	}
}

// advanced returns a clock moved on by d, so a test walks time without reaching
// into the model.
func advanced(now func() time.Time, d time.Duration) func() time.Time {
	at := now().Add(d)
	return func() time.Time { return at }
}

func TestMotionEnabledNeedsBothThePreferenceAndColour(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode  string
		color bool
		want  bool
	}{
		{MotionOn, true, true},
		{"", true, true},
		{"ON", true, true},
		{MotionOff, true, false},
		{MotionOn, false, false},
		{"", false, false},
		{MotionOff, false, false},
	}
	for _, tc := range cases {
		if got := MotionEnabled(tc.mode, tc.color); got != tc.want {
			t.Errorf("MotionEnabled(%q, %v) = %v, want %v", tc.mode, tc.color, got, tc.want)
		}
	}
}

func TestThePulseStopsOnceTheReaderHasGoneIdle(t *testing.T) {
	t.Parallel()
	for _, idle := range []time.Duration{0, MotionIdleAfter - time.Nanosecond} {
		if !PulseRunning(MotionOn, true, idle) {
			t.Errorf("the pulse stopped after %s, inside the idle window", idle)
		}
	}
	for _, idle := range []time.Duration{MotionIdleAfter, MotionIdleAfter + time.Minute} {
		if PulseRunning(MotionOn, true, idle) {
			t.Errorf("the pulse was still running after %s idle", idle)
		}
	}
}

// TestAnIdleSessionStopsProducingFrames is the requirement rather than an
// optimisation: `tix ssh` renders server side, so a session nobody is at must
// send nothing at all. The assertion is the frames themselves, driven until the
// model asks for no further phase, and then one further phase offered anyway to
// prove nothing restarts it.
func TestAnIdleSessionStopsProducingFrames(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	// Generously more phases than the idle window holds, so a pulse that never
	// stops fails here rather than looping for ever.
	limit := int(MotionIdleAfter/PulseInterval) * 4
	m, produced := frames(m, limit)
	if len(produced) >= limit {
		t.Fatalf("the pulse produced %d frames and was still going; it never stops", len(produced))
	}
	if len(produced) < 2 || produced[0] == produced[1] {
		t.Fatalf("consecutive phases drew the same frame, so there was no pulse to stop")
	}
	// The last phase delivered is the one that finds the session idle, so the
	// chain must end inside one interval of the timeout: sooner would cut the
	// motion short, later would keep sending frames past it.
	stopped := time.Duration(len(produced)) * PulseInterval
	if stopped < MotionIdleAfter || stopped >= MotionIdleAfter+PulseInterval {
		t.Errorf("the pulse ran for %s over %d frames, want it to stop within one %s phase of %s",
			stopped, len(produced), PulseInterval, MotionIdleAfter)
	}
	settled := m.View()
	next, cmd := m.reduce(pulseMsg{})
	if cmd != nil {
		t.Error("a phase arriving after the idle pause asked for another one")
	}
	if next.View() != settled {
		t.Error("a phase arriving after the idle pause changed the frame, so bytes went out")
	}
}

// TestThePulseRestsAtFullEmphasis keeps the idle pause from reading as a fault:
// a session that stopped animating draws the row a reader with motion off
// draws, rather than the low phase caught mid-cycle.
func TestThePulseRestsAtFullEmphasis(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	// The pause is reached after an odd number of phases, so a pulse that
	// simply stopped wherever it happened to be would be caught on its quiet
	// phase and fail here. An even number would pass either way.
	m, produced := frames(m, 3)
	if len(produced) != 3 {
		t.Fatalf("the pulse stopped after %d phases, before the pause was reached", len(produced))
	}
	m.now = advanced(m.now, MotionIdleAfter)
	m, cmd := m.reduce(pulseMsg{})
	if cmd != nil {
		t.Fatal("the pulse did not stop once the idle window had passed")
	}
	still := New(Config{
		Access: fullAccess(), Environ: []string{"TERM=xterm-256color"},
		Renderer: NewRenderer([]string{"TERM=xterm-256color"}, io.Discard),
		Color:    boolPtr(true), Prefs: Preferences{Motion: MotionOff},
	})
	if got, want := m.selection().Render("row"), still.selection().Render("row"); got != want {
		t.Errorf("a rested pulse draws the selected row as %q, a session with motion off draws %q", got, want)
	}
}

func TestThePulseResumesOnTheNextKeystroke(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	m, _ = frames(m, int(MotionIdleAfter/PulseInterval)*4)
	m, cmd := m.reduce(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if !hasPulse(cmd) {
		t.Fatal("a keystroke after the idle pause did not start the pulse again")
	}
	if _, produced := frames(m, 3); len(produced) != 3 || produced[0] == produced[1] {
		t.Fatalf("the resumed pulse produced %d frames and did not alternate", len(produced))
	}
}

// TestASessionWithNoColourNeverPulses is the degradation rule: a terminal that
// cannot draw a difference gets a static selection rather than a stream of
// identical frames. NO_COLOR is one way in, output.color = never is another.
func TestASessionWithNoColourNeverPulses(t *testing.T) {
	t.Parallel()
	m := New(Config{Access: fullAccess(), Environ: []string{"NO_COLOR=1", "TERM=xterm-256color"}})
	if m.pulsing {
		t.Error("a NO_COLOR session armed the pulse")
	}
	if cmd := m.Init(); hasPulse(cmd) {
		t.Error("a NO_COLOR session asked for a pulse when it opened")
	}
	if _, cmd := m.afterInput(nil); hasPulse(cmd) {
		t.Error("a keystroke started a pulse on a NO_COLOR session")
	}
	if got := m.selection().Render("row"); strings.Contains(got, "\x1b") {
		t.Errorf("a colourless selection rendered %q", got)
	}
}

// TestMotionIsOnWhenNobodyConfiguredIt covers the session the SSH listener
// opens, which passes no preferences at all.
func TestMotionIsOnWhenNobodyConfiguredIt(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	if !m.pulsing {
		t.Fatal("a session given no preferences did not animate")
	}
	if cmd := m.Init(); !hasPulse(cmd) {
		t.Fatal("opening a session given no preferences asked for no pulse")
	}
}

// TestTurningMotionOffStopsThePulseWithoutARestart holds the preference against
// the running chain: the row it is changed on is the row that stops moving.
func TestTurningMotionOffStopsThePulseWithoutARestart(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	m.prefs.Motion = MotionOff
	if _, cmd := m.reduce(pulseMsg{}); cmd != nil {
		t.Fatal("the pulse carried on after the motion preference was turned off")
	}
	m.prefs.Motion = MotionOn
	m.pulsing = false
	if _, cmd := m.afterInput(nil); !hasPulse(cmd) {
		t.Fatal("turning the motion back on did not arm the pulse")
	}
}

// TestTheSelectionIsIdentifiableAtBothPhasesOfThePulse is the reader's half of
// the feature, and every half of it is asserted, because each alone passes
// while the selection is lost. "Different from an unselected row" is the
// weakest of them and passes on its own even when the quiet phase is drawn with
// no attributes at all, since an unselected card is built from other styles
// entirely: the phase has to be shown to still carry the selection's own
// colour, with only its weight varying.
func TestTheSelectionIsIdentifiableAtBothPhasesOfThePulse(t *testing.T) {
	t.Parallel()
	m, _ := animatedModel(t)
	card := task("a", "todo", 1, core.PriorityNormal)
	marker := strings.TrimSpace(SelectionMarker(true))
	phases := map[bool]string{}
	for _, quiet := range []bool{false, true} {
		m.pulseQuiet = quiet
		selected := m.cardLine(card, 40, true)
		if !strings.Contains(selected, marker) {
			t.Errorf("%s dropped the selection marker: %q", phaseName(quiet), selected)
		}
		if selected == m.cardLine(card, 40, false) {
			t.Errorf("%s draws the selected card exactly like an unselected one: %q",
				phaseName(quiet), selected)
		}
		if len(colorsOf(selected)) == 0 {
			t.Errorf("%s draws the selected card in no colour at all: %q", phaseName(quiet), selected)
		}
		phases[quiet] = selected
	}
	if phases[false] == phases[true] {
		t.Errorf("both phases render identically, so nothing pulses: %q", phases[true])
	}
	// Same colour, different weight: the pulse varies emphasis rather than
	// swapping the selection for something else.
	if quiet, loud := colorsOf(phases[true]), colorsOf(phases[false]); !slices.Equal(quiet, loud) {
		t.Errorf("the quiet phase draws the selected card in %v, the emphasised phase in %v", quiet, loud)
	}
}

// colorsOf is a styled string's colour attributes, with the weight left out, so
// two phases of the pulse can be compared on the colour they share.
func colorsOf(s string) []string {
	var out []string
	for _, p := range sgrParams(s) {
		if p != "1" {
			out = append(out, p)
		}
	}
	return out
}

// phaseName names one phase of the pulse for a failure message.
func phaseName(quiet bool) string {
	if quiet {
		return "the quiet phase"
	}
	return "the emphasised phase"
}

func TestTheMotionRowSaysWhatItWillDoOnThisTerminal(t *testing.T) {
	t.Parallel()
	cases := []struct {
		mode  string
		color bool
		want  string
	}{
		{MotionOn, true, "pulses"},
		{"", true, "pulses"},
		{MotionOn, false, "no colour"},
		{MotionOff, true, "same in every frame"},
	}
	for _, tc := range cases {
		got := MotionDescription(tc.mode, tc.color)
		if !strings.Contains(got, tc.want) {
			t.Errorf("motion %q with colour %v reads %q, want it to mention %q",
				tc.mode, tc.color, got, tc.want)
		}
	}
	// The row is described by the same function the screen renders it with, and
	// the colour it resolves against is the mode in force rather than the probe:
	// with colour refused, the row must not claim the selection is moving.
	quiet := OptionDescription(SettingMotion, MotionOn,
		Preferences{Color: output.ColorNever}, time.Time{}, true)
	if !strings.Contains(quiet, "no colour") {
		t.Errorf("with output.color = never the motion row reads %q", quiet)
	}
	loud := OptionDescription(SettingMotion, MotionOn,
		Preferences{Color: output.ColorAlways}, time.Time{}, false)
	if !strings.Contains(loud, "pulses") {
		t.Errorf("with output.color = always on a colourless probe the motion row reads %q", loud)
	}
}

// TestSteppingTheMotionRowStopsAndStartsThePulse is the row working like the
// other four: the change applies to the frame it is read in, and it is written
// down in the same act.
func TestSteppingTheMotionRowStopsAndStartsThePulse(t *testing.T) {
	t.Parallel()
	var saved []Preferences
	m, _ := animatedModel(t)
	m.savePrefs = func(p Preferences) error { saved = append(saved, p); return nil }
	m = m.openSettings()
	m.settingSel = SettingMotion

	m = m.cycleSetting(1)
	if m.prefs.Motion != MotionOff {
		t.Fatalf("stepping the motion row left it on %q", m.prefs.Motion)
	}
	if len(saved) != 1 || saved[0].Motion != MotionOff {
		t.Fatalf("the change was not written down: %v", saved)
	}
	if !strings.Contains(m.status, KeyTUIMotion) {
		t.Errorf("the status bar said %q, which does not name the key that was written", m.status)
	}
	stopped, cmd := m.reduce(pulseMsg{})
	if cmd != nil {
		t.Error("the pulse carried on after the row was stepped to off")
	}
	m = stopped

	m = m.cycleSetting(1)
	if m.prefs.Motion != MotionOn {
		t.Fatalf("stepping the motion row again left it on %q", m.prefs.Motion)
	}
	if _, cmd := m.afterInput(nil); !hasPulse(cmd) {
		t.Error("stepping the row back to on did not start the pulse again")
	}
}
