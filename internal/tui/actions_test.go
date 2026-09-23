// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// typeInto opens a prompt with a key, types text into it and accepts it,
// returning the model and whatever the acceptance asked to be run.
func typeInto(t *testing.T, m Model, open, text string) (Model, tea.Cmd) {
	t.Helper()
	m, _ = m.reduce(pressKey(open))
	if m.prompt == promptNone {
		t.Fatalf("key %q opened no prompt", open)
	}
	m.input.SetValue(text)
	return m.reduce(pressKey("enter"))
}

func TestPromptsReachTheService(t *testing.T) {
	tests := []struct {
		name  string
		key   string
		text  string
		check func(*testing.T, *fakeService)
	}{
		{"new task", "n", "write the release notes", func(t *testing.T, f *fakeService) {
			if len(f.created) != 1 || f.created[0].Title != "write the release notes" {
				t.Fatalf("created = %+v", f.created)
			}
			if f.created[0].ProjectRef != "infra" {
				t.Fatalf("a task was created outside the open project: %+v", f.created[0])
			}
		}},
		{"edit title", "e", "a better title", func(t *testing.T, f *fakeService) {
			if len(f.updated) != 1 || f.updated[0].Title == nil || *f.updated[0].Title != "a better title" {
				t.Fatalf("updated = %+v", f.updated)
			}
		}},
		{"edit body", "E", "what actually happened", func(t *testing.T, f *fakeService) {
			if len(f.updated) != 1 || f.updated[0].Body == nil || *f.updated[0].Body != "what actually happened" {
				t.Fatalf("updated = %+v", f.updated)
			}
		}},
		{"set assignee", "A", "u-42", func(t *testing.T, f *fakeService) {
			if len(f.updated) != 1 || f.updated[0].AssigneeActorID == nil || *f.updated[0].AssigneeActorID != "u-42" {
				t.Fatalf("updated = %+v", f.updated)
			}
		}},
		{"comment", "m", "this is blocked on the migration", func(t *testing.T, f *fakeService) {
			if len(f.comments) != 1 || f.comments[0] != "this is blocked on the migration" {
				t.Fatalf("comments = %+v", f.comments)
			}
		}},
		{"add tag", "#", "urgent", func(t *testing.T, f *fakeService) {
			if len(f.tagged) != 1 || f.tagged[0] != "urgent" {
				t.Fatalf("tagged = %+v", f.tagged)
			}
		}},
		{"remove tag", "U", "urgent", func(t *testing.T, f *fakeService) {
			if len(f.untagged) != 1 || f.untagged[0] != "urgent" {
				t.Fatalf("untagged = %+v", f.untagged)
			}
		}},
		{"add dependency", "D", "infra-7", func(t *testing.T, f *fakeService) {
			if len(f.deps) != 1 || f.deps[0].ProjectKey != "infra" || f.deps[0].Seq != 7 {
				t.Fatalf("deps = %+v", f.deps)
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := boardModel(t)
			svc := newFakeService()
			m.svc = svc
			m, cmd := typeInto(t, m, tc.key, tc.text)
			if m.prompt != promptNone {
				t.Fatalf("the prompt stayed open: %v", m.prompt)
			}
			if cmd == nil {
				t.Fatal("accepting the prompt asked for nothing to be done")
			}
			cmd()
			tc.check(t, svc)
		})
	}
}

func TestEveryPromptIsAlsoReachableFromTheDetailView(t *testing.T) {
	for _, key := range []string{"e", "E", "A", "m", "#", "U", "D"} {
		t.Run(key, func(t *testing.T) {
			m := boardModel(t)
			m.svc = newFakeService()
			task, _ := TaskAt(m.columns, m.sel)
			m, _ = m.reduce(detailMsg{task: task})
			if m.view != viewDetail {
				t.Fatal("the detail view did not open")
			}
			next, _ := m.reduce(pressKey(key))
			if next.prompt == promptNone {
				t.Fatalf("key %q does nothing in the detail view", key)
			}
		})
	}
}

func TestAnEmptyPromptCancelsRatherThanSendingABlankEdit(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := typeInto(t, m, "m", "   ")
	if m.prompt != promptNone {
		t.Fatal("the prompt stayed open")
	}
	if cmd != nil {
		cmd()
	}
	if len(svc.comments) != 0 {
		t.Fatalf("a blank comment was sent: %+v", svc.comments)
	}
}

func TestPromptsSeedThemselvesWithWhatTheyWouldReplace(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	next, _ := m.reduce(pressKey("e"))
	task, _ := TaskAt(m.columns, m.sel)
	if next.input.Value() != task.Title {
		t.Fatalf("the title prompt opened on %q, not on %q", next.input.Value(), task.Title)
	}
	next, _ = m.reduce(pressKey("m"))
	if next.input.Value() != "" {
		t.Fatalf("the comment prompt opened on %q", next.input.Value())
	}
}

func TestAPromptThatNeedsATaskRefusesWithoutOne(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m.columns = nil
	next, cmd := m.reduce(pressKey("m"))
	if next.prompt != promptNone || cmd != nil {
		t.Fatal("a comment prompt opened with no task selected")
	}
	if !strings.Contains(next.err, "no task") {
		t.Fatalf("err = %q", next.err)
	}
}

func TestNewTaskRefusesWithNoProjectOpen(t *testing.T) {
	m := New(Config{})
	m.svc = newFakeService()
	m.view = viewBoard
	next, _ := m.reduce(pressKey("n"))
	if next.prompt != promptNone || !strings.Contains(next.err, "open a project") {
		t.Fatalf("prompt = %v err = %q", next.prompt, next.err)
	}
}

func TestEscapeCancelsEveryPrompt(t *testing.T) {
	for _, key := range []string{"n", "e", "E", "A", "m", "#", "U", "D", "/"} {
		t.Run(key, func(t *testing.T) {
			m := boardModel(t)
			m.svc = newFakeService()
			m, _ = m.reduce(pressKey(key))
			m, _ = m.reduce(pressKey("esc"))
			if m.prompt != promptNone {
				t.Fatalf("prompt %v survived a cancel", m.prompt)
			}
		})
	}
}

func TestPriorityPickerSetsThePriority(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, _ = m.reduce(pressKey("P"))
	if m.choice != choicePriority || len(m.choices) != 5 {
		t.Fatalf("choice = %v choices = %+v", m.choice, m.choices)
	}
	m, cmd := m.reduce(pressKey("1"))
	if m.choice != choiceNone || cmd == nil {
		t.Fatalf("choice = %v cmd = %v", m.choice, cmd)
	}
	cmd()
	if len(svc.updated) != 1 || svc.updated[0].Priority == nil || *svc.updated[0].Priority != core.PriorityHighest {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

func TestPriorityChoicesCoverTheWholeRange(t *testing.T) {
	choices := PriorityChoices()
	if len(choices) != 5 {
		t.Fatalf("choices = %+v", choices)
	}
	for i, c := range choices {
		if c.Label != PriorityLabel(core.Priority(i+1)) {
			t.Fatalf("choice %d is %q", i, c.Label)
		}
	}
	if _, ok := ChoiceAt(choices, "6"); ok {
		t.Fatal("an out of range press picked an option")
	}
	if picked, ok := ChoiceAt(choices, "3"); !ok || picked.Value != "3" {
		t.Fatalf("ChoiceAt(3) = %+v %v", picked, ok)
	}
}

func TestAMalformedDependencyRefIsReportedRatherThanSent(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	_, cmd := typeInto(t, m, "D", "not a ref at all")
	if cmd == nil {
		t.Fatal("nothing was reported")
	}
	msg, ok := cmd().(actionMsg)
	if !ok || msg.err == nil {
		t.Fatalf("msg = %+v", msg)
	}
	if len(svc.deps) != 0 {
		t.Fatalf("a malformed ref reached the service: %+v", svc.deps)
	}
}

func TestAFailedActionIsExplainedInTheServicesWords(t *testing.T) {
	m := boardModel(t)
	m.project = core.Project{ID: "p1", Key: "infra"}
	next, _ := m.reduce(actionMsg{
		kind: actionUpdate,
		ref:  core.TaskRef{ID: "a"},
		err:  core.Invalid("task title must be at most 500 characters"),
	})
	if !strings.Contains(next.err, "at most 500 characters") {
		t.Fatalf("err = %q", next.err)
	}
	if !strings.Contains(next.err, "edit the task") {
		t.Fatalf("the refusal did not name the action: %q", next.err)
	}
}

func TestEveryActionKindNamesItself(t *testing.T) {
	for kind := actionClaim; kind <= actionNewProject; kind++ {
		if kind.Label() == "" || kind.Past() == "" {
			t.Fatalf("action %d is unnamed: %q / %q", kind, kind.Label(), kind.Past())
		}
	}
	if actionClaim.Mutates() || actionRelease.Mutates() {
		t.Fatal("a lease change was treated as an edit of the task")
	}
	if !actionUpdate.Mutates() || !actionComment.Mutates() {
		t.Fatal("an edit was treated as though it changed nothing")
	}
}

func TestEveryPromptIntroducesItself(t *testing.T) {
	for kind := promptFilter; kind <= promptNewProject; kind++ {
		spec, ok := kind.Spec()
		if !ok {
			t.Fatalf("prompt %d has no spec", kind)
		}
		if spec.Prompt == "" || spec.Placeholder == "" || spec.Limit <= 0 {
			t.Fatalf("prompt %d is underspecified: %+v", kind, spec)
		}
	}
	if _, ok := promptNone.Spec(); ok {
		t.Fatal("the closed prompt described itself as open")
	}
	if promptFilter.NeedsTask() || promptNewTask.NeedsTask() {
		t.Fatal("a prompt that acts on the board was said to need a task")
	}
	if !promptComment.NeedsTask() || !promptTag.NeedsTask() {
		t.Fatal("a prompt that acts on a task was said not to need one")
	}
}

func TestChoiceKindsIntroduceThemselves(t *testing.T) {
	if choiceTransition.Prompt() == "" || choicePriority.Prompt() == "" {
		t.Fatal("a picker opens without saying what it is picking")
	}
	if choiceNone.Prompt() != "" {
		t.Fatal("the closed picker described itself as open")
	}
}

func TestTransitionChoicesNameEveryPermittedState(t *testing.T) {
	choices := TransitionChoices(testWorkflow(), "todo")
	if len(choices) != 1 || choices[0].Value != "doing" {
		t.Fatalf("choices = %+v", choices)
	}
	if choices[0].Label == "" {
		t.Fatal("a state was offered without a label")
	}
	if got := TransitionChoices(nil, "todo"); len(got) != 0 {
		t.Fatalf("a nil workflow offered %+v", got)
	}
	if labels := ChoiceLabels(choices); len(labels) != 1 || !strings.HasPrefix(labels[0], "1) ") {
		t.Fatalf("labels = %+v", labels)
	}
}

func TestAnEditInTheDetailViewFetchesTheTaskAgain(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	task, _ := TaskAt(m.columns, m.sel)
	m, _ = m.reduce(detailMsg{task: task})
	next, cmd := m.reduce(actionMsg{kind: actionComment, ref: core.TaskRef{ID: task.ID}})
	if cmd == nil {
		t.Fatal("an edit in the detail view reloaded nothing")
	}
	if !strings.Contains(next.status, "commented on") {
		t.Fatalf("status = %q", next.status)
	}
}

func TestClaimNextAsksTheQueueForWork(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := m.reduce(pressKey("N"))
	if cmd == nil {
		t.Fatal("claim next asked for nothing")
	}
	msg, ok := cmd().(actionMsg)
	if !ok || msg.kind != actionClaim {
		t.Fatalf("msg = %+v", msg)
	}
	if !svc.claimedNext {
		t.Fatal("the queue was never asked")
	}
	_ = m
}

func TestRenewRefusesWithoutALeaseAndSucceedsWithOne(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	next, cmd := m.reduce(pressKey("R"))
	if cmd != nil || !strings.Contains(next.err, "does not hold a lease") {
		t.Fatalf("err = %q cmd = %v", next.err, cmd)
	}
	task, _ := TaskAt(m.columns, m.sel)
	m.leases[task.ID] = "token"
	_, cmd = m.reduce(pressKey("R"))
	if cmd == nil {
		t.Fatal("renew asked for nothing with a lease held")
	}
	cmd()
	if svc.renewed != 1 {
		t.Fatalf("renewed = %d", svc.renewed)
	}
}

func TestANewProjectIsCreatedFromTheProjectList(t *testing.T) {
	m := projectListModel(t, 3, 130, 24)
	svc := newFakeService()
	m.svc = svc
	m, cmd := typeInto(t, m, "n", "infra  Infrastructure Team")
	if cmd == nil {
		t.Fatal("creating a project asked for nothing")
	}
	cmd()
	if len(svc.projectsMade) != 1 {
		t.Fatalf("created = %+v", svc.projectsMade)
	}
	if svc.projectsMade[0].Key != "infra" || svc.projectsMade[0].Name != "Infrastructure Team" {
		t.Fatalf("created = %+v", svc.projectsMade[0])
	}
	_ = m
}

func TestSplitProjectEntry(t *testing.T) {
	tests := []struct {
		in        string
		key, name string
	}{
		{"infra Infrastructure", "infra", "Infrastructure"},
		{"infra   Two  Words", "infra", "Two Words"},
		{"infra", "infra", ""},
		{"   ", "", ""},
	}
	for _, tc := range tests {
		key, name := SplitProjectEntry(tc.in)
		if key != tc.key || name != tc.name {
			t.Fatalf("SplitProjectEntry(%q) = %q,%q want %q,%q", tc.in, key, name, tc.key, tc.name)
		}
	}
}

func TestCreatingAProjectReloadsTheProjectList(t *testing.T) {
	m := projectListModel(t, 3, 130, 24)
	m.svc = newFakeService()
	next, cmd := m.reduce(actionMsg{kind: actionNewProject, ref: core.TaskRef{ID: "infra"}})
	if cmd == nil {
		t.Fatal("a created project did not refresh the list")
	}
	if !strings.Contains(next.status, "created") {
		t.Fatalf("status = %q", next.status)
	}
}

func TestTheStatusBarNamesATaskTheWayAPersonReadsIt(t *testing.T) {
	m := boardModel(t)
	m.project = core.Project{ID: "p1", Key: "infra"}
	next, _ := m.reduce(actionMsg{
		kind:  actionComment,
		ref:   core.TaskRef{ID: "01ABCDEF0123456789"},
		label: "infra-42",
	})
	if strings.Contains(next.status, "01ABCDEF") {
		t.Fatalf("the status bar showed a raw identifier: %q", next.status)
	}
	if !strings.Contains(next.status, "infra-42") {
		t.Fatalf("status = %q", next.status)
	}
}

func TestAnActionWithoutALabelStillNamesItself(t *testing.T) {
	msg := actionMsg{kind: actionClaim, ref: core.TaskRef{ProjectKey: "infra", Seq: 7}}
	if msg.name() != "infra-7" {
		t.Fatalf("name = %q", msg.name())
	}
	if (actionMsg{kind: actionNewProject, label: "infra"}).name() != "infra" {
		t.Fatal("a label was ignored in favour of an empty ref")
	}
}

func TestEveryActionThatNamesATaskCarriesItsRef(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	task, _ := TaskAt(m.columns, m.sel)
	for _, tc := range []struct {
		name string
		key  string
		text string
	}{
		{"comment", "m", "note"},
		{"tag", "#", "urgent"},
		{"untag", "U", "urgent"},
		{"title", "e", "renamed"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := typeInto(t, m, tc.key, tc.text)
			msg, ok := cmd().(actionMsg)
			if !ok || msg.label != task.Ref {
				t.Fatalf("msg = %+v, want label %q", msg, task.Ref)
			}
		})
	}
}
