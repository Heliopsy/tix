package tui

import (
	"fmt"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestScrollWindow(t *testing.T) {
	tests := []struct {
		name                            string
		offset, selected, height, total int
		want                            int
	}{
		{"everything fits", 0, 2, 10, 4, 0},
		{"scrolls down to the selection", 0, 7, 5, 12, 3},
		{"scrolls up to the selection", 6, 2, 5, 12, 2},
		{"stays put while the selection is visible", 3, 5, 5, 12, 3},
		{"never runs past the end", 9, 11, 5, 12, 7},
		{"a list that shrank pulls the window back", 8, 0, 5, 6, 0},
		{"no room", 3, 9, 0, 12, 0},
		{"nothing to show", 3, 0, 5, 0, 0},
		{"a selection past the end is clamped", 0, 99, 5, 12, 7},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScrollWindow(tc.offset, tc.selected, tc.height, tc.total)
			if got != tc.want {
				t.Fatalf("ScrollWindow = %d, want %d", got, tc.want)
			}
			if tc.height > 0 && tc.total > 0 {
				sel := clamp(tc.selected, 0, tc.total-1)
				if sel < got || sel >= got+tc.height {
					t.Fatalf("selection %d fell outside the window %d..%d", sel, got, got+tc.height)
				}
			}
		})
	}
}

func TestScrollHintSaysWhatIsHidden(t *testing.T) {
	tests := []struct {
		name                   string
		offset, height, total  int
		wantAbove, wantBelow   bool
		wantEmpty              bool
		containsAbove, contain string
	}{
		{name: "everything visible", offset: 0, height: 10, total: 4, wantEmpty: true},
		{name: "more below", offset: 0, height: 5, total: 12, wantBelow: true},
		{name: "more above", offset: 7, height: 5, total: 12, wantAbove: true},
		{name: "more both ways", offset: 3, height: 5, total: 12, wantAbove: true, wantBelow: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ScrollHint(tc.offset, tc.height, tc.total)
			if tc.wantEmpty {
				if got != "" {
					t.Fatalf("ScrollHint = %q, want nothing", got)
				}
				return
			}
			if strings.Contains(got, "↑") != tc.wantAbove {
				t.Fatalf("ScrollHint = %q, above = %v", got, tc.wantAbove)
			}
			if strings.Contains(got, "↓") != tc.wantBelow {
				t.Fatalf("ScrollHint = %q, below = %v", got, tc.wantBelow)
			}
		})
	}
	if MoreAbove(0) {
		t.Fatal("a window at the top reported rows above it")
	}
	if MoreBelow(0, 0, 10) {
		t.Fatal("a window with no height reported rows below it")
	}
}

// projectListModel returns a model showing n projects at one terminal size.
func projectListModel(t *testing.T, n, width, height int) Model {
	t.Helper()
	projects := make([]core.Project, 0, n)
	for i := range n {
		projects = append(projects, core.Project{
			ID:    fmt.Sprintf("p%d", i),
			Key:   fmt.Sprintf("proj%d", i),
			Name:  fmt.Sprintf("Project Number %d", i),
			Color: core.ProjectColors()[i%len(core.ProjectColors())],
		})
	}
	m := New(Config{Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = width, height
	m, _ = m.reduce(projectsMsg{projects: projects})
	return m
}

// TestEveryProjectIsReachableAtASmallHeight walks the list one key at a time
// and demands that every project is drawn at some point. A test at the default
// size passes whether or not the list scrolls, because every project fits.
func TestEveryProjectIsReachableAtASmallHeight(t *testing.T) {
	for _, height := range []int{9, 10, 14, 20, 30} {
		t.Run(fmt.Sprintf("height %d", height), func(t *testing.T) {
			const total = 13
			m := projectListModel(t, total, 130, height)
			seen := map[string]bool{}
			for step := 0; step <= total; step++ {
				for _, line := range m.projectLines() {
					for i := range total {
						if strings.Contains(line, fmt.Sprintf("proj%d ", i)) {
							seen[fmt.Sprintf("proj%d", i)] = true
						}
					}
				}
				m, _ = m.reduce(pressKey("j"))
			}
			for i := range total {
				if key := fmt.Sprintf("proj%d", i); !seen[key] {
					t.Fatalf("%s is never drawn at height %d, so it cannot be opened", key, height)
				}
			}
		})
	}
}

func TestTheProjectListSaysWhenItRunsOnPastTheScreen(t *testing.T) {
	m := projectListModel(t, 13, 130, 14)
	lines := m.projectLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "more") {
		t.Fatalf("a list that does not fit gave no sign of it:\n%s", joined)
	}
	short := projectListModel(t, 3, 130, 30)
	if strings.Contains(strings.Join(short.projectLines(), "\n"), "more") {
		t.Fatal("a list that fits claimed there was more to see")
	}
}

func TestTheProjectListNeverDrawsMoreRowsThanItHas(t *testing.T) {
	for _, height := range []int{9, 12, 14, 24, 40} {
		m := projectListModel(t, 13, 130, height)
		body := LayoutFor(m.width, m.height, 0).BodyHeight
		if got := len(m.projectLines()); got > body {
			t.Fatalf("height %d: drew %d rows into %d lines of body", height, got, body)
		}
	}
}

func TestTheSelectionStaysOnScreenAsItMoves(t *testing.T) {
	m := projectListModel(t, 13, 130, 14)
	for step := range 13 {
		m, _ = m.reduce(pressKey("j"))
		want := fmt.Sprintf("proj%d ", min(step+1, 12))
		found := false
		for _, line := range m.projectLines() {
			if strings.Contains(line, SelectionMarker(true)) && strings.Contains(line, want) {
				found = true
			}
		}
		if !found {
			t.Fatalf("after %d moves the selected project %q is off screen", step+1, want)
		}
	}
}

func TestABoardColumnScrollsWithItsSelection(t *testing.T) {
	tasks := make([]core.Task, 0, 40)
	for i := range 40 {
		tasks = append(tasks, task(fmt.Sprintf("t%d", i), "todo", int64(i+1), core.PriorityNormal))
	}
	m := New(Config{Environ: []string{"NO_COLOR=1"}})
	m.width, m.height = 130, 14
	m, _ = m.reduce(boardMsg{
		project:  core.Project{ID: "p1", Key: "infra"},
		workflow: testWorkflow(),
		tasks:    tasks,
	})
	layout := LayoutFor(m.width, m.height, len(m.columns))
	seen := map[string]bool{}
	for range len(tasks) + 1 {
		for _, line := range m.boardLines(layout) {
			for i := range len(tasks) {
				if strings.Contains(line, fmt.Sprintf("infra-t%d ", i)) {
					seen[fmt.Sprintf("infra-t%d", i)] = true
				}
			}
		}
		m, _ = m.reduce(pressKey("j"))
	}
	for i := range len(tasks) {
		if ref := fmt.Sprintf("infra-t%d", i); !seen[ref] {
			t.Fatalf("%s is never drawn, so a column does not scroll", ref)
		}
	}
}
