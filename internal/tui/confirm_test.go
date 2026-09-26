// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
)

// confirmLine returns the one line the confirmation drew, found by the
// instruction only it renders. A search of the frame for the task's ref would
// find the title bar's copy and the card's copy of it, so the assertion that a
// question names its subject has to read the question.
func confirmLine(t *testing.T, frame, agree, cancel string) string {
	t.Helper()
	want := ConfirmHelp(agree, cancel)
	var found []string
	for _, l := range strings.Split(frame, "\n") {
		if strings.Contains(l, want) {
			found = append(found, l)
		}
	}
	if len(found) != 1 {
		t.Fatalf("the frame carries %d confirmation lines, want exactly one:\n%s", len(found), frame)
	}
	return found[0]
}

func TestAConfirmationNamesWhatItWillDo(t *testing.T) {
	tests := []struct {
		name    string
		confirm Confirm
		want    string
	}{
		{"a task", Confirm{Kind: confirmDeleteTask, Target: "infra-42"}, "delete task infra-42?"},
		{
			"a task and its subtasks, permanently",
			Confirm{Kind: confirmDeleteTask, Target: "infra-42", Note: DeleteNote(true, true)},
			"delete task infra-42, permanently, with its subtasks?",
		},
		{
			"a comment",
			Confirm{Kind: confirmDeleteComment, Target: "the comment by @ada from 09:30"},
			"delete the comment by @ada from 09:30?",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.confirm.Question(); got != tc.want {
				t.Fatalf("Question() = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestAConfirmationWithNothingNamedAsksNothing is the whole value of the
// primitive stated as a guard: "are you sure?" is a question about whatever
// happened to be selected, so a confirmation that cannot name its subject
// renders nothing and the interface refuses to open it.
func TestAConfirmationWithNothingNamedAsksNothing(t *testing.T) {
	if got := (Confirm{Kind: confirmDeleteTask}).Question(); got != "" {
		t.Fatalf("a confirmation with no target asked %q", got)
	}
	if got := (Confirm{Target: "infra-42"}).Question(); got != "" {
		t.Fatalf("a confirmation of nothing asked %q", got)
	}
	if (Confirm{}).Open() {
		t.Fatal("the zero confirmation reads as open")
	}
}

func TestDeleteNoteOnlySpeaksWhenTheDeleteReachesFurtherThanItLooks(t *testing.T) {
	cases := map[[2]bool]string{
		{false, false}: "",
		{true, false}:  "permanently",
		{false, true}:  "with its subtasks",
		{true, true}:   "permanently, with its subtasks",
	}
	for in, want := range cases {
		if got := DeleteNote(in[0], in[1]); got != want {
			t.Errorf("DeleteNote(%v, %v) = %q, want %q", in[0], in[1], got, want)
		}
	}
}

func TestTheConfirmationLineNamesTheTaskItWillDelete(t *testing.T) {
	m, _ := threadModel(t)
	task, _ := m.selectedTask()
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("enter"))
	if !m.confirm.Open() {
		t.Fatalf("accepting the delete form asked no question: %q", m.err)
	}
	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, task.Ref) {
		t.Fatalf("the question does not name the task it will delete: %q", line)
	}
	if !strings.Contains(line, "delete task") {
		t.Fatalf("the question does not say what it will do: %q", line)
	}
}

func TestOnlyTheAgreementKeyRunsADestructiveAction(t *testing.T) {
	for _, press := range []string{"enter", "n", "d", "right", "q"} {
		t.Run(press, func(t *testing.T) {
			m, svc := threadModel(t)
			m, _ = m.reduce(pressKey("X"))
			m, _ = m.reduce(pressKey("enter"))
			next, cmd := m.reduce(pressKey(press))
			if !next.confirm.Open() {
				t.Fatalf("%q answered the question", press)
			}
			if cmd != nil {
				cmd()
			}
			if len(svc.deletedRefs) != 0 {
				t.Fatalf("%q deleted %+v", press, svc.deletedRefs)
			}
		})
	}
}

func TestAgreeingDeletesTheTaskAsFarAsTheFormSaid(t *testing.T) {
	m, svc := threadModel(t)
	task, _ := m.selectedTask()
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	line := formRow(t, m.View(), "delete", "with subtasks")
	if !strings.Contains(line, yesValue) {
		t.Fatalf("the subtask row reads %q", line)
	}
	m, _ = m.reduce(pressKey("enter"))
	if note := m.confirm.Note; note != "permanently, with its subtasks" {
		t.Fatalf("the question's note reads %q", note)
	}
	m, cmd := m.reduce(pressKey("y"))
	if m.confirm.Open() {
		t.Fatal("the question stayed open after it was agreed to")
	}
	run(t, cmd)
	if len(svc.deleted) != 1 || !svc.deleted[0].Hard || !svc.deleted[0].Cascade {
		t.Fatalf("deleted = %+v", svc.deleted)
	}
	if len(svc.deletedRefs) != 1 || svc.deletedRefs[0].ID != task.ID {
		t.Fatalf("deleted the wrong task: %+v", svc.deletedRefs)
	}
}

func TestDeletingTheOpenTaskLeavesItsDetailView(t *testing.T) {
	m, _ := threadModel(t)
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("enter"))
	m, cmd := m.reduce(pressKey("y"))
	msg := run(t, cmd)
	m, _ = m.reduce(msg)
	if m.view == viewDetail {
		t.Fatal("the interface stayed on the detail view of a task it had just deleted")
	}
	if m.detail != nil {
		t.Fatal("the deleted task is still open")
	}
}

func TestDeletingTheSelectedCommentNamesItAndRemovesIt(t *testing.T) {
	m, svc := threadModel(t)
	m, _ = m.reduce(pressKey("right"))
	m, _ = m.reduce(pressKey("X"))
	m, _ = m.reduce(pressKey("right"))
	if got := formRow(t, m.View(), "delete", "what"); !strings.Contains(got, "comment") {
		t.Fatalf("the subject row reads %q", got)
	}
	m, _ = m.reduce(pressKey("enter"))
	line := confirmLine(t, m.View(), m.keys.Agree.Help().Key, m.keys.Cancel.Help().Key)
	if !strings.Contains(line, "grace") {
		t.Fatalf("the question does not name the comment's author: %q", line)
	}
	m, cmd := m.reduce(pressKey("y"))
	if m.confirm.Open() {
		t.Fatal("the question stayed open after it was agreed to")
	}
	run(t, cmd)
	if len(svc.commentsGone) != 1 || svc.commentsGone[0] != "c-2" {
		t.Fatalf("commentsGone = %v, want the selected comment", svc.commentsGone)
	}
	if len(svc.deletedRefs) != 0 {
		t.Fatalf("deleting a comment deleted the task too: %+v", svc.deletedRefs)
	}
}

func TestTheDeleteKeyOffersNoSubjectTheReaderMayNotRemove(t *testing.T) {
	m, _ := threadModel(t)
	m.allowed = ActionAccess{"DeleteComment": true}
	m, _ = m.reduce(pressKey("X"))
	if !m.form.Open() {
		t.Fatalf("a reader who may delete a comment was offered nothing: %q", m.err)
	}
	if got := formRow(t, m.View(), "delete", "what"); strings.Contains(got, "task") {
		t.Fatalf("a reader who may not delete a task was offered one: %q", got)
	}

	none, _ := threadModel(t)
	none.allowed = ActionAccess{}
	next, cmd := none.reduce(pressKey("X"))
	if next.form.Open() || cmd != nil {
		t.Fatal("a reader who may delete nothing was offered a delete form")
	}
}
