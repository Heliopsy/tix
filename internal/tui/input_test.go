// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"
	"testing"
)

// panelBlock returns the one input panel of a frame: the lines from its rule
// to the end. Reading the panel rather than the frame is what keeps "the
// legend names esc" from passing on the board's own footer.
func panelBlock(t *testing.T, frame string) []string {
	t.Helper()
	lines := strings.Split(stripANSI(frame), "\n")
	at := -1
	for i, l := range lines {
		if l != "" && strings.Trim(l, InputRule) == "" {
			if at >= 0 {
				t.Fatalf("the frame carries more than one input panel:\n%s", frame)
			}
			at = i
		}
	}
	if at < 0 {
		t.Fatalf("the frame carries no input panel:\n%s", frame)
	}
	return lines[at+1:]
}

// panelLegend is the last line of the panel, which is the line that names the
// keys that answer it.
func panelLegend(t *testing.T, frame string) string {
	t.Helper()
	block := panelBlock(t, frame)
	for i := len(block) - 1; i >= 0; i-- {
		if strings.TrimSpace(block[i]) != "" {
			return block[i]
		}
	}
	t.Fatalf("the input panel has no legend:\n%s", frame)
	return ""
}

// TestEveryInputModeOwnsTheFooter is the reported defect: while a prompt was
// open the footer still advertised the board's keys, so the interface told the
// reader to press n for a new task when pressing n would type the letter n.
func TestEveryInputModeOwnsTheFooter(t *testing.T) {
	tests := []struct {
		name  string
		keys  []string
		title string
		legnd string
	}{
		{"a prompt", []string{"n"}, "new task:", "enter apply"},
		{"a picker", []string{"t"}, "transition to:", "choose"},
		{"a form", []string{"X"}, "delete:", "field"},
		{"a confirmation", []string{"X", "enter"}, "confirm:", "confirms"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := boardModel(t)
			m.svc = newFakeService()
			for _, k := range tc.keys {
				m, _ = m.reduce(pressKey(k))
			}
			frame := m.Frame()
			block := strings.Join(panelBlock(t, frame), "\n")
			if !strings.Contains(block, tc.title) {
				t.Fatalf("the panel does not name itself %q:\n%s", tc.title, block)
			}
			legend := panelLegend(t, frame)
			if !strings.Contains(legend, tc.legnd) {
				t.Fatalf("the legend %q does not name %q", legend, tc.legnd)
			}
			// The board's own keys are gone. Each of these is typed as text or
			// swallowed while the mode is open, so advertising it is a lie.
			for _, gone := range []string{"new task", "edit title", "transition", "filter"} {
				if strings.Contains(legend, gone) {
					t.Fatalf("the legend still advertises the board's %q: %q", gone, legend)
				}
			}
		})
	}
}

// TestNoInputModeLeavesTheBoardFooterBehind is the other half. An assertion
// that the panel exists passes while the board's footer is still drawn under
// it, which is exactly the state the defect was reported in.
func TestNoInputModeLeavesTheBoardFooterBehind(t *testing.T) {
	m := boardModel(t)
	open := m
	open, _ = open.reduce(pressKey("n"))
	if !open.inputOpen() {
		t.Fatal("pressing n opened no input mode")
	}
	closed := footerLine(t, m.Frame())
	if !strings.Contains(closed, "new task") {
		t.Fatalf("the board's own footer does not advertise n: %q", closed)
	}
	if got := footerLine(t, open.Frame()); strings.Contains(got, "new task  ") {
		t.Fatalf("the board's footer survived under the prompt: %q", got)
	}
}

