// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func columnKeys(cols []Column) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Key
	}
	return out
}

func TestArrangeColumns(t *testing.T) {
	tasks := []core.Task{task("a", "todo", 1, core.PriorityNormal), task("c", "doing", 3, core.PriorityNormal),
		task("s", "shipped", 4, core.PriorityNormal)}
	cases := []struct {
		name   string
		prefs  []core.GroupPref
		want   []string
		hidden []string
	}{
		{"nothing arranged keeps workflow order", nil, []string{"todo", "doing", "review", "done", "shipped"}, nil},
		{"hidden is left out and named", []core.GroupPref{{Group: "review", Hidden: true}},
			[]string{"todo", "doing", "done", "shipped"}, []string{"review"}},
		{"starred leads", []core.GroupPref{{Group: "done", Starred: true}},
			[]string{"done", "todo", "doing", "review", "shipped"}, nil},
		{"positioned follow the starred, in position", []core.GroupPref{
			{Group: "done", Starred: true}, {Group: "review", Position: 2}, {Group: "doing", Position: 1}},
			[]string{"done", "doing", "review", "todo", "shipped"}, nil},
		{"a stray status is never arranged", []core.GroupPref{{Group: "shipped", Hidden: true}},
			[]string{"todo", "doing", "review", "done", "shipped"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, hidden := ArrangeColumns(BuildColumns(testWorkflow(), tasks), tc.prefs)
			if !slices.Equal(columnKeys(got), tc.want) || !slices.Equal(hidden, tc.hidden) {
				t.Fatalf("drawn %v hidden %v, want %v hidden %v", columnKeys(got), hidden, tc.want, tc.hidden)
			}
		})
	}
}

func TestACollapsedColumnKeepsItsCountAndNoCards(t *testing.T) {
	cols, _ := ArrangeColumns(BuildColumns(testWorkflow(), []core.Task{
		task("a", "todo", 1, core.PriorityNormal), task("b", "todo", 2, core.PriorityNormal),
	}), []core.GroupPref{{Group: "todo", Collapsed: true}})
	todo := cols[0]
	if !todo.Collapsed || len(todo.Tasks) != 0 || todo.Folded != 2 {
		t.Fatalf("collapsed todo = %+v", todo)
	}
	if got := ColumnHeadingText(todo); got != CollapsedMark+" To do (2)" {
		t.Fatalf("heading = %q", got)
	}
}

func TestColumnNeighbourStaysOnItsSideOfTheStars(t *testing.T) {
	cols, _ := ArrangeColumns(BuildColumns(testWorkflow(), nil), []core.GroupPref{{Group: "review", Starred: true}})
	// review, todo, doing, done
	cases := []struct {
		i, step, want int
	}{
		{0, -1, -1}, {0, 1, -1}, {1, -1, -1}, {1, 1, 2}, {2, -1, 1}, {3, 1, -1},
	}
	for _, tc := range cases {
		if got := ColumnNeighbour(cols, tc.i, tc.step); got != tc.want {
			t.Errorf("neighbour of %d by %d = %d, want %d", tc.i, tc.step, got, tc.want)
		}
	}
	if got := SwappedOrder(cols, 1, 2); !slices.Equal(got, []string{"review", "doing", "todo", "done"}) {
		t.Errorf("swapped order = %v", got)
	}
}

// arrangedBoard is the three task board read through a fake that holds the
// given arrangement of its columns.
func arrangedBoard(t *testing.T, prefs ...core.GroupPref) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	svc.tasks = slices.Clone(m.tasks)
	svc.groupPrefs = map[string][]core.GroupPref{"project:infra": prefs}
	m.svc = svc
	m, _ = m.reduce(run(t, m.loadTasks()))
	return m, svc
}

// press sends one key and runs what it asked for and the reload after it.
func press(t *testing.T, m Model, k string) Model {
	t.Helper()
	next, cmd := m.reduce(pressKey(k))
	if cmd == nil {
		return next
	}
	next, reload := next.reduce(run(t, cmd))
	if next.err != "" {
		t.Fatalf("pressing %s: %s", k, next.err)
	}
	if reload != nil {
		next, _ = next.reduce(reload())
	}
	return next
}

