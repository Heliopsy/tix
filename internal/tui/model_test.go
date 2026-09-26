// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/heliopsy/tix/internal/core"
)

// pressKey builds the key message a terminal would send for a binding.
func pressKey(s string) tea.KeyMsg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEscape}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

// boardModel returns a model already showing a three task board.
func boardModel(t *testing.T) Model {
	t.Helper()
	m := New(Config{Access: fullAccess(), Environ: []string{"NO_COLOR=1"}, Now: func() time.Time {
		return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	}})
	m.width, m.height = 120, 40
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
		tasks: []core.Task{
			task("a", "todo", 1, core.PriorityNormal),
			task("b", "todo", 2, core.PriorityNormal),
			task("c", "doing", 3, core.PriorityNormal),
		},
	})
	return m
}

func TestReduceWindowSize(t *testing.T) {
	m := New(Config{Access: fullAccess()})
	m, cmd := m.reduce(tea.WindowSizeMsg{Width: 200, Height: 50})
	if m.width != 200 || m.height != 50 {
		t.Fatalf("size = %dx%d", m.width, m.height)
	}
	if cmd != nil {
		t.Fatal("a resize asked for work")
	}

	unknown, _ := m.reduce(tea.WindowSizeMsg{})
	if unknown.width != 200 || unknown.height != 50 {
		t.Fatalf("an unreportable size overwrote the layout: %dx%d", unknown.width, unknown.height)
	}
}

func TestReduceProjects(t *testing.T) {
	t.Run("installs the listing", func(t *testing.T) {
		m := New(Config{Access: fullAccess()})
		m, _ = m.reduce(projectsMsg{projects: []core.Project{{Key: "infra"}, {Key: "web"}}})
		if len(m.projects) != 2 || m.view != viewProjects {
			t.Fatalf("projects = %+v view = %v", m.projects, m.view)
		}
	})
	t.Run("empty listing is not an error", func(t *testing.T) {
		m := New(Config{Access: fullAccess()})
		m, _ = m.reduce(projectsMsg{})
		if m.err != "" || m.fatal != nil {
			t.Fatalf("an empty listing produced err=%q fatal=%v", m.err, m.fatal)
		}
	})
	t.Run("opens the requested project", func(t *testing.T) {
		m := New(Config{Access: fullAccess(), Project: "web", Service: newFakeService()})
		m, cmd := m.reduce(projectsMsg{projects: []core.Project{{Key: "infra"}, {Key: "web"}}})
		if m.projectSel != 1 || cmd == nil {
			t.Fatalf("sel = %d cmd = %v", m.projectSel, cmd)
		}
		if m.openProject != "" {
			t.Fatal("the requested project was not consumed")
		}
	})
	t.Run("reports an unreachable project", func(t *testing.T) {
		m := New(Config{Access: fullAccess(), Project: "gone"})
		m, _ = m.reduce(projectsMsg{projects: []core.Project{{Key: "infra"}}})
		if !strings.Contains(m.err, "gone") {
			t.Fatalf("err = %q", m.err)
		}
	})
}

func TestReduceBoardOpensTheProject(t *testing.T) {
	m := boardModel(t)
	if m.view != viewBoard || m.project.Key != "infra" {
		t.Fatalf("view = %v project = %q", m.view, m.project.Key)
	}
	if len(m.columns) != 4 || len(m.columns[0].Tasks) != 2 {
		t.Fatalf("columns = %+v", m.columns)
	}
}

