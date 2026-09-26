// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
)

// threadHeaders returns the author lines of the comment thread and nothing else:
// the section is located by its own heading, and within it only the lines at the
// comment's own indentation are kept, because a comment's body is indented
// further and a card on the board carries the very same selection marker. An
// assertion that searched the frame for the marker would pass on the selected
// card and say nothing about the thread.
func threadHeaders(t *testing.T, frame string) []string {
	t.Helper()
	lines := strings.Split(frame, "\n")
	start := -1
	for i, l := range lines {
		trimmed := strings.TrimSpace(l)
		if strings.HasPrefix(trimmed, "comments (") && strings.HasSuffix(trimmed, "):") {
			start = i + 1
			break
		}
	}
	if start < 0 {
		t.Fatalf("the frame draws no comment section:\n%s", frame)
	}
	var out []string
	for _, l := range lines[start:] {
		if strings.TrimSpace(l) == "" {
			break
		}
		if strings.HasPrefix(l, "    ") {
			continue
		}
		out = append(out, l)
	}
	if len(out) == 0 {
		t.Fatalf("the comment section drew no comments:\n%s", frame)
	}
	return out
}

// selectedHeader is the index of the marked comment among the thread's headers.
func selectedHeader(t *testing.T, frame string) int {
	t.Helper()
	marked := -1
	for i, l := range threadHeaders(t, frame) {
		if !strings.Contains(l, SelectionMarker(true)) {
			continue
		}
		if marked >= 0 {
			t.Fatalf("two comments are marked as selected:\n%s", strings.Join(threadHeaders(t, frame), "\n"))
		}
		marked = i
	}
	if marked < 0 {
		t.Fatalf("no comment is marked as selected:\n%s", strings.Join(threadHeaders(t, frame), "\n"))
	}
	return marked
}

func TestTheThreadMarksTheCommentTheActionsWillActOn(t *testing.T) {
	m, _ := threadModel(t)
	if got := selectedHeader(t, m.View()); got != 0 {
		t.Fatalf("the thread opens with comment %d marked", got)
	}
	if !strings.Contains(threadHeaders(t, m.View())[0], "ada") {
		t.Fatalf("the first header names the wrong author: %q", threadHeaders(t, m.View())[0])
	}
}

func TestTheColumnKeysStepThroughTheThreadInTheDetailView(t *testing.T) {
	m, _ := threadModel(t)
	m, _ = m.reduce(pressKey("right"))
	if got := selectedHeader(t, m.View()); got != 1 {
		t.Fatalf("stepping forward marked comment %d", got)
	}
	m, _ = m.reduce(pressKey("right"))
	if got := selectedHeader(t, m.View()); got != 1 {
		t.Fatalf("stepping past the last comment marked %d", got)
	}
	m, _ = m.reduce(pressKey("left"))
	m, _ = m.reduce(pressKey("left"))
	if got := selectedHeader(t, m.View()); got != 0 {
		t.Fatalf("stepping back past the first comment marked %d", got)
	}
}

// TestTheDetailViewOffersTheThreadCursorOnlyWhenThereIsAThread keeps the footer
// honest: the column keys mean something else on the board, and a task with no
// comments has nothing to step through.
func TestTheDetailViewOffersTheThreadCursorOnlyWhenThereIsAThread(t *testing.T) {
	k := DefaultKeyMap()
	with := k.ShortHelp(viewDetail, ActionContext{May: permitAll, HasTask: true, HasComment: true})
	without := k.ShortHelp(viewDetail, ActionContext{May: permitAll, HasTask: true})
	descs := func(entries []HelpEntry) []string {
		out := make([]string, 0, len(entries))
		for _, e := range entries {
			out = append(out, e.Desc)
		}
		return out
	}
	if !slices.Contains(descs(with), "next comment") {
		t.Fatalf("a task with a thread does not offer the cursor: %v", descs(with))
	}
	if slices.Contains(descs(without), "next comment") {
		t.Fatalf("a task with no thread offers a cursor over nothing: %v", descs(without))
	}
	board := descs(k.ShortHelp(viewBoard, ActionContext{May: permitAll, HasTask: true, HasComment: true}))
	if slices.Contains(board, "next comment") {
		t.Fatalf("the board relabelled its column keys as a thread cursor: %v", board)
	}
}

func TestEditingTheSelectedCommentOpensOnItsOwnBody(t *testing.T) {
	m, svc := threadModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("M"))
	if m.prompt != promptCommentEdit {
		t.Fatalf("M opened prompt %v: %q", m.prompt, m.err)
	}
	if got := m.input.Value(); got != "the migration landed" {
		t.Fatalf("the edit opened on %q rather than on the selected comment", got)
	}
	m.input.SetValue("the migration landed on Tuesday")
	m, cmd := m.reduce(pressKey("enter"))
	if m.prompt != promptNone {
		t.Fatal("the prompt stayed open")
	}
	run(t, cmd)
	if len(svc.commentsEdited) != 1 {
		t.Fatalf("commentsEdited = %+v", svc.commentsEdited)
	}
	if svc.commentsEdited[0][0] != "c-2" {
		t.Fatalf("the edit reached comment %q rather than the selected one", svc.commentsEdited[0][0])
	}
	if svc.commentsEdited[0][1] != "the migration landed on Tuesday" {
		t.Fatalf("the edit sent %q", svc.commentsEdited[0][1])
	}
}

