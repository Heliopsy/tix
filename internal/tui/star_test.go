// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// starredBoard is the three task board with task b starred by the reader.
func starredBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	tasks := slices.Clone(m.tasks)
	for i := range tasks {
		tasks[i].Starred = tasks[i].ID == "b"
	}
	m, _ = m.installTasks(tasks)
	return m, svc
}

func TestAStarredCardLeadsItsColumnAndIsMarked(t *testing.T) {
	m, _ := starredBoard(t)
	todo := m.columns[0].Tasks
	if len(todo) != 2 || todo[0].ID != "b" || todo[1].ID != "a" {
		t.Fatalf("todo column = %v, want the starred b ahead of a", refsOf(todo))
	}
	frame := m.Frame()
	if line := cardLineFor(t, frame, "infra-b"); !strings.Contains(line, StarMarker+"infra-b") {
		t.Errorf("the starred card carries no star: %q", line)
	}
	for _, ref := range []string{"infra-a", "infra-c"} {
		if line := cardLineFor(t, frame, ref); strings.Contains(line, StarMarker+ref) {
			t.Errorf("an unstarred card carries a star: %q", line)
		}
	}
}

func TestTheStarKeyToggles(t *testing.T) {
	cases := []struct {
		name     string
		selected string
		star     bool
	}{
		{"an unstarred task is starred", "a", true},
		{"a starred task is unstarred", "b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := starredBoard(t)
			m.sel = PreserveSelection(m.columns, m.sel, tc.selected)
			if got, _ := TaskAt(m.columns, m.sel); got.ID != tc.selected {
				t.Fatalf("selected %q, want %q", got.ID, tc.selected)
			}
			next, cmd := m.reduce(pressKey("*"))
			msg := run(t, cmd)
			want := []core.TaskRef{{ID: tc.selected}}
			calls, other := svc.starred, svc.unstars
			if !tc.star {
				calls, other = svc.unstars, svc.starred
			}
			if !slices.Equal(calls, want) || len(other) != 0 {
				t.Fatalf("starred %v, unstarred %v, want one call for %q", svc.starred, svc.unstars, tc.selected)
			}
			after, reload := next.reduce(msg)
			if after.err != "" || reload == nil {
				t.Fatalf("the toggle did not reload the board: err %q", after.err)
			}
		})
	}
}

func TestTheStarKeyIsNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := starredBoard(t)
	m.actor = &core.Actor{ID: "u-none", TenantID: "t", Kind: core.ActorAgent, Scopes: []core.Scope{core.ScopeProjectRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("StarTask") {
		t.Fatal("the reader may star, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "*" }) {
		t.Errorf("a reader who may not star is told about *: %+v", help)
	}
	next, cmd := m.reduce(pressKey("*"))
	if cmd != nil {
		next.reduce(run(t, cmd))
	}
	if len(svc.starred)+len(svc.unstars) != 0 {
		t.Fatalf("a reader who may not star called the service: %v %v", svc.starred, svc.unstars)
	}
}

func TestTheStarKeyIsOfferedInHelpAndPalette(t *testing.T) {
	m, _ := starredBoard(t)
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if !slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "*" && e.Desc == "star" }) {
		t.Errorf("help does not list the star key: %+v", help)
	}
	m, _ = m.reduce(pressKey(":"))
	if !slices.ContainsFunc(m.paletteEntries(), func(e PaletteEntry) bool { return strings.Contains(e.Name, "star") }) {
		t.Errorf("the palette does not offer star: %+v", m.paletteEntries())
	}
}

func TestIsStarredNarrowsTheBoard(t *testing.T) {
	m, _ := starredBoard(t)
	m = m.applyFilterText("is:starred")
	if m.filterErr != "" {
		t.Fatalf("the board refused is:starred: %s", m.filterErr)
	}
	var shown []string
	for _, c := range m.columns {
		shown = append(shown, refsOf(c.Tasks)...)
	}
	if !slices.Equal(shown, []string{"infra-b"}) {
		t.Fatalf("is:starred shows %v, want only infra-b", shown)
	}
}

func refsOf(tasks []core.Task) []string {
	out := make([]string, len(tasks))
	for i, task := range tasks {
		out[i] = task.Ref
	}
	return out
}