func TestReduceTasksPreservesTheSelectionAcrossALiveUpdate(t *testing.T) {
	m := boardModel(t)
	m.sel = Selection{Col: 0, Row: 1}
	selected, _ := TaskAt(m.columns, m.sel)
	if selected.ID != "b" {
		t.Fatalf("test set up the wrong selection: %q", selected.ID)
	}

	t.Run("task moves column", func(t *testing.T) {
		next, _ := m.reduce(tasksMsg{tasks: []core.Task{
			task("a", "todo", 1, core.PriorityNormal),
			task("b", "doing", 2, core.PriorityNormal),
			task("c", "doing", 3, core.PriorityNormal),
		}})
		got, ok := TaskAt(next.columns, next.sel)
		if !ok || got.ID != "b" {
			t.Fatalf("selection moved to %+v (%q)", next.sel, got.ID)
		}
	})

	t.Run("new task arrives above the selection", func(t *testing.T) {
		next, _ := m.reduce(tasksMsg{tasks: []core.Task{
			task("z", "todo", 0, core.PriorityHighest),
			task("a", "todo", 1, core.PriorityNormal),
			task("b", "todo", 2, core.PriorityNormal),
		}})
		got, _ := TaskAt(next.columns, next.sel)
		if got.ID != "b" {
			t.Fatalf("selection slipped to %q", got.ID)
		}
	})

	t.Run("selected task disappears", func(t *testing.T) {
		next, _ := m.reduce(tasksMsg{tasks: []core.Task{task("a", "todo", 1, core.PriorityNormal)}})
		if _, ok := TaskAt(next.columns, next.sel); !ok {
			t.Fatalf("selection %+v is off the board", next.sel)
		}
	})
}

func TestReduceEvent(t *testing.T) {
	t.Run("records the position and reloads", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		next, cmd := m.reduce(eventMsg{event: core.Event{Seq: 7, ProjectID: "p1", Type: core.EventTaskUpdated}})
		if next.lastSeq != 7 || !next.connected {
			t.Fatalf("seq = %d connected = %v", next.lastSeq, next.connected)
		}
		if cmd == nil {
			t.Fatal("an event for the open project did not refresh it")
		}
	})
	t.Run("ignores another project", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		if cmd := m.reloadFor(core.Event{ProjectID: "other", Type: core.EventTaskUpdated}); cmd != nil {
			t.Fatal("an unrelated project's event caused a reload")
		}
	})
	t.Run("never moves the position backwards", func(t *testing.T) {
		m := boardModel(t)
		m.lastSeq = 10
		next, _ := m.reduce(eventMsg{event: core.Event{Seq: 3, ProjectID: "p1"}})
		if next.lastSeq != 10 {
			t.Fatalf("seq went backwards to %d", next.lastSeq)
		}
	})
}

func TestReduceStream(t *testing.T) {
	t.Run("connected starts reading", func(t *testing.T) {
		m := New(Config{Access: fullAccess()})
		events := make(chan core.Event, 1)
		next, cmd := m.reduce(streamMsg{connected: true, events: events})
		if !next.connected || cmd == nil {
			t.Fatalf("connected = %v cmd = %v", next.connected, cmd)
		}
	})
	t.Run("a drop is reported and retried", func(t *testing.T) {
		m := New(Config{Access: fullAccess()})
		m.connected = true
		next, cmd := m.reduce(streamMsg{err: errors.New("connection reset")})
		if next.connected {
			t.Fatal("a dropped stream still reports connected")
		}
		if !strings.Contains(next.err, "connection reset") || cmd == nil {
			t.Fatalf("err = %q cmd = %v", next.err, cmd)
		}
	})
	t.Run("resubscribing resumes from the last sequence", func(t *testing.T) {
		svc := newFakeService()
		m := New(Config{Access: fullAccess(), Service: svc})
		m.lastSeq = 42
		next, _ := m.reduce(reconnectMsg{})
		if next.lastSeq != 42 {
			t.Fatalf("lost the stream position: %d", next.lastSeq)
		}
	})
}

