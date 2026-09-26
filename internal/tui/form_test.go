// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// keyMsgFor builds the message a terminal sends for a key a scheme names,
// including the control chords three of the schemes rebind onto.
func keyMsgFor(name string) tea.KeyMsg {
	if strings.HasPrefix(name, "ctrl+") && len(name) == len("ctrl+")+1 {
		return tea.KeyMsg{Type: tea.KeyType(name[len("ctrl+")] - 'a' + 1)}
	}
	return pressKey(name)
}

// formBlock returns only the lines the open form drew, located from the end of
// the frame because a form draws in the footer while the body above it carries
// the same words: the detail view prints "tags: urgent" and a form titled
// "tags" would match it.
//
// Every assertion about a form goes through here rather than through the frame,
// which is the difference between a guard that watches the form and one that
// passes on the task the form is drawn over.
func formBlock(t *testing.T, frame, title string) string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == title+":" {
			start = i
			break
		}
	}
	if start < 0 {
		t.Fatalf("no form titled %q is drawn:\n%s", title, frame)
	}
	for i := start; i < len(lines); i++ {
		if strings.Contains(lines[i], "apply") {
			return strings.Join(lines[start:i+1], "\n")
		}
	}
	t.Fatalf("the %q form drew no key line, so its block has no end:\n%s", title, frame)
	return ""
}

// formRow returns the one row of a form that carries a named field, so an
// assertion about that field's value cannot be satisfied by another field's.
func formRow(t *testing.T, frame, title, label string) string {
	t.Helper()
	var found []string
	for _, l := range strings.Split(formBlock(t, frame, title), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), label) {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the %q form draws %d rows for %q, want exactly one:\n%s",
			title, len(found), label, formBlock(t, frame, title))
	}
	return found[0]
}

// formRows reports whether a form draws a row for a field at all.
func formRows(t *testing.T, frame, title, label string) int {
	t.Helper()
	n := 0
	for _, l := range strings.Split(formBlock(t, frame, title), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), label) {
			n++
		}
	}
	return n
}

// threadModel opens a task with a comment thread and two dependencies, which is
// what the comment and dependency actions act on.
func threadModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	when := time.Date(2026, 3, 4, 9, 30, 0, 0, time.UTC)
	svc.thread = []core.Comment{
		{ID: "c-1", AuthorActorID: "a-ada", Body: "blocked on the migration", CreatedAt: when},
		{ID: "c-2", AuthorActorID: "a-grace", Body: "the migration landed", CreatedAt: when.Add(time.Hour)},
	}
	svc.dependencies = []core.Dependency{{DependsOn: "infra-7"}, {DependsOn: "infra-9"}}
	svc.tags = []core.Tag{{Name: "urgent"}, {Name: "backend"}}
	m.svc = svc
	task, _ := TaskAt(m.columns, m.sel)
	task.Tags = []string{"backend"}
	m, _ = m.reduce(detailMsg{task: task, comments: svc.thread, deps: svc.dependencies,
		actors: map[string]string{"a-ada": "ada", "a-grace": "grace"}})
	if m.view != viewDetail {
		t.Fatal("the detail view did not open")
	}
	return m, svc
}

// run drains the command an action returned, the way the real runtime would.
func run(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("the action asked for nothing to be done")
	}
	return cmd()
}

func TestAFormHidesTheFieldsItsOwnAnswersMakeIrrelevant(t *testing.T) {
	form := DeleteForm("infra-1", "the comment by @ada", true, true)
	if form.Value("hard") != noValue {
		t.Fatalf("a task deletion does not offer its reach: %+v", form.Visible())
	}
	comment := form.Cycle(1)
	if comment.Value("what") != "comment" {
		t.Fatalf("cycling the subject gave %q", comment.Value("what"))
	}
	if comment.Value("hard") != "" || comment.Value("cascade") != "" {
		t.Fatal("deleting a comment still answers the task's reach switches")
	}
	for _, field := range comment.Visible() {
		if field.Key == "hard" || field.Key == "cascade" {
			t.Fatalf("field %q is shown for a subject it does not apply to", field.Key)
		}
	}
}

