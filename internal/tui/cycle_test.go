// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// priorityBoard shows one task at a chosen priority, so a cycling assertion
// starts from the place it names rather than from whatever the shared board
// happens to hold.
func priorityBoard(t *testing.T, p core.Priority) (Model, *fakeService) {
	t.Helper()
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks:    []core.Task{task("a", "todo", 1, p)},
	})
	svc := newFakeService()
	m.svc = svc
	return m, svc
}

// TestNextPriorityWraps holds the step itself, away from the keystroke that
// drives it. It wraps rather than stopping, because a cycle that stopped at the
// bottom would leave a reader who overshot unable to climb back.
func TestNextPriorityWraps(t *testing.T) {
	for _, tc := range []struct {
		name string
		from core.Priority
		want core.Priority
	}{
		{"highest", core.PriorityHighest, core.PriorityHigh},
		{"high", core.PriorityHigh, core.PriorityNormal},
		{"normal", core.PriorityNormal, core.PriorityLow},
		{"low", core.PriorityLow, core.PriorityLowest},
		{"lowest wraps to highest", core.PriorityLowest, core.PriorityHighest},
		{"unset lands on a valid priority", core.Priority(0), core.PriorityHighest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := NextPriority(tc.from); got != tc.want {
				t.Fatalf("NextPriority(%d) = %d, want %d", tc.from, got, tc.want)
			}
		})
	}
}

// TestCyclingReachesEveryPriority is the point of wrapping: five steps from
// anywhere visit all five and come back.
func TestCyclingReachesEveryPriority(t *testing.T) {
	seen := map[core.Priority]bool{}
	p := core.PriorityNormal
	for range 5 {
		p = NextPriority(p)
		seen[p] = true
	}
	for want := core.PriorityHighest; want <= core.PriorityLowest; want++ {
		if !seen[want] {
			t.Errorf("cycling never reached %s", PriorityLabel(want))
		}
	}
	if p != core.PriorityNormal {
		t.Fatalf("five steps landed on %s, not back where they started", PriorityLabel(p))
	}
}

// TestCyclingPriorityStepsOnePlaceDownAndWrapsAtTheBottom is the whole of what
// p promises at the keyboard: one press, nothing to answer, and P5 back round
// to P1.
func TestCyclingPriorityStepsOnePlaceDownAndWrapsAtTheBottom(t *testing.T) {
	for _, tc := range []struct {
		name string
		from core.Priority
		want core.Priority
	}{
		{"highest", core.PriorityHighest, core.PriorityHigh},
		{"high", core.PriorityHigh, core.PriorityNormal},
		{"normal", core.PriorityNormal, core.PriorityLow},
		{"low", core.PriorityLow, core.PriorityLowest},
		{"lowest wraps", core.PriorityLowest, core.PriorityHighest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := priorityBoard(t, tc.from)
			next, cmd := m.reduce(pressKey("p"))
			if next.choice != choiceNone || next.prompt != promptNone {
				t.Fatalf("p opened something to answer: choice %v prompt %v", next.choice, next.prompt)
			}
			if next.view != viewBoard {
				t.Fatalf("p left the board for view %v", next.view)
			}
			if cmd == nil {
				t.Fatal("p asked for nothing to be done")
			}
			cmd()
			if len(svc.updated) != 1 || svc.updated[0].Priority == nil {
				t.Fatalf("updated = %+v", svc.updated)
			}
			if got := *svc.updated[0].Priority; got != tc.want {
				t.Fatalf("p on %s sent %s, want %s",
					PriorityLabel(tc.from), PriorityLabel(got), PriorityLabel(tc.want))
			}
		})
	}
}