func TestReduceAction(t *testing.T) {
	ref := core.TaskRef{ID: "a"}
	t.Run("claim stores the lease", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		next, cmd := m.reduce(actionMsg{kind: actionClaim, ref: ref, token: "tok"})
		if next.leases["a"] != "tok" || next.err != "" || cmd == nil {
			t.Fatalf("leases = %v err = %q", next.leases, next.err)
		}
	})
	t.Run("losing the claim race is explained and the board is reloaded", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		next, cmd := m.reduce(actionMsg{
			kind: actionClaim, ref: ref,
			err: core.Conflict("task %q is already claimed by %q", "infra-1", "worker-2"),
		})
		if _, held := next.leases["a"]; held {
			t.Fatal("a lost race still recorded a lease")
		}
		if !strings.Contains(next.err, "already claimed") || !strings.Contains(next.err, "worker-2") {
			t.Fatalf("err = %q", next.err)
		}
		if cmd == nil {
			t.Fatal("the board was not reloaded to show the current owner")
		}
	})
	t.Run("release drops the lease", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		m.leases["a"] = "tok"
		next, _ := m.reduce(actionMsg{kind: actionRelease, ref: ref})
		if _, held := next.leases["a"]; held {
			t.Fatal("the lease survived a release")
		}
		if !strings.Contains(next.status, "released") {
			t.Fatalf("status = %q", next.status)
		}
	})
	t.Run("an illegal transition is refused without changing anything", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		before := m.columns
		next, _ := m.reduce(actionMsg{
			kind: actionTransition, ref: ref,
			err: core.Precondition("transition from %q to %q is not permitted", "todo", "done"),
		})
		if !strings.Contains(next.err, "not permitted") {
			t.Fatalf("err = %q", next.err)
		}
		if len(next.columns) != len(before) || next.columns[0].Tasks[0].Status != "todo" {
			t.Fatal("a refused transition changed the board")
		}
	})
	t.Run("an expired lease is explained", func(t *testing.T) {
		m := boardModel(t)
		next, _ := m.reduce(actionMsg{kind: actionRelease, ref: ref, err: core.LeaseExpired("lease expired")})
		if !strings.Contains(next.err, "expired") {
			t.Fatalf("err = %q", next.err)
		}
	})
}

func TestReduceErrorsAreStateNotPanics(t *testing.T) {
	t.Run("a recoverable error keeps the interface running", func(t *testing.T) {
		m := boardModel(t)
		next, cmd := m.reduce(errMsg{err: core.Forbidden("token lacks task:write")})
		if !strings.Contains(next.err, "task:write") {
			t.Fatalf("err = %q", next.err)
		}
		if next.fatal != nil || cmd != nil {
			t.Fatal("a recoverable error ended the program")
		}
		if next.view != viewBoard || len(next.columns) != 4 {
			t.Fatal("a recoverable error lost the board")
		}
	})
	t.Run("a fatal error quits with the error recorded", func(t *testing.T) {
		m := boardModel(t)
		next, cmd := m.reduce(errMsg{err: core.Internal("database is gone"), fatal: true})
		if next.fatal == nil || cmd == nil {
			t.Fatalf("fatal = %v cmd = %v", next.fatal, cmd)
		}
	})
	t.Run("an unknown message changes nothing", func(t *testing.T) {
		m := boardModel(t)
		next, cmd := m.reduce(struct{ nonsense int }{1})
		if cmd != nil || next.err != "" || next.view != m.view {
			t.Fatal("an unknown message disturbed the model")
		}
	})
}

func TestHelpToggle(t *testing.T) {
	for _, view := range []viewKind{viewProjects, viewBoard, viewDetail} {
		m := boardModel(t)
		m.view = view
		m.sel = Selection{Col: 1}

		opened, _ := m.reduce(pressKey("?"))
		if opened.view != viewHelp || opened.underView() != view {
			t.Fatalf("help did not open from view %v: %+v", view, opened.view)
		}
		if len(opened.keys.ViewHelp(opened.underView())) == 0 || len(opened.keys.GlobalHelp(allViews)) == 0 {
			t.Fatal("help listed no bindings")
		}

		closed, _ := opened.reduce(pressKey("esc"))
		if closed.view != view || closed.sel.Col != 1 {
			t.Fatalf("dismissing help lost state: view %v sel %+v", closed.view, closed.sel)
		}
	}
}

