// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// twoParagraphs is what a reader types into a body: prose, with a newline in
// it. The reported defect is that the interface had nowhere to put one.
const twoParagraphs = "Everything can talk to everything.\nStart with staging, then repeat in production."

// taskEditModel opens the whole-task form over the selected task, draining the
// directory read the way the runtime would. The form cannot open until that
// lands, so every assertion about it goes through here.
func taskEditModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	svc.directory = []core.Actor{{ID: "a-ada", Handle: "ada"}, {ID: "a-grace", Handle: "grace"}}
	m.svc = svc
	m, cmd := m.reduce(pressKey("E"))
	m, _ = m.reduce(run(t, cmd))
	if !m.form.Open() {
		t.Fatalf("E opened no form: %q", m.err)
	}
	return m, svc
}

// fieldAt moves the form's cursor onto a named field with the keys a reader
// has, failing rather than reaching into Sel, so a form that cannot be walked
// fails here rather than passing on a value nothing could have reached.
func fieldAt(t *testing.T, m Model, key string) Model {
	t.Helper()
	for range len(m.form.Fields) + 1 {
		if f, ok := m.form.Selected(); ok && f.Key == key {
			return m
		}
		m, _ = m.reduce(pressKey("tab"))
	}
	t.Fatalf("tab never reached the %q field of the %q form", key, m.form.Title)
	return m
}

// TestTheBodyIsEditedAsProse is the reported defect: E opened a single-line
// input, so a body was crammed into one clipped line and a newline could not
// be typed at all.
func TestTheBodyIsEditedAsProse(t *testing.T) {
	m, _ := taskEditModel(t)
	m = fieldAt(t, m, "body")
	field, _ := m.form.Selected()
	if field.Kind != FieldProse {
		t.Fatalf("the body field is kind %v, not the multi-line one", field.Kind)
	}
	if !m.form.Prose() {
		t.Fatal("the form does not report the body as prose, so enter will submit it")
	}
}

// TestEnterInsertsANewlineInProse pins the whole point of the field. Enter
// submitting here is the behaviour that makes a body one line.
func TestEnterInsertsANewlineInProse(t *testing.T) {
	m, svc := taskEditModel(t)
	m = fieldAt(t, m, "body")
	m.area.SetValue("first line")
	m, _ = m.reduce(pressKey("enter"))
	if !m.form.Open() {
		t.Fatal("enter submitted the form from inside the multi-line field")
	}
	m, _ = m.reduce(pressKey("x"))
	if got := m.area.Value(); got != "first line\nx" {
		t.Fatalf("the field holds %q; enter inserted no newline", got)
	}
	if len(svc.updated) != 0 {
		t.Fatalf("enter sent an edit from inside the field: %+v", svc.updated)
	}
}

// TestTheCommitKeyAppliesTheFormAndIsOnScreen is the other half. A multi-line
// field whose reader cannot discover how to save it is worse than the one line
// it replaced, so the key that applies is named in the panel's own legend.
func TestTheCommitKeyAppliesTheFormAndIsOnScreen(t *testing.T) {
	m, svc := taskEditModel(t)
	m = fieldAt(t, m, "body")
	legend := panelLegend(t, m.Frame())
	for _, want := range []string{"ctrl+s apply", "enter newline", "tab", "esc cancel"} {
		if !strings.Contains(legend, want) {
			t.Fatalf("the legend %q does not name %q", legend, want)
		}
	}
	m.area.SetValue(twoParagraphs)
	m, cmd := m.reduce(keyMsgFor("ctrl+s"))
	if m.form.Open() {
		t.Fatal("the commit key did not apply the form")
	}
	run(t, cmd)
	if len(svc.updated) != 1 || svc.updated[0].Body == nil {
		t.Fatalf("updated = %+v", svc.updated)
	}
	if got := *svc.updated[0].Body; got != twoParagraphs {
		t.Fatalf("the service was sent %q, want %q", got, twoParagraphs)
	}
}

// TestAProseFieldWritesNoEscapeWithoutColour keeps docs/tui.md's promise. The
// widget ships its own colours, its own line numbers, its own prompt character
// and a reverse-video caret, and a dumb terminal over tix ssh gets none of it.
func TestAProseFieldWritesNoEscapeWithoutColour(t *testing.T) {
	for _, tc := range []struct {
		name string
		open func(*testing.T) Model
	}{
		{"the task form", func(t *testing.T) Model {
			m, _ := taskEditModel(t)
			return fieldAt(t, m, "body")
		}},
		{"a comment", func(t *testing.T) Model {
			m := boardModel(t)
			m.svc = newFakeService()
			m, _ = m.reduce(pressKey("m"))
			return m
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := tc.open(t)
			if m.theme.Color {
				t.Fatal("the model under test is drawing in colour, so this proves nothing")
			}
			m.area.SetValue(twoParagraphs)
			block := strings.Join(panelBlock(t, m.Frame()), "\n")
			if strings.Contains(block, "\x1b") {
				t.Fatalf("a colourless prose field emitted an escape sequence:\n%q", block)
			}
			if strings.Contains(block, "1 ") && strings.Contains(block, "2 ") {
				t.Fatalf("the prose field is drawing line numbers:\n%s", block)
			}
			for _, want := range strings.Split(twoParagraphs, "\n") {
				if !strings.Contains(block, want) {
					t.Fatalf("the panel does not carry %q:\n%s", want, block)
				}
			}
		})
	}
}

