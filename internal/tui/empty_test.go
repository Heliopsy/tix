// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestBoardEmptyState(t *testing.T) {
	populated := []Column{{Key: "todo", Tasks: []core.Task{{ID: "a"}}}, {Key: "done"}}
	bare := []Column{{Key: "todo"}, {Key: "doing"}, {Key: "done"}}

	tests := []struct {
		name       string
		projectKey string
		columns    []Column
		filtered   bool
		wantEmpty  bool
		saysAbout  string
	}{
		{"a board with tasks says nothing", "infra", populated, false, false, ""},
		{"a board with tasks and a filter says nothing", "infra", populated, true, false, ""},
		{"an empty board names the way to fill it", "infra", bare, false, true, "infra"},
		{"a filtered empty board blames the filter", "infra", bare, true, true, "filter"},
		{"a project with no workflow says so", "infra", nil, false, true, "workflow"},
		{"an empty board with no project key still helps", "", bare, false, true, "n to add"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BoardEmptyState(tc.projectKey, tc.columns, tc.filtered)
			if got.Zero() == tc.wantEmpty {
				t.Fatalf("BoardEmptyState = %+v, wantEmpty = %v", got, tc.wantEmpty)
			}
			if !tc.wantEmpty {
				return
			}
			if got.Title == "" || got.Hint == "" {
				t.Fatalf("an empty state that explains nothing: %+v", got)
			}
			if !strings.Contains(got.Title+" "+got.Hint, tc.saysAbout) {
				t.Fatalf("%+v does not mention %q", got, tc.saysAbout)
			}
		})
	}
}

func TestProjectsEmptyState(t *testing.T) {
	got := ProjectsEmptyState(0)
	if got.Zero() {
		t.Fatal("no projects at all produced no empty state")
	}
	if !strings.Contains(got.Hint, "tix project create") {
		t.Fatalf("the empty state does not say how to create one: %+v", got)
	}
	if !ProjectsEmptyState(1).Zero() {
		t.Fatal("a list with a project in it claimed to be empty")
	}
}

func TestEmptyStateRendersAsReadableLines(t *testing.T) {
	lines := EmptyState{Title: "This board is empty.", Hint: "Press n."}.Lines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "This board is empty.") || !strings.Contains(joined, "Press n.") {
		t.Fatalf("lines = %q", joined)
	}
	if got := (EmptyState{}).Lines(); got != nil {
		t.Fatalf("a zero state rendered %v", got)
	}
	if got := (EmptyState{Title: "alone"}).Lines(); strings.Count(strings.Join(got, "\n"), "alone") != 1 {
		t.Fatalf("a hintless state rendered %v", got)
	}
}

// TestAnEmptyBoardSaysSoOnScreen is the defect this replaced: five columns
// reading "(0)" over blank space is indistinguishable from a failure.
func TestAnEmptyBoardSaysSoOnScreen(t *testing.T) {
	m := New(Config{Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 130, 24
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra", Name: "Infrastructure"},
		workflow: testWorkflow(),
	})
	frame := m.View()
	if !strings.Contains(frame, "This board is empty.") {
		t.Fatalf("an empty board drew no empty state:\n%s", frame)
	}
	if !strings.Contains(frame, "infra") {
		t.Fatalf("the empty state does not name the project:\n%s", frame)
	}
}

func TestAnEmptyProjectListSaysSoOnScreen(t *testing.T) {
	m := New(Config{Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 130, 24
	m, _ = m.reduce(projectsMsg{})
	frame := m.View()
	if !strings.Contains(frame, "No projects yet.") {
		t.Fatalf("an empty project list drew no empty state:\n%s", frame)
	}
}

func TestAFilteredOutBoardBlamesTheFilterRatherThanLookingBroken(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m = m.applyFilterText("status:nothing-matches-this")
	frame := m.View()
	if !strings.Contains(frame, "No task matches the filter.") {
		t.Fatalf("a filtered-out board gave no reason:\n%s", frame)
	}
	if !strings.Contains(frame, "clear the filter") {
		t.Fatalf("the empty state does not say how to recover:\n%s", frame)
	}
}