func TestEditingACommentRefusesWhereThereIsNoneToEdit(t *testing.T) {
	board := boardModel(t)
	board.svc = newFakeService()
	next, cmd := board.reduce(pressKey("M"))
	if next.prompt != promptNone || cmd != nil {
		t.Fatal("the board opened a comment edit with no thread in view")
	}
	if !strings.Contains(next.err, "open the task") {
		t.Fatalf("err = %q", next.err)
	}

	empty, _ := threadModel(t)
	empty.detail.comments = nil
	next, _ = empty.reduce(pressKey("M"))
	if next.prompt != promptNone {
		t.Fatal("a task with no comments opened an edit")
	}
	if !strings.Contains(next.err, "no comments") {
		t.Fatalf("err = %q", next.err)
	}
}

// TestTheNewActionsAreOfferedOnlyToAReaderWhoMayUseThem is the scope rule
// applied to the affordances this change added: the footer, the help overlay and
// the keystroke all have to agree, because an interface that shows a key and
// then reports the service's refusal has told the reader the refusal was theirs.
func TestTheNewActionsAreOfferedOnlyToAReaderWhoMayUseThem(t *testing.T) {
	k := DefaultKeyMap()
	cases := []struct {
		name    string
		method  string
		binding string
	}{
		{"remove a dependency", "RemoveDependency", k.Undepend.Help().Key},
		{"edit a comment", "EditComment", k.CommentEdit.Help().Key},
		{"pick a tag", "AddTag", k.Tags.Help().Key},
		{"delete", "DeleteTask", k.Delete.Help().Key},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			offered := keysOf(k.ViewHelp(viewDetail, permitAll))
			if !slices.Contains(offered, tc.binding) {
				t.Fatalf("a reader who may do everything is not told about %q: %v", tc.binding, offered)
			}
			refused := keysOf(k.ViewHelp(viewDetail, permitOnly()))
			if slices.Contains(refused, tc.binding) {
				t.Fatalf("a reader who may do nothing is told about %q: %v", tc.binding, refused)
			}
			m, svc := threadModel(t)
			m.allowed = ActionAccess{}
			before := m.View()
			next, cmd := m.reduce(pressKey(tc.binding))
			if cmd != nil {
				cmd()
			}
			if next.form.Open() || next.prompt != promptNone || next.confirm.Open() {
				t.Fatalf("%q offered an affordance to a reader refused %s", tc.binding, tc.method)
			}
			if next.View() != before {
				t.Fatalf("a refused key changed the screen:\n%s\n---\n%s", before, next.View())
			}
			if len(svc.deleted)+len(svc.depsRemoved)+len(svc.commentsEdited) != 0 {
				t.Fatalf("a refused key reached the service")
			}
		})
	}
}

// keysOf is the key column of a help listing.
func keysOf(entries []HelpEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Keys)
	}
	return out
}

// TestEveryOperationThisChangeBoundHasAKeyThatReachesIt is the half of parity
// the registry cannot check: the registry records that an operation is reachable
// from a view, and only pressing the key proves that it is. A binding recorded
// and not wired reads as parity and is not.
func TestEveryOperationThisChangeBoundHasAKeyThatReachesIt(t *testing.T) {
	bound := []string{"DeleteTask", "DeleteComment", "RemoveDependency", "EditComment", "ListTags"}
	for _, method := range bound {
		op, ok := capability.ByMethod(method)
		if !ok {
			t.Fatalf("%s is not in the registry", method)
		}
		if !op.Binds(capability.SurfaceTUI) {
			t.Errorf("%s: the registry records no tui binding", op.Name)
		}
		if _, exempted := op.ExemptionFor(capability.SurfaceTUI); exempted {
			t.Errorf("%s: the registry both binds and exempts the terminal", op.Name)
		}
	}
	k := DefaultKeyMap()
	for _, method := range bound {
		found := false
		for _, action := range k.taskBindings() {
			if slices.Contains(action.methods, method) || slices.Contains(action.needs, method) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is bound in the registry and no key acts through it", method)
		}
	}
}

// TestTheTagPickerNeedsBothTheListingAndAChange is the rule an any-of gate would
// have got wrong in both directions: a reader who may only look at tags would be
// offered a picker every value of which the service refuses, and a reader who may
// change tags without reading them would be offered a picker with nothing in it.
func TestTheTagPickerNeedsBothTheListingAndAChange(t *testing.T) {
	k := DefaultKeyMap()
	offered := func(may func(string) bool) bool {
		return slices.Contains(keysOf(k.ViewHelp(viewDetail, may)), k.Tags.Help().Key)
	}
	if !offered(permitOnly("ListTags", "AddTag")) {
		t.Error("a reader who may list and attach is not offered the picker")
	}
	if !offered(permitOnly("ListTags", "RemoveTag")) {
		t.Error("a reader who may list and detach is not offered the picker")
	}
	if offered(permitOnly("ListTags")) {
		t.Error("a reader who may change no tag is offered the picker")
	}
	if offered(permitOnly("AddTag", "RemoveTag")) {
		t.Error("a reader who may not list tags is offered a picker with nothing to list")
	}
}