func TestTheCursorNeverRestsOnAHiddenField(t *testing.T) {
	form := DeleteForm("infra-1", "the comment by @ada", true, true)
	form = form.Move(2)
	if got, _ := form.Selected(); got.Key != "cascade" {
		t.Fatalf("the cursor did not reach the last field: %q", got.Key)
	}
	form = form.Move(-2).Cycle(1)
	if got, ok := form.Selected(); !ok || got.Key != "what" {
		t.Fatalf("selected = %q %v", got.Key, ok)
	}
	if form.Sel >= len(form.Visible()) {
		t.Fatalf("the cursor sits at %d with %d visible fields", form.Sel, len(form.Visible()))
	}

	// A field can hide the field above it, which is the shape that leaves the
	// cursor pointing past the end: the answer that hides one row shortens the
	// list the cursor indexes into. Built by hand because none of the three
	// forms shipped so far is that shape, and the clamp is for the next one.
	shrinking := Form{Kind: formDelete, Title: "delete", Sel: 1, Fields: []FormField{
		{Key: "reach", Label: "reach", Options: switchOptions(), Value: noValue,
			Needs: FieldCondition{Key: "what", Value: "task"}},
		{Key: "what", Label: "what", Options: []string{"task", "comment"}, Value: "task"},
	}}
	if got, _ := shrinking.Selected(); got.Key != "what" {
		t.Fatalf("the hand-built form opens on %q", got.Key)
	}
	shrunk := shrinking.Cycle(1)
	if len(shrunk.Visible()) != 1 {
		t.Fatalf("answering the subject left %d fields visible", len(shrunk.Visible()))
	}
	if _, ok := shrunk.Selected(); !ok {
		t.Fatalf("the cursor sits at %d over %d fields, so it selects nothing",
			shrunk.Sel, len(shrunk.Visible()))
	}
}

func TestAMoveStopsAtTheEndsRatherThanWrapping(t *testing.T) {
	form := DeleteForm("infra-1", "", true, false)
	if up := form.Move(-1); up.Sel != 0 {
		t.Fatalf("moving up from the first field landed on %d", up.Sel)
	}
	if down := form.Move(99); down.Sel != len(form.Visible())-1 {
		t.Fatalf("moving down past the last field landed on %d", down.Sel)
	}
}

func TestFieldHintListsTheAlternativesAndCountsThemWhenItCannot(t *testing.T) {
	short := FormField{Options: switchOptions(), Value: noValue}
	if got := FieldHint(short); got != "no / yes" {
		t.Fatalf("a two-value switch hints %q", got)
	}
	long := make([]string, 0, 12)
	for i := range 12 {
		long = append(long, "infra-"+string(rune('a'+i))+"-dependency")
	}
	field := FormField{Options: long, Value: long[3]}
	if got := FieldHint(field); got != "4 of 12" {
		t.Fatalf("a long list hints %q, which does not say where in it the answer sits", got)
	}
	if got := FieldHint(FormField{Options: []string{"only"}}); got != "" {
		t.Fatalf("a field with one value hints %q", got)
	}
}

func TestTheTagFormOffersTheTenantsTagsAndNamesTheOnesOnTheTask(t *testing.T) {
	form := TagForm([]string{"urgent", "backend", "urgent", ""}, []string{"backend"}, true, true)
	field, ok := form.Selected()
	if !ok || field.Key != "tag" {
		t.Fatalf("the tag form opens on %q", field.Key)
	}
	if len(field.Options) != 2 || field.Options[0] != "backend" || field.Options[1] != "urgent" {
		t.Fatalf("the tag field offers %v, want the tenant's two tags once each in order", field.Options)
	}
	if !strings.Contains(form.Note, "backend") || strings.Contains(form.Note, "urgent") {
		t.Fatalf("the note says %q rather than naming only what the task carries", form.Note)
	}
	if form.Value("action") != "detach" {
		t.Fatalf("a tag the task already carries opens on %q", form.Value("action"))
	}
	if TagForm(nil, nil, true, true).Open() {
		t.Fatal("a tenant with no tags still opened a picker with nothing to pick")
	}
	// A reader who may look at tags and change none of them is offered no
	// picker: every value in it would be refused.
	if TagForm([]string{"urgent"}, nil, false, false).Open() {
		t.Fatal("a reader who may change no tag was offered a picker")
	}
	attachOnly := TagForm([]string{"backend"}, []string{"backend"}, true, false)
	if got := attachOnly.Value("action"); got != "attach" {
		t.Fatalf("a reader who may not detach was offered %q", got)
	}
	if field, _ := attachOnly.Selected(); len(field.Options) != 1 {
		t.Fatalf("the tag field offers %v", field.Options)
	}
}

