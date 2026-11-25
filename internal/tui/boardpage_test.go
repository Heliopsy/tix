// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/heliopsy/tix/internal/core"
)

// pagedBoard is a board over a project holding one task more than a read.
func pagedBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	fake := newFakeService()
	fake.workflows = []core.Workflow{{ID: "w1", Key: "default", Definition: *testWorkflow()}}
	for i := range core.MaxPageLimit + 1 {
		fake.tasks = append(fake.tasks, task(fmt.Sprint("t", i), "todo", int64(i+1), core.PriorityNormal))
	}
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Service: fake, Environ: []string{"NO_COLOR=1"},
		Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	m.width, m.height = 160, 40
	m = runCmd(t, m, m.loadBoard(core.Project{ID: "p1", Key: "infra", WorkflowID: "w1"}))
	if m.err != "" {
		t.Fatalf("opening the board: %s", m.err)
	}
	return m, fake
}

func TestTheBoardSaysWhichPageOfALongProjectItHolds(t *testing.T) {
	m, _ := pagedBoard(t)
	if bar := stripANSI(m.statusBar()); !strings.Contains(bar, "page 1/2") {
		t.Errorf("a project of %d tasks says %q, want page 1/2", core.MaxPageLimit+1, bar)
	}
	if countTasks(m.columns) != core.MaxPageLimit {
		t.Errorf("the first read holds %d tasks, want %d", countTasks(m.columns), core.MaxPageLimit)
	}
}

func TestPageDownReadsTheNextPageByNumber(t *testing.T) {
	m, fake := pagedBoard(t)
	next, cmd := m.reduce(tea.KeyPressMsg{Code: tea.KeyPgDown})
	m = runCmd(t, next, cmd)
	if got := fake.listed[len(fake.listed)-1].PageNumber; got != 2 {
		t.Errorf("pgdown asked for page %d, want 2", got)
	}
	if bar := stripANSI(m.statusBar()); !strings.Contains(bar, "page 2/2") {
		t.Errorf("after pgdown the status says %q, want page 2/2", bar)
	}
	if countTasks(m.columns) != 1 {
		t.Errorf("page 2 holds %d tasks, want 1", countTasks(m.columns))
	}
	if _, cmd := m.reduce(tea.KeyPressMsg{Code: tea.KeyPgDown}); cmd != nil {
		t.Error("pgdown on the last page asked for another read")
	}
	next, cmd = m.reduce(tea.KeyPressMsg{Code: tea.KeyPgUp})
	m = runCmd(t, next, cmd)
	if bar := stripANSI(m.statusBar()); !strings.Contains(bar, "page 1/2") {
		t.Errorf("after pgup the status says %q, want page 1/2", bar)
	}
}

func TestAShortProjectCarriesNoPageIndicator(t *testing.T) {
	m := boardModel(t)
	if bar := stripANSI(m.statusBar()); strings.Contains(bar, "page ") {
		t.Errorf("a three task board says %q", bar)
	}
}