// TestASinglePromptIsAFormWithOneField is the convergence the reader asked
// for: the label sits in the same column a form's labels sit in, so a prompt
// is recognisable from having used the delete form.
func TestASinglePromptIsAFormWithOneField(t *testing.T) {
	tests := []struct {
		key   string
		field string
	}{
		{"n", "title"},
		{"/", "expression"},
		{"m", "comment"},
		{"#", "tag"},
	}
	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			m := boardModel(t)
			m.svc = newFakeService()
			m, _ = m.reduce(pressKey(tc.key))
			if m.prompt == promptNone {
				t.Fatalf("%q opened no prompt", tc.key)
			}
			var row string
			for _, l := range panelBlock(t, m.Frame()) {
				if strings.HasPrefix(l, "  "+tc.field) {
					row = l
				}
			}
			if row == "" {
				t.Fatalf("no row of the panel is labelled %q:\n%s",
					tc.field, strings.Join(panelBlock(t, m.Frame()), "\n"))
			}
			if got := strings.Index(row, strings.TrimSpace(row)); got != 2 {
				t.Fatalf("the label starts at column %d, not the form's column 2: %q", got, row)
			}
			if len(row) < 2+FormLabelWidth {
				t.Fatalf("the value does not reach the form's value column: %q", row)
			}
		})
	}
}

// TestAnInputModeIsVisibleWithoutColour keeps the focus cue off colour. A
// colourless run writes no escapes, so the rule and the panel's own legend are
// the whole of the signal that the keyboard has been taken.
func TestAnInputModeIsVisibleWithoutColour(t *testing.T) {
	m := boardModel(t)
	if m.theme.Color {
		t.Fatal("the board under test is drawing in colour, so this proves nothing")
	}
	m.svc = newFakeService()
	m, _ = m.reduce(pressKey("n"))
	frame := m.Frame()
	if strings.Contains(frame, "\x1b") {
		t.Fatalf("a colourless frame emitted an escape sequence:\n%q", frame)
	}
	rule := ""
	for _, l := range strings.Split(frame, "\n") {
		if l != "" && strings.Trim(l, InputRule) == "" {
			rule = l
		}
	}
	if rule == "" {
		t.Fatalf("nothing separates the panel from the board it covers:\n%s", frame)
	}
	if got := len([]rune(rule)); got != m.width {
		t.Fatalf("the rule is %d cells wide on a %d cell terminal", got, m.width)
	}
}

// TestAPickerKeepsItsSingleKeystroke pins the speed of the numbered picker.
// Converging the idioms must not make t cost more keys than it did.
func TestAPickerKeepsItsSingleKeystroke(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, _ = m.reduce(pressKey("t"))
	if m.choice != choiceTransition {
		t.Fatal("t opened no picker")
	}
	m, cmd := m.reduce(pressKey("1"))
	if m.choice != choiceNone {
		t.Fatal("one digit did not answer the picker")
	}
	if cmd == nil {
		t.Fatal("answering the picker asked for no work")
	}
	cmd()
	if len(svc.transitions) != 1 {
		t.Fatalf("one keystroke recorded %d transitions", len(svc.transitions))
	}
}

// TestTheBoardCountIsOnlyOnTheBoard pins the other reported defect: the count
// of tasks and columns sat under the task detail, where it describes nothing
// the reader is looking at.
func TestTheBoardCountIsOnlyOnTheBoard(t *testing.T) {
	board := boardModel(t)
	count := fmt.Sprintf("%d tasks in %d columns", countTasks(board.columns), len(board.columns))
	if !strings.Contains(board.statusBar(), count) {
		t.Fatalf("the board's own status bar does not carry %q: %q", count, board.statusBar())
	}
	// The status bar alone, not the frame: the phrase appears nowhere else, so
	// a frame-wide search would pass on a board that had simply not changed.
	for _, view := range []viewKind{viewDetail, viewStats, viewActivity, viewHistory, viewSettings, viewProjects} {
		m := board
		m.view = view
		if bar := stripANSI(m.statusBar()); strings.Contains(bar, "tasks in") {
			t.Errorf("the %s view's status bar carries the board's count: %q", viewName(view), bar)
		}
	}
}