func TestTheDependencyFormOffersEveryDependencyRatherThanTheFirstNine(t *testing.T) {
	refs := make([]string, 0, 12)
	for i := range 12 {
		refs = append(refs, "infra-"+string(rune('a'+i)))
	}
	form := DependencyForm(refs)
	field, _ := form.Selected()
	if len(field.Options) != 12 {
		t.Fatalf("the dependency field offers %d of %d", len(field.Options), len(refs))
	}
	if !DependencyForm(refs).Open() || DependencyForm(nil).Open() {
		t.Fatal("an empty dependency list opened a form")
	}
}

func TestTheTagPickerReachesTheServiceForBothDirections(t *testing.T) {
	tests := []struct {
		name    string
		presses []string
		check   func(*testing.T, *fakeService)
	}{
		{"detach the tag the task carries", nil, func(t *testing.T, f *fakeService) {
			if len(f.untagged) != 1 || f.untagged[0] != "backend" {
				t.Fatalf("untagged = %v", f.untagged)
			}
			if len(f.tagged) != 0 {
				t.Fatalf("a detach also attached %v", f.tagged)
			}
		}},
		{"attach a tag it does not", []string{"right"}, func(t *testing.T, f *fakeService) {
			if len(f.tagged) != 1 || f.tagged[0] != "urgent" {
				t.Fatalf("tagged = %v", f.tagged)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := threadModel(t)
			m, cmd := m.reduce(pressKey("L"))
			m, _ = m.reduce(run(t, cmd).(tagsMsg))
			if !m.form.Open() {
				t.Fatalf("L opened no tag form: %q", m.err)
			}
			for _, press := range tc.presses {
				m, _ = m.reduce(pressKey(press))
			}
			if tc.name == "attach a tag it does not" {
				// The cursor starts on the tag field; the second press moves to
				// the action and turns it around.
				m, _ = m.reduce(pressKey("down"))
				m, _ = m.reduce(pressKey("right"))
			}
			m, cmd = m.reduce(pressKey("enter"))
			if m.form.Open() {
				t.Fatal("the form stayed open after it was applied")
			}
			run(t, cmd)
			tc.check(t, svc)
		})
	}
}

func TestTheTagPickerSaysWhenTheTenantHasNoTags(t *testing.T) {
	m, svc := threadModel(t)
	svc.tags = nil
	m, cmd := m.reduce(pressKey("L"))
	m, _ = m.reduce(run(t, cmd).(tagsMsg))
	if m.form.Open() {
		t.Fatal("a picker opened with nothing to pick")
	}
	if !strings.Contains(m.err, "no tags exist yet") {
		t.Fatalf("err = %q", m.err)
	}
}

func TestRemovingADependencyReachesTheServiceWithTheChosenTask(t *testing.T) {
	m, svc := threadModel(t)
	m, _ = m.reduce(pressKey("-"))
	if !m.form.Open() {
		t.Fatalf("- opened no form: %q", m.err)
	}
	if got := formRow(t, m.View(), "remove dependency", "depends on"); !strings.Contains(got, "infra-7") {
		t.Fatalf("the dependency row reads %q", got)
	}
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "remove dependency", "depends on"); !strings.Contains(got, "infra-9") {
		t.Fatalf("cycling the dependency row left it reading %q", got)
	}
	m, cmd := m.reduce(pressKey("enter"))
	if m.form.Open() {
		t.Fatal("the form stayed open after it was applied")
	}
	run(t, cmd)
	if len(svc.depsRemoved) != 1 || svc.depsRemoved[0].Seq != 9 {
		t.Fatalf("depsRemoved = %+v", svc.depsRemoved)
	}
}

func TestRemovingADependencyNeedsTheTaskOpen(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	next, cmd := m.reduce(pressKey("-"))
	if next.form.Open() || cmd != nil {
		t.Fatal("the board offered a dependency form it has no list for")
	}
	if !strings.Contains(next.err, "open the task") {
		t.Fatalf("err = %q", next.err)
	}
}

func TestADependencyFormOnATaskThatWaitsOnNothingSaysSo(t *testing.T) {
	m, _ := threadModel(t)
	m.detail.deps = nil
	next, _ := m.reduce(pressKey("-"))
	if next.form.Open() || !strings.Contains(next.err, "waits on nothing") {
		t.Fatalf("form = %v err = %q", next.form.Open(), next.err)
	}
}