func TestQuitAndInterruptAreDistinct(t *testing.T) {
	m := boardModel(t).rootView()

	quit, cmd := m.reduce(pressKey("q"))
	if cmd == nil || quit.interrupted {
		t.Fatal("q did not quit cleanly from the top level")
	}

	interrupted, cmd := m.reduce(pressKey("ctrl+c"))
	if cmd == nil || !interrupted.interrupted {
		t.Fatal("ctrl+c was not recorded as an interrupt")
	}
}

func TestInterruptEndsTheProgramFromAnyDepth(t *testing.T) {
	m := boardModel(t)
	task, _ := TaskAt(m.columns, m.sel)
	m, _ = m.reduce(detailMsg{task: task})
	interrupted, cmd := m.reduce(pressKey("ctrl+c"))
	if cmd == nil || !interrupted.interrupted {
		t.Fatal("ctrl+c was swallowed by the view stack")
	}
}

func TestUnknownKeyIsHarmless(t *testing.T) {
	m := boardModel(t)
	next, cmd := m.reduce(pressKey("Z"))
	if cmd != nil || next.err != "" || next.sel != m.sel || next.view != m.view {
		t.Fatalf("an unbound key changed the model: err=%q sel=%+v", next.err, next.sel)
	}
}

func TestBoardNavigationKeys(t *testing.T) {
	tests := []struct {
		name    string
		keys    []string
		wantCol int
		wantRow int
	}{
		{"down", []string{"j"}, 0, 1},
		{"down then up", []string{"j", "k"}, 0, 0},
		{"right", []string{"l"}, 1, 0},
		{"right then left", []string{"l", "h"}, 0, 0},
		{"arrow keys", []string{"down", "right"}, 1, 0},
		{"last row", []string{"G"}, 0, 1},
		{"first row", []string{"j", "g"}, 0, 0},
		{"clamped at the far column", []string{"l", "l", "l", "l", "l"}, 3, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := boardModel(t)
			for _, k := range tc.keys {
				m, _ = m.reduce(pressKey(k))
			}
			if m.sel.Col != tc.wantCol || m.sel.Row != tc.wantRow {
				t.Fatalf("selection = %+v, want {%d %d}", m.sel, tc.wantCol, tc.wantRow)
			}
		})
	}
}

func TestFilterBar(t *testing.T) {
	t.Run("slash opens it with the current expression", func(t *testing.T) {
		m := boardModel(t)
		m.filterText = "status:todo"
		next, _ := m.reduce(pressKey("/"))
		if next.prompt != promptFilter || next.input.Value() != "status:todo" {
			t.Fatalf("prompt = %v value = %q", next.prompt, next.input.Value())
		}
	})
	t.Run("escape abandons the edit", func(t *testing.T) {
		m := boardModel(t)
		m, _ = m.reduce(pressKey("/"))
		m, _ = m.reduce(pressKey("esc"))
		if m.prompt != promptNone || m.filterText != "" {
			t.Fatalf("prompt = %v text = %q", m.prompt, m.filterText)
		}
	})
	t.Run("a valid expression filters every column", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		m, _ = m.reduce(pressKey("/"))
		m.input.SetValue("status:doing")
		m, cmd := m.reduce(pressKey("enter"))
		if m.prompt != promptNone || m.filterErr != "" || cmd == nil {
			t.Fatalf("prompt = %v filterErr = %q", m.prompt, m.filterErr)
		}
		if len(m.columns[0].Tasks) != 0 || len(m.columns[1].Tasks) != 1 {
			t.Fatalf("filter was not applied to the columns: %+v", m.columns)
		}
	})
	t.Run("a malformed expression is explained and the results stand", func(t *testing.T) {
		m := boardModel(t)
		before := len(m.columns[0].Tasks)
		m, _ = m.reduce(pressKey("/"))
		m.input.SetValue("colour:red")
		m, cmd := m.reduce(pressKey("enter"))
		if m.filterErr == "" || !strings.Contains(m.filterErr, "colour") {
			t.Fatalf("filterErr = %q", m.filterErr)
		}
		if m.prompt != promptFilter || cmd != nil {
			t.Fatal("a malformed expression left the filter bar")
		}
		if len(m.columns[0].Tasks) != before {
			t.Fatal("a malformed expression discarded the previous results")
		}
	})
	t.Run("clearing restores every task", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		m = m.applyFilterText("status:doing")
		m, _ = m.reduce(pressKey("C"))
		if m.filterText != "" || len(m.columns[0].Tasks) != 2 {
			t.Fatalf("filter text = %q columns = %+v", m.filterText, m.columns)
		}
	})
	t.Run("typing reaches the input rather than the board", func(t *testing.T) {
		m := boardModel(t)
		m, _ = m.reduce(pressKey("/"))
		m, _ = m.reduce(pressKey("j"))
		if m.sel.Row != 0 {
			t.Fatal("a keystroke meant for the filter moved the board")
		}
		if m.input.Value() != "j" {
			t.Fatalf("input = %q", m.input.Value())
		}
	})
}

