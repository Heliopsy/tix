// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// menuModel is a hosted SSH session that named nothing, with two projects.
func menuModel(t *testing.T) Model {
	t.Helper()
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Environ: []string{"NO_COLOR=1"}, Start: true,
		Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	m.width, m.height = 120, 40
	m, _ = m.reduce(projectsMsg{projects: []core.Project{
		{ID: "p1", Key: "ops", Name: "Operations"},
		{ID: "p2", Key: "web", Name: "Website"},
	}})
	return m
}

// menuRow is the one body line naming title, so an assertion about a row
// cannot be satisfied by the footer or the title bar.
func menuRow(t *testing.T, m Model, title string) string {
	t.Helper()
	for _, line := range m.menuLines() {
		if strings.Contains(line, " "+title+" ") {
			return line
		}
	}
	return ""
}

// TestTheStartMenuHasARowForEveryInterface is the registry's menu half: the
// SSH listener's half is in internal/sshd.
func TestTheStartMenuHasARowForEveryInterface(t *testing.T) {
	m := menuModel(t)
	if m.view != viewMenu {
		t.Fatalf("a session started with Start lands on %q, want the start menu", m.viewName())
	}
	for _, i := range Interfaces() {
		if menuRow(t, m, i.Title) == "" {
			t.Errorf("the start menu has no row for the %q interface", i.Name)
		}
		if i.Name == "" || i.Usage == "" || !strings.HasPrefix(i.Usage, i.Name) {
			t.Errorf("interface %+v has no command usage starting with its name", i)
		}
		if _, ok := LookupInterface(strings.ToUpper(i.Name)); !ok {
			t.Errorf("LookupInterface does not find %q in another case", i.Name)
		}
	}
}

func TestALocalSessionStillStartsOnTheProjectList(t *testing.T) {
	m := New(Config{Access: fullAccess()})
	if m.view != viewProjects {
		t.Fatalf("tix tui lands on %q, want projects", m.viewName())
	}
}

// TestWatchFromTheMenuNarrowsToThePickedProject asserts both halves: the line
// from the picked project is drawn and the other project's line is not.
func TestWatchFromTheMenuNarrowsToThePickedProject(t *testing.T) {
	m := menuModel(t)
	ops, web := taskEvent(1, "alice"), taskEvent(2, "bob")
	ops.ProjectID, web.ProjectID = "p1", "p2"
	m, _ = m.reduce(eventMsg{event: ops})
	m, _ = m.reduce(eventMsg{event: web})

	m, _ = m.reduce(pressKey("down"))
	m, _ = m.reduce(pressKey("right"))
	if row := menuRow(t, m, "Watch"); !strings.Contains(row, "[OPS]") {
		t.Fatalf("the Watch row reads %q after one step right, want it set to OPS", row)
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.view != viewActivity {
		t.Fatalf("enter on Watch opened %q, want activity", m.viewName())
	}
	shown := m.shownActivity()
	if len(shown) != 1 || shown[0].ProjectID != "p1" {
		t.Fatalf("watching OPS shows %+v, want only the OPS event", shown)
	}

	m, _ = m.reduce(pressKey("esc"))
	if m.view != viewMenu {
		t.Fatalf("back from Watch lands on %q, want the start menu", m.viewName())
	}
	m, _ = m.reduce(pressKey("v"))
	if got := len(m.shownActivity()); got != 2 {
		t.Fatalf("v shows %d events after watching OPS from the menu, want every project's 2", got)
	}
}

func TestTildeOpensTheStartMenuFromABoard(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("~"))
	if m.view != viewMenu {
		t.Fatalf("~ on the board opened %q, want the start menu", m.viewName())
	}
	m, _ = m.reduce(pressKey("enter"))
	if m.view != viewProjects {
		t.Fatalf("enter on Boards opened %q, want projects", m.viewName())
	}
}

func TestPausingTheTailHoldsEventsUntilResumed(t *testing.T) {
	m := boardModel(t)
	m, _ = m.reduce(pressKey("v"))
	m, _ = m.reduce(eventMsg{event: taskEvent(1, "alice")})
	space := tea.KeyPressMsg{Code: tea.KeySpace}

	m, _ = m.reduce(space)
	m, _ = m.reduce(eventMsg{event: taskEvent(2, "bob")})
	m, _ = m.reduce(eventMsg{event: taskEvent(3, "carol")})
	if got := len(m.shownActivity()); got != 1 {
		t.Fatalf("a paused tail draws %d events, want the 1 it had", got)
	}
	if title := m.titleBar(); !strings.Contains(title, "paused, 2 new") {
		t.Fatalf("the title bar reads %q, want it to say two events are held", title)
	}

	m, _ = m.reduce(space)
	shown := m.shownActivity()
	if len(shown) != 3 || m.activitySel != 2 {
		t.Fatalf("resuming drew %d events with the selection on %d, want 3 following the newest",
			len(shown), m.activitySel)
	}
}