func TestTheBoardReadsTheArrangementAndTheKeysChangeIt(t *testing.T) {
	m, svc := arrangedBoard(t, core.GroupPref{Group: "review", Hidden: true})
	if got := columnKeys(m.columns); !slices.Equal(got, []string{"todo", "doing", "done"}) {
		t.Fatalf("drawn %v, want review hidden", got)
	}

	m = press(t, m, "Z")
	if got := columnKeys(m.columns); !slices.Equal(got, []string{"doing", "done"}) {
		t.Fatalf("after Z drawn %v, want todo hidden", got)
	}
	m.sel = Selection{Col: 1}
	m = press(t, m, "+")
	if got := columnKeys(m.columns); !slices.Equal(got, []string{"done", "doing"}) || !m.columns[0].Starred {
		t.Fatalf("after + drawn %v, want done starred first", got)
	}
	if !strings.Contains(m.Frame(), StarredColumnMark+" Done") {
		t.Error("the starred column is not marked")
	}
	m.sel = Selection{Col: 1}
	m = press(t, m, "z")
	if !m.columns[1].Collapsed || !strings.Contains(m.Frame(), CollapsedNote) {
		t.Fatalf("after z doing = %+v", m.columns[1])
	}

	m = press(t, m, "=")
	if got := columnKeys(m.columns); !slices.Equal(got, []string{"done", "todo", "doing", "review"}) {
		t.Fatalf("after = drawn %v, want every column back", got)
	}
	m.sel = Selection{Col: 2}
	m = press(t, m, "<")
	if got := columnKeys(m.columns); !slices.Equal(got, []string{"done", "doing", "todo", "review"}) {
		t.Fatalf("after < drawn %v, want doing before todo", got)
	}
	if m.columns[m.sel.Col].Key != "doing" {
		t.Errorf("the selection did not follow the moved column: %v", m.sel)
	}
	for _, p := range svc.groupPrefs["project:infra"] {
		if p.Group == "doing" && !p.Collapsed {
			t.Errorf("the move lost the collapse: %+v", p)
		}
	}
}

func TestColumnKeysAreNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := arrangedBoard(t)
	m.actor = &core.Actor{ID: "u-none", TenantID: "t", Kind: core.ActorAgent, Scopes: []core.Scope{core.ScopeProjectRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("SetGroupPref") {
		t.Fatal("the reader may arrange, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "Z" || e.Keys == "<" }) {
		t.Errorf("a reader who may not arrange is told about the column keys: %+v", help)
	}
	for _, k := range []string{"Z", "+", "z", "<", ">"} {
		_, cmd := m.reduce(pressKey(k))
		if cmd != nil {
			t.Errorf("%s asked for work from a reader who may not arrange", k)
		}
	}
	if len(svc.groupPrefs["project:infra"]) != 0 {
		t.Fatalf("the arrangement changed: %+v", svc.groupPrefs)
	}
}

func TestThePaletteOffersOnlyTheColumnActionsThatApply(t *testing.T) {
	m, _ := arrangedBoard(t, core.GroupPref{Group: "done", Starred: true})
	names := func(m Model) []string {
		m, _ = m.reduce(pressKey(":"))
		var out []string
		for _, e := range m.paletteEntries() {
			out = append(out, e.Name)
		}
		return out
	}
	m.sel = Selection{Col: 1}
	got := names(m)
	for _, want := range []string{"hide column", "star column", "collapse column", "move column right"} {
		if !slices.Contains(got, want) {
			t.Errorf("the palette does not offer %q: %v", want, got)
		}
	}
	for _, absent := range []string{"move column left", "show hidden columns"} {
		if slices.Contains(got, absent) {
			t.Errorf("the palette offers %q with nothing to do: %v", absent, got)
		}
	}
	m.view = viewDetail
	if slices.Contains(names(m), "hide column") {
		t.Error("the palette offers a column action away from the board")
	}
}