// TestProseHeightStaysWithinTheTerminal pins the range. A short body reserving
// half the screen and a long one drawn into two lines are the same mistake.
func TestProseHeightStaysWithinTheTerminal(t *testing.T) {
	for _, tc := range []struct {
		name     string
		lines    int
		terminal int
		want     int
	}{
		{"a one-line body takes the floor", 1, 40, MinProseLines},
		{"a body grows with what it holds", 6, 40, 6},
		{"a long body stops at the ceiling", 200, 90, MaxProseLines},
		{"a short terminal wins over the floor", 200, 24, 8},
		{"a tiny terminal still gets the floor", 200, 9, MinProseLines},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := ProseHeight(tc.lines, tc.terminal); got != tc.want {
				t.Fatalf("ProseHeight(%d, %d) = %d, want %d", tc.lines, tc.terminal, got, tc.want)
			}
		})
	}
}

// TestTheLegendIsStillTheLastLineUnderAProseField is what the panel work
// promised and what a taller field is most likely to break.
func TestTheLegendIsStillTheLastLineUnderAProseField(t *testing.T) {
	m, _ := taskEditModel(t)
	m = fieldAt(t, m, "body")
	m.area.SetValue(strings.Repeat("a paragraph of the body\n", 40))
	m = m.fitArea()
	frame := m.Frame()
	lines := strings.Split(strings.TrimRight(stripANSI(frame), "\n"), "\n")
	if got := len(lines); got > m.height {
		t.Fatalf("the frame is %d lines on a %d line terminal", got, m.height)
	}
	if last, legend := lines[len(lines)-1], panelLegend(t, frame); last != legend {
		t.Fatalf("the last line on screen is %q, not the legend %q", last, legend)
	}
}

// TestTabMovesBetweenFieldsAndTheArrowsStayWithTheField is the conflict a
// text field in a form creates: both cannot own up and down.
func TestTabMovesBetweenFieldsAndTheArrowsStayWithTheField(t *testing.T) {
	m, _ := taskEditModel(t)
	m = fieldAt(t, m, "body")
	m.area.SetValue(twoParagraphs)
	m.area.MoveToEnd()
	before := m.area.Line()
	up, _ := m.reduce(pressKey("up"))
	if f, _ := up.form.Selected(); f.Key != "body" {
		t.Fatalf("up left the body field for %q, so the field cannot be moved through", f.Key)
	}
	if up.area.Line() == before {
		t.Fatal("up moved no cursor inside the body, so the arrow reached neither the form nor the field")
	}
	back, _ := m.reduce(pressKey("shift+tab"))
	if f, _ := back.form.Selected(); f.Key != "title" {
		t.Fatalf("shift+tab reached %q, not the field above the body", f.Key)
	}
}

// TestACancelledFormWritesNothing is what a form that shows a change before
// making it has to guarantee.
func TestACancelledFormWritesNothing(t *testing.T) {
	m, svc := taskEditModel(t)
	m = fieldAt(t, m, "body")
	m.area.SetValue("something else entirely")
	m, cmd := m.reduce(pressKey("esc"))
	if m.form.Open() {
		t.Fatal("esc left the form open")
	}
	if cmd != nil {
		cmd()
	}
	if len(svc.updated) != 0 {
		t.Fatalf("cancelling the form sent an edit: %+v", svc.updated)
	}
}

// TestTheTaskFormSendsOnlyWhatChanged keeps the audit log honest: a trip that
// touched the body must not record an edit to the title.
func TestTheTaskFormSendsOnlyWhatChanged(t *testing.T) {
	m, svc := taskEditModel(t)
	m = fieldAt(t, m, "body")
	m.area.SetValue(twoParagraphs)
	m, cmd := m.reduce(keyMsgFor("ctrl+s"))
	run(t, cmd)
	if len(svc.updated) != 1 {
		t.Fatalf("updated = %+v", svc.updated)
	}
	in := svc.updated[0]
	if in.Title != nil {
		t.Fatalf("an untouched title was sent as %q", *in.Title)
	}
	if in.Priority != nil || in.AssigneeActorID != nil {
		t.Fatalf("untouched fields were sent: %+v", in)
	}
	if fields := updatedFields(in); len(fields) != 1 || fields[0] != "body" {
		t.Fatalf("the edit names %v, want the body alone", fields)
	}
	_ = m
}