// TestCyclingPriorityReachesTheDetailViewToo keeps the board and the open task
// running one set of task actions.
func TestCyclingPriorityReachesTheDetailViewToo(t *testing.T) {
	m, svc := priorityBoard(t, core.PriorityNormal)
	open, _ := TaskAt(m.columns, m.sel)
	m, _ = m.reduce(detailMsg{task: open})
	if m.view != viewDetail {
		t.Fatal("the detail view did not open")
	}
	_, cmd := m.reduce(pressKey("p"))
	if cmd == nil {
		t.Fatal("p does nothing in the detail view")
	}
	cmd()
	if len(svc.updated) != 1 || svc.updated[0].Priority == nil || *svc.updated[0].Priority != core.PriorityLow {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

// TestTheBoardFooterOffersTheCycle reads the footer, not the frame: the same
// words are in the help overlay, and a card's own title could hold them.
func TestTheBoardFooterOffersTheCycle(t *testing.T) {
	m, _ := priorityBoard(t, core.PriorityNormal)
	footer := footerLine(t, m.Frame())
	for _, want := range []string{m.keys.CyclePriority.Help().Key, "cycle priority"} {
		if !strings.Contains(footer, want) {
			t.Errorf("the board footer does not offer %q: %q", want, footer)
		}
	}
}

// TestThePriorityPickerStillPicksInOneTrip is the other half of the pair. The
// cycle is the fast path; naming a priority outright stays one trip rather than
// up to four presses.
func TestThePriorityPickerStillPicksInOneTrip(t *testing.T) {
	m, svc := priorityBoard(t, core.PriorityLowest)
	m, _ = m.reduce(pressKey("P"))
	if m.choice != choicePriority || len(m.choices) != 5 {
		t.Fatalf("choice = %v choices = %+v", m.choice, m.choices)
	}
	_, cmd := m.reduce(pressKey("2"))
	if cmd == nil {
		t.Fatal("the picker asked for nothing to be done")
	}
	cmd()
	if len(svc.updated) != 1 || svc.updated[0].Priority == nil || *svc.updated[0].Priority != core.PriorityHigh {
		t.Fatalf("updated = %+v", svc.updated)
	}
}

// TestTheTaskViewsAreCheckedForCollisionsOnTheirEditingKeys names the two
// views that dispatch these actions in the list Validate walks. An action a
// view dispatches but does not list is an action a rebinding can quietly
// shadow, which is how p came to mean two things in the first place.
func TestTheTaskViewsAreCheckedForCollisionsOnTheirEditingKeys(t *testing.T) {
	for _, v := range []viewKind{viewBoard, viewDetail} {
		listed := viewActions(v)
		for _, action := range []string{"Edit", "CyclePriority", "Priority"} {
			if !slices.Contains(listed, action) {
				t.Errorf("the %s view dispatches %s but does not list it, so a scheme could bind its key twice",
					viewName(v), action)
			}
		}
	}
	for _, scheme := range Schemes() {
		if _, err := KeyMapFrom(scheme, map[string]string{"Projects": "p"}); err == nil {
			t.Errorf("under %s, moving the project list onto p was accepted although p cycles a priority", scheme)
		}
	}
}

// TestAReaderWhoMayNotEditIsNotOfferedTheCycle asserts both halves, because
// either one alone passes while the other is broken: the key is absent from
// what the reader is shown, and pressing it anyway reaches no service.
func TestAReaderWhoMayNotEditIsNotOfferedTheCycle(t *testing.T) {
	reader := &core.Actor{ID: "p", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeTaskRead, core.ScopeProjectRead}}
	m := New(Config{Actor: reader, Access: capability.TUIAccess(reader)})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks:    []core.Task{task("a", "todo", 1, core.PriorityNormal)},
	})
	svc := newFakeService()
	m.svc = svc
	if m.mayPerform("UpdateTask") {
		t.Fatal("this reader may update tasks, so the guard is watching nothing")
	}

	for _, entry := range m.keys.ViewHelp(viewBoard, m.permits()) {
		if entry.Desc == m.keys.CyclePriority.Help().Desc {
			t.Errorf("the help overlay offers %q to a reader who may not update a task", entry.Desc)
		}
	}
	if strings.Contains(footerLine(t, m.Frame()), m.keys.CyclePriority.Help().Desc) {
		t.Errorf("the footer offers the cycle to a reader who may not update a task: %q",
			footerLine(t, m.Frame()))
	}
	if _, cmd := m.reduce(pressKey("p")); cmd != nil {
		cmd()
	}
	if len(svc.updated) != 0 {
		t.Errorf("p sent an update for a reader who may not make one: %+v", svc.updated)
	}
}

// TestTheEditKeyOpensTheWholeTaskForm is change one at the keyboard: e is the
// form, not a title-only prompt, and there is no longer a second key for the
// title alone.
func TestTheEditKeyOpensTheWholeTaskForm(t *testing.T) {
	m, _ := priorityBoard(t, core.PriorityNormal)
	svc := newFakeService()
	svc.directory = []core.Actor{{ID: "a-ada", Handle: "ada"}}
	m.svc = svc
	selected, _ := TaskAt(m.columns, m.sel)

	next, cmd := m.reduce(pressKey("e"))
	if next.prompt != promptNone {
		t.Fatalf("e opened a prompt rather than the form: %v", next.prompt)
	}
	next, _ = next.reduce(run(t, cmd))
	if !next.form.Open() {
		t.Fatalf("e opened no form: %q", next.err)
	}
	for _, field := range []string{"title", "body", "priority", "assignee"} {
		if formRows(t, next.Frame(), "edit "+selected.Ref, field) != 1 {
			t.Errorf("the form e opened draws no row for %q", field)
		}
	}
}

// TestNoKeyOpensATitleOnlyPrompt is the removal, asserted rather than assumed.
// The form holds the title, so a second way to change only the title is a
// slower way to do part of what e does.
func TestNoKeyOpensATitleOnlyPrompt(t *testing.T) {
	for _, spec := range promptSpecs {
		if spec.Title() == "title" {
			t.Fatalf("a title-only prompt is still registered: %+v", spec)
		}
	}
}