func TestTransitionChooser(t *testing.T) {
	t.Run("offers what the workflow permits", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		next, _ := m.reduce(pressKey("t"))
		if next.choice != choiceTransition || len(next.choices) != 1 || next.choices[0].Value != "doing" {
			t.Fatalf("choices = %+v", next.choices)
		}
	})
	t.Run("a number picks a target", func(t *testing.T) {
		m := boardModel(t)
		svc := newFakeService()
		m.svc = svc
		m, _ = m.reduce(pressKey("t"))
		m, cmd := m.reduce(pressKey("1"))
		if m.choice != choiceNone || cmd == nil {
			t.Fatalf("choice = %v cmd = %v", m.choice, cmd)
		}
		cmd()
		if len(svc.transitions) != 1 || svc.transitions[0].To != "doing" {
			t.Fatalf("transitions = %+v", svc.transitions)
		}
	})
	t.Run("an out of range number is ignored", func(t *testing.T) {
		m := boardModel(t)
		m.svc = newFakeService()
		m, _ = m.reduce(pressKey("t"))
		next, cmd := m.reduce(pressKey("9"))
		if next.choice != choiceTransition || cmd != nil {
			t.Fatal("an out of range choice acted")
		}
	})
	t.Run("escape cancels", func(t *testing.T) {
		m := boardModel(t)
		m, _ = m.reduce(pressKey("t"))
		m, _ = m.reduce(pressKey("esc"))
		if m.choice != choiceNone || m.choices != nil {
			t.Fatal("the chooser survived a cancel")
		}
	})
	t.Run("a terminal state says so instead of offering nothing", func(t *testing.T) {
		m := boardModel(t)
		m.sel = Selection{Col: 3}
		m, _ = m.reduce(tasksMsg{tasks: []core.Task{task("d", "done", 4, core.PriorityNormal)}})
		m.sel, _ = FindTask(m.columns, "d")
		next, _ := m.reduce(pressKey("t"))
		if next.choice != choiceNone || !strings.Contains(next.err, "no transition") {
			t.Fatalf("choice = %v err = %q", next.choice, next.err)
		}
	})
}

