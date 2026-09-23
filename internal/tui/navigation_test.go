// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// drillIn opens a board and then a task, leaving the model in the detail view
// with a full stack under it.
func drillIn(t *testing.T) Model {
	t.Helper()
	m := boardModel(t)
	m.svc = newFakeService()
	task, _ := TaskAt(m.columns, m.sel)
	m, _ = m.reduce(detailMsg{task: task})
	if m.view != viewDetail {
		t.Fatal("the detail view did not open")
	}
	return m
}

func TestBackPopsOneLevelAtATime(t *testing.T) {
	for _, back := range []string{"esc", "backspace"} {
		t.Run(back, func(t *testing.T) {
			m := drillIn(t)
			m, _ = m.reduce(pressKey(back))
			if m.view != viewBoard {
				t.Fatalf("back from the detail view landed on %v", m.view)
			}
			m, _ = m.reduce(pressKey(back))
			if m.view != viewProjects {
				t.Fatalf("back from the board landed on %v", m.view)
			}
		})
	}
}

func TestBackAtTheTopLevelDoesNothing(t *testing.T) {
	m := New(Config{})
	m, _ = m.reduce(projectsMsg{projects: []core.Project{{ID: "p1", Key: "infra"}}})
	next, cmd := m.reduce(pressKey("esc"))
	if next.view != viewProjects || cmd != nil {
		t.Fatalf("esc at the root moved to %v with cmd %v", next.view, cmd)
	}
}

func TestQuitPopsFromANestedViewAndQuitsAtTheTop(t *testing.T) {
	m := drillIn(t)
	m, cmd := m.reduce(pressKey("q"))
	if cmd != nil || m.view != viewBoard {
		t.Fatalf("q in the detail view quit outright: view %v cmd %v", m.view, cmd)
	}
	m, cmd = m.reduce(pressKey("q"))
	if cmd != nil || m.view != viewProjects {
		t.Fatalf("q on the board quit outright: view %v cmd %v", m.view, cmd)
	}
	_, cmd = m.reduce(pressKey("q"))
	if cmd == nil {
		t.Fatal("q at the top level did not quit")
	}
}

func TestAnOpenPromptSwallowsBackRatherThanPoppingTheView(t *testing.T) {
	for _, key := range []string{"e", "m", "#"} {
		t.Run(key, func(t *testing.T) {
			m := drillIn(t)
			m, _ = m.reduce(pressKey(key))
			if m.prompt == promptNone {
				t.Fatalf("key %q opened no prompt", key)
			}
			m, _ = m.reduce(pressKey("esc"))
			if m.prompt != promptNone {
				t.Fatal("esc did not cancel the prompt")
			}
			if m.view != viewDetail {
				t.Fatalf("cancelling a prompt also left the view: %v", m.view)
			}
		})
	}
}

func TestAnOpenPickerSwallowsBackRatherThanPoppingTheView(t *testing.T) {
	m := drillIn(t)
	m, _ = m.reduce(pressKey("P"))
	if m.choice == choiceNone {
		t.Fatal("the priority picker did not open")
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.choice != choiceNone {
		t.Fatal("esc did not cancel the picker")
	}
	if m.view != viewDetail {
		t.Fatalf("cancelling a picker also left the view: %v", m.view)
	}
}

func TestReopeningTheSameViewIsARefreshRatherThanAStepDeeper(t *testing.T) {
	m := boardModel(t)
	depth := len(m.stack)
	for range 3 {
		m, _ = m.reduce(tasksMsg{tasks: []core.Task{task("a", "todo", 1, core.PriorityNormal)}})
		m, _ = m.reduce(boardMsg{
			project:  core.Project{ID: "p1", Key: "infra"},
			workflow: testWorkflow(),
		})
	}
	if len(m.stack) != depth {
		t.Fatalf("reloading the board grew the way back to %d levels", len(m.stack))
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewProjects {
		t.Fatalf("one press did not reach the project list: %v", m.view)
	}
}

func TestTheProjectsKeyReturnsToTheRootInOnePress(t *testing.T) {
	m := drillIn(t)
	m, _ = m.reduce(pressKey("p"))
	if m.view != viewProjects || len(m.stack) != 0 {
		t.Fatalf("p left view %v with %d levels below it", m.view, len(m.stack))
	}
	if m.detail != nil {
		t.Fatal("returning to the root kept an open task")
	}
}

func TestLeavingTheDetailViewForgetsTheOpenTask(t *testing.T) {
	m := drillIn(t)
	m, _ = m.reduce(pressKey("esc"))
	if m.detail != nil {
		t.Fatal("the detail view was left with its task still open")
	}
}

func TestEveryViewAdvertisesTheWayBackOrTheWayOut(t *testing.T) {
	k := DefaultKeyMap()
	for _, v := range []viewKind{viewProjects, viewBoard, viewDetail, viewHelp} {
		var advertised []string
		for _, e := range k.ShortHelp(v, ActionContext{HasProject: true, HasTask: true, CanTransition: true}) {
			advertised = append(advertised, e.Keys)
		}
		joined := strings.Join(advertised, " ")
		if !strings.Contains(joined, "esc") && !strings.Contains(joined, "q") {
			t.Fatalf("view %v advertises no way out: %v", v, advertised)
		}
	}
}

func TestHelpDescribesTheViewItWasOpenedFrom(t *testing.T) {
	m := drillIn(t)
	m, _ = m.reduce(pressKey("?"))
	if m.view != viewHelp || m.underView() != viewDetail {
		t.Fatalf("help opened over %v, under %v", m.view, m.underView())
	}
	m, _ = m.reduce(pressKey("?"))
	if m.view != viewDetail {
		t.Fatalf("help did not close back onto the detail view: %v", m.view)
	}
	if m.detail == nil {
		t.Fatal("closing help discarded the open task")
	}
}

func TestTheHelpViewScrollsRatherThanLosingItsTop(t *testing.T) {
	m := boardModel(t)
	m.width, m.height = 100, 16
	m, _ = m.reduce(pressKey("?"))
	layout := LayoutFor(m.width, m.height, len(m.columns))
	first := m.helpLines(layout)
	if len(first) == 0 {
		t.Fatal("help rendered nothing")
	}
	if !strings.Contains(strings.Join(first, "\n"), "help") {
		t.Fatalf("the help view does not start at its top:\n%s", strings.Join(first, "\n"))
	}
	if !strings.Contains(strings.Join(first, "\n"), "more") {
		t.Fatal("help longer than the terminal gave no sign of it")
	}
	seen := map[string]bool{}
	for range 40 {
		for _, line := range m.helpLines(layout) {
			seen[strings.TrimSpace(line)] = true
		}
		m, _ = m.reduce(pressKey("j"))
	}
	for _, want := range []string{"card markers: @ claimed, ! blocked, + has dependencies, * has a due date"} {
		found := false
		for line := range seen {
			if strings.Contains(line, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("scrolling help never reached %q", want)
		}
	}
}

func TestReopeningHelpStartsAtTheTop(t *testing.T) {
	m := boardModel(t)
	m.width, m.height = 100, 16
	m, _ = m.reduce(pressKey("?"))
	for range 10 {
		m, _ = m.reduce(pressKey("j"))
	}
	if m.helpOff == 0 {
		t.Fatal("help did not scroll at all")
	}
	m, _ = m.reduce(pressKey("esc"))
	m, _ = m.reduce(pressKey("?"))
	if m.helpOff != 0 {
		t.Fatalf("help reopened at offset %d", m.helpOff)
	}
}