func TestTheFormRowDrawsItsOwnFieldAndTheValuesItCouldHold(t *testing.T) {
	m, _ := threadModel(t)
	m, _ = m.reduce(pressKey("X"))
	frame := m.View()
	what := formRow(t, frame, "delete", "what")
	if !strings.Contains(what, "task") {
		t.Fatalf("the subject row reads %q", what)
	}
	if !strings.Contains(what, "task / comment") {
		t.Fatalf("the selected row does not offer its alternatives: %q", what)
	}
	if got := formRow(t, frame, "delete", "permanently"); !strings.Contains(got, noValue) {
		t.Fatalf("the reach row reads %q", got)
	}
	// Cycling the subject to a comment must take the reach rows off the screen,
	// not merely stop reading them.
	m, _ = m.reduce(pressKey("right"))
	if n := formRows(t, m.View(), "delete", "permanently"); n != 0 {
		t.Fatalf("deleting a comment still draws %d reach rows", n)
	}
}

func TestEverySchemeDrivesTheFormAndTheConfirmation(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m, svc := threadModel(t)
			m = m.installScheme(string(scheme))
			if m.err != "" {
				t.Fatalf("installing %q: %s", scheme, m.err)
			}
			keys := m.keys
			m, _ = m.reduce(keyMsgFor(keys.Delete.Keys()[0]))
			if !m.form.Open() {
				t.Fatalf("%q does not open the delete form under %s", keys.Delete.Keys()[0], scheme)
			}
			m, _ = m.reduce(keyMsgFor(keys.Down.Keys()[0]))
			m, _ = m.reduce(keyMsgFor(keys.Right.Keys()[0]))
			if !m.form.Yes("hard") {
				t.Fatalf("under %s the form's own keys did not set the reach switch", scheme)
			}
			m, _ = m.reduce(keyMsgFor(keys.Accept.Keys()[0]))
			if !m.confirm.Open() {
				t.Fatalf("under %s accepting the form asked no question", scheme)
			}
			m, cmd := m.reduce(keyMsgFor(keys.Agree.Keys()[0]))
			if m.confirm.Open() {
				t.Fatalf("under %s the agreement key left the question standing", scheme)
			}
			run(t, cmd)
			if len(svc.deleted) != 1 || !svc.deleted[0].Hard {
				t.Fatalf("under %s the delete reached the service as %+v", scheme, svc.deleted)
			}
		})
	}
}

func TestEverySchemeCancelsTheFormAndTheConfirmation(t *testing.T) {
	for _, scheme := range Schemes() {
		t.Run(string(scheme), func(t *testing.T) {
			m, svc := threadModel(t)
			m = m.installScheme(string(scheme))
			keys := m.keys
			m, _ = m.reduce(keyMsgFor(keys.Delete.Keys()[0]))
			m, _ = m.reduce(keyMsgFor(keys.Cancel.Keys()[0]))
			if m.form.Open() {
				t.Fatalf("under %s the form survived a cancel", scheme)
			}
			m, _ = m.reduce(keyMsgFor(keys.Delete.Keys()[0]))
			m, _ = m.reduce(keyMsgFor(keys.Accept.Keys()[0]))
			m, cmd := m.reduce(keyMsgFor(keys.Cancel.Keys()[0]))
			if m.confirm.Open() {
				t.Fatalf("under %s the question survived a cancel", scheme)
			}
			if cmd != nil {
				cmd()
			}
			if len(svc.deleted) != 0 {
				t.Fatalf("under %s a cancelled confirmation deleted %+v", scheme, svc.deleted)
			}
		})
	}
}

func TestTheFormAndTheConfirmationDocumentTheKeysOfTheLoadedScheme(t *testing.T) {
	m, _ := threadModel(t)
	m = m.installScheme(string(SchemeNano))
	m, _ = m.reduce(keyMsgFor(m.keys.Delete.Keys()[0]))
	block := formBlock(t, m.View(), "delete")
	if !strings.Contains(block, "ctrl+o apply") {
		t.Fatalf("the form promises a key nano does not bind:\n%s", block)
	}
	m, _ = m.reduce(pressKey("enter"))
	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "y confirms") {
		t.Fatalf("the question does not name the key that answers it: %q", line)
	}
}