func TestClaimAndReleaseFromTheBoard(t *testing.T) {
	t.Run("claim asks the service for a lease", func(t *testing.T) {
		m := boardModel(t)
		svc := newFakeService()
		m.svc = svc
		_, cmd := m.reduce(pressKey("c"))
		if cmd == nil {
			t.Fatal("claim issued no work")
		}
		msg, ok := cmd().(actionMsg)
		if !ok || msg.kind != actionClaim || msg.token == "" {
			t.Fatalf("msg = %+v", msg)
		}
		if len(svc.claimed) != 1 || svc.claimed[0].ID != "a" {
			t.Fatalf("claimed = %+v", svc.claimed)
		}
	})
	t.Run("a lost race comes back as a conflict", func(t *testing.T) {
		m := boardModel(t)
		svc := newFakeService()
		svc.claimErr = core.Conflict("task is already claimed")
		m.svc = svc
		_, cmd := m.reduce(pressKey("c"))
		msg := cmd().(actionMsg)
		next, _ := m.reduce(msg)
		if core.KindOf(msg.err) != core.KindConflict || !strings.Contains(next.err, "already claimed") {
			t.Fatalf("msg = %+v err = %q", msg, next.err)
		}
	})
	t.Run("release without a lease is refused locally", func(t *testing.T) {
		m := boardModel(t)
		svc := newFakeService()
		m.svc = svc
		next, cmd := m.reduce(pressKey("x"))
		if cmd != nil || len(svc.released) != 0 {
			t.Fatal("released a task this session does not hold")
		}
		if !strings.Contains(next.err, "does not hold") {
			t.Fatalf("err = %q", next.err)
		}
	})
	t.Run("release of a held task reaches the service", func(t *testing.T) {
		m := boardModel(t)
		svc := newFakeService()
		m.svc = svc
		m.leases["a"] = "tok"
		_, cmd := m.reduce(pressKey("x"))
		if cmd == nil {
			t.Fatal("release issued no work")
		}
		msg := cmd().(actionMsg)
		if msg.kind != actionRelease || msg.err != nil || len(svc.released) != 1 {
			t.Fatalf("msg = %+v released = %+v", msg, svc.released)
		}
	})
}

func TestDetailViewRoundTrip(t *testing.T) {
	m := boardModel(t)
	m.sel = Selection{Col: 0, Row: 1}
	m, _ = m.reduce(detailMsg{task: task("b", "todo", 2, core.PriorityNormal)})
	if m.view != viewDetail || m.detail == nil {
		t.Fatalf("view = %v detail = %v", m.view, m.detail)
	}
	back, _ := m.reduce(pressKey("esc"))
	if back.view != viewBoard || back.sel.Row != 1 {
		t.Fatalf("returning to the board lost the selection: view %v sel %+v", back.view, back.sel)
	}
}

func TestProjectPickerKeysOpenABoard(t *testing.T) {
	svc := newFakeService()
	svc.projects = []core.Project{{ID: "p1", Key: "infra"}, {ID: "p2", Key: "web"}}
	m := New(Config{Access: fullAccess(), Service: svc})
	m, _ = m.reduce(projectsMsg{projects: svc.projects})

	m, _ = m.reduce(pressKey("j"))
	if m.projectSel != 1 {
		t.Fatalf("projectSel = %d", m.projectSel)
	}
	m, _ = m.reduce(pressKey("k"))
	m, _ = m.reduce(pressKey("G"))
	if m.projectSel != 1 {
		t.Fatalf("G did not reach the last project: %d", m.projectSel)
	}
	_, cmd := m.reduce(pressKey("enter"))
	if cmd == nil {
		t.Fatal("enter did not open a board")
	}
}

func TestRefreshLoadsWhateverIsOpen(t *testing.T) {
	svc := newFakeService()
	m := New(Config{Access: fullAccess(), Service: svc})
	if m.refresh() == nil {
		t.Fatal("refresh did nothing without a project")
	}
	board := boardModel(t)
	board.svc = svc
	if board.refresh() == nil {
		t.Fatal("refresh did nothing with a board open")
	}
}

func TestListErrorSurfacesAsState(t *testing.T) {
	svc := newFakeService()
	svc.listErr = core.Forbidden("token lacks task:read")
	m := boardModel(t)
	m.svc = svc
	msg := m.loadTasks()()
	failure, ok := msg.(errMsg)
	if !ok || failure.fatal {
		t.Fatalf("msg = %#v", msg)
	}
	next, _ := m.reduce(failure)
	if !strings.Contains(next.err, "task:read") || next.view != viewBoard {
		t.Fatalf("err = %q view = %v", next.err, next.view)
	}
}