// TestTheTaskFormShowsWhatItWillChange is the other half of showing before
// changing: every field the form gathers is on screen with its current value.
func TestTheTaskFormShowsWhatItWillChange(t *testing.T) {
	m, _ := taskEditModel(t)
	task, _ := TaskAt(m.columns, m.sel)
	block := strings.Join(panelBlock(t, m.Frame()), "\n")
	if !strings.Contains(block, task.Ref) {
		t.Fatalf("the panel does not name the task it edits:\n%s", block)
	}
	for _, want := range []string{"title", "body", "priority", "assignee"} {
		if formRows(t, m.Frame(), "edit "+task.Ref, want) != 1 {
			t.Fatalf("the form draws no row for %q:\n%s", want, block)
		}
	}
	if got := formRow(t, m.Frame(), "edit "+task.Ref, "priority"); !strings.Contains(got, PriorityLabel(task.Priority)) {
		t.Fatalf("the priority row %q does not show what the task holds", got)
	}
}

// TestTheSingleKeyEditsStillWork is the promise that converging on one form
// took nothing away: the keys that change one thing in one press are untouched.
func TestTheSingleKeyEditsStillWork(t *testing.T) {
	for _, tc := range []struct {
		key  string
		want func(Model) bool
	}{
		{"e", func(m Model) bool { return m.prompt == promptTitle }},
		{"P", func(m Model) bool { return m.choice == choicePriority }},
		{"#", func(m Model) bool { return m.prompt == promptTag }},
		{"U", func(m Model) bool { return m.prompt == promptUntag }},
		{"D", func(m Model) bool { return m.prompt == promptDependency }},
		{"m", func(m Model) bool { return m.prompt == promptComment }},
	} {
		t.Run(tc.key, func(t *testing.T) {
			m := boardModel(t)
			m.svc = newFakeService()
			next, _ := m.reduce(pressKey(tc.key))
			if !tc.want(next) {
				t.Fatalf("%q no longer opens what it opened: %q", tc.key, next.err)
			}
		})
	}
}

// TestACommentKeepsItsNewlines is the round trip for the other prose field.
// The seed used to flatten every newline into a space before the reader saw it.
func TestACommentKeepsItsNewlines(t *testing.T) {
	m, svc := threadModel(t)
	m, _ = m.reduce(pressKey("m"))
	if m.prompt != promptComment {
		t.Fatalf("m opened prompt %v", m.prompt)
	}
	m.area.SetValue(twoParagraphs)
	m, cmd := m.reduce(keyMsgFor("ctrl+s"))
	if m.prompt != promptNone {
		t.Fatal("the commit key left the comment prompt open")
	}
	run(t, cmd)
	if len(svc.comments) != 1 || svc.comments[0] != twoParagraphs {
		t.Fatalf("comments = %+v", svc.comments)
	}
}

// TestEditingACommentOpensOnEveryLineOfIt pins the seed. A comment written
// over three lines arrived in the field as one, so saving it flattened it.
func TestEditingACommentOpensOnEveryLineOfIt(t *testing.T) {
	m, svc := threadModel(t)
	svc.thread[1].Body = twoParagraphs
	m, _ = m.reduce(detailMsg{task: mustTask(t, m), comments: svc.thread})
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("M"))
	if m.prompt != promptCommentEdit {
		t.Fatalf("M opened prompt %v: %q", m.prompt, m.err)
	}
	if got := m.area.Value(); got != twoParagraphs {
		t.Fatalf("the edit opened on %q, want the comment as written", got)
	}
}

// mustTask is the selected task, for a test that needs it to build a message.
func mustTask(t *testing.T, m Model) core.Task {
	t.Helper()
	task, ok := TaskAt(m.columns, m.sel)
	if !ok {
		t.Fatal("no task is selected")
	}
	return task
}

// TestTheDetailKeepsTheNewlinesTheBodyWasWrittenWith is the far end of the
// round trip. A body typed as two lines that comes back as one paragraph is
// the multi-line field having been for nothing.
func TestTheDetailKeepsTheNewlinesTheBodyWasWrittenWith(t *testing.T) {
	m := boardModel(t)
	task := mustTask(t, m)
	task.Body = twoParagraphs
	m, _ = m.reduce(detailMsg{task: task})
	got := bodySection(t, m.Frame())
	want := strings.Split(twoParagraphs, "\n")
	if len(got) != len(want) {
		t.Fatalf("the detail drew %d body lines, want %d:\n%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("body line %d is %q, want %q", i, got[i], want[i])
		}
	}
}

// TestNoValueFieldBecameProse is the audit, held against the code. A tag name,
// a reference or a filter expression is a value, and a value in a field ten
// lines tall is a panel spending the terminal on nothing.
func TestNoValueFieldBecameProse(t *testing.T) {
	prose := map[promptKind]bool{promptComment: true, promptCommentEdit: true, promptProjectDesc: true}
	for kind, spec := range promptSpecs {
		if spec.Multiline != prose[kind] {
			t.Errorf("the %q input is multi-line=%v, want %v", spec.Title(), spec.Multiline, prose[kind])
		}
	}
}
