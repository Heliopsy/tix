// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// savedBoard is the three task board read through a fake holding the given
// saved filters.
func savedBoard(t *testing.T, filters ...core.SavedFilter) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	svc.tasks = slices.Clone(m.tasks)
	svc.savedFilters = filters
	m.svc = svc
	m, _ = m.reduce(run(t, m.loadTasks()))
	return m, svc
}

// openPicker presses s and folds the listing it asked for back in.
func openPicker(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.reduce(pressKey("s"))
	next, _ = next.reduce(run(t, cmd))
	return next
}

func TestThePickerListsSavedFiltersAndAppliesOneWithItsSort(t *testing.T) {
	m, _ := savedBoard(t,
		core.SavedFilter{ID: "1", Name: "mine", Query: "status:todo", Mine: true},
		core.SavedFilter{ID: "2", Name: "doing by due", Query: "status:doing", Sort: "due_at", Mine: true},
		core.SavedFilter{ID: "3", Name: "team", Query: "tag:ops", Shared: true},
	)
	m = openPicker(t, m)
	if m.form.Kind != formSavedFilter {
		t.Fatalf("s opened %v, want the saved filter picker (err %q)", m.form.Kind, m.err)
	}
	options := m.form.Fields[0].Options
	want := []string{"mine  status:todo", "doing by due  status:doing", "team (shared)  tag:ops"}
	if !slices.Equal(options, want) {
		t.Fatalf("the picker offers %q, want %q", options, want)
	}
	m, _ = m.reduce(pressKey("down"))
	m, cmd := m.reduce(pressKey("enter"))
	if m.form.Open() {
		t.Fatal("enter left the picker open")
	}
	if cmd == nil {
		t.Fatal("applying a saved filter did not reload the board")
	}
	if m.filterText != "sort:due_at status:doing" || m.filter.Page.Sort != "due_at" {
		t.Fatalf("filter = %q sorted by %q, want the saved one with its sort", m.filterText, m.filter.Page.Sort)
	}
	if got := columnKeys(m.columns); len(m.columns[0].Tasks) != 0 || len(m.columns[1].Tasks) != 1 {
		t.Errorf("the board did not narrow to doing: %v %+v", got, m.columns)
	}
	if !strings.Contains(m.status, "doing by due") {
		t.Errorf("the status bar says %q", m.status)
	}
}

func TestEscapeClosesOnlyThePicker(t *testing.T) {
	m, _ := savedBoard(t, core.SavedFilter{ID: "1", Name: "mine", Query: "status:todo", Mine: true})
	m = m.applyFilterText("status:doing")
	m = openPicker(t, m)
	m, cmd := m.reduce(pressKey("esc"))
	if m.form.Open() || cmd != nil {
		t.Fatalf("esc left the picker open or asked for work: %v", m.form.Kind)
	}
	if m.view != viewBoard {
		t.Fatalf("esc left the board for %v", m.view)
	}
	if m.filterText != "status:doing" {
		t.Errorf("esc changed the board's filter to %q", m.filterText)
	}
}

func TestAnEmptyListSaysHowToSaveOne(t *testing.T) {
	m, _ := savedBoard(t)
	m = openPicker(t, m)
	if m.form.Open() {
		t.Fatal("an empty picker was opened")
	}
	if !strings.Contains(m.err, "K") || !strings.Contains(m.err, "tix filter save") {
		t.Errorf("the empty list says %q", m.err)
	}
}

func TestSavingKeepsTheBoardsFilterUnderAName(t *testing.T) {
	m, svc := savedBoard(t)
	if _, cmd := m.reduce(pressKey("K")); cmd != nil {
		t.Fatal("K asked for work with no filter in force")
	}
	m = m.applyFilterText("status:todo -tag:ops")
	m, _ = m.reduce(pressKey("K"))
	if m.prompt != promptSaveFilter {
		t.Fatalf("K opened prompt %v (err %q)", m.prompt, m.err)
	}
	m = typeText(m, "my todo")
	m, cmd := m.reduce(pressKey("enter"))
	m, _ = m.reduce(run(t, cmd))
	if len(svc.savedInputs) != 1 {
		t.Fatalf("CreateSavedFilter was called %d times", len(svc.savedInputs))
	}
	if in := svc.savedInputs[0]; in.Name != "my todo" || in.Query != "status:todo -tag:ops" {
		t.Errorf("kept %+v, want the board's filter under the typed name", in)
	}
	if !strings.Contains(m.status, "saved filter my todo") {
		t.Errorf("the status bar says %q", m.status)
	}

	svc.savedErr = errors.New("you already have a saved filter named \"my todo\"")
	m, _ = m.reduce(pressKey("K"))
	m = typeText(m, "my todo")
	m, cmd = m.reduce(pressKey("enter"))
	m, _ = m.reduce(run(t, cmd))
	if !strings.Contains(m.err, "already have") {
		t.Errorf("a refused save says %q", m.err)
	}
}

func TestSavedFilterKeysAreNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := savedBoard(t, core.SavedFilter{ID: "1", Name: "mine", Query: "status:todo", Mine: true})
	m = m.applyFilterText("status:todo")
	m.actor = &core.Actor{ID: "u-none", TenantID: "t", Kind: core.ActorAgent, Scopes: []core.Scope{core.ScopeProjectRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("ListSavedFilters") || m.mayPerform("CreateSavedFilter") {
		t.Fatal("the reader may use saved filters, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "s" || e.Keys == "K" }) {
		t.Errorf("a reader who may not read tasks is told about the saved filter keys: %+v", help)
	}
	for _, k := range []string{"s", "K"} {
		if _, cmd := m.reduce(pressKey(k)); cmd != nil {
			t.Errorf("%s asked for work from a reader who may not use saved filters", k)
		}
	}
	if len(svc.savedInputs) != 0 {
		t.Fatalf("a filter was saved: %+v", svc.savedInputs)
	}
}

func TestThePaletteOffersSavingOnlyWithAFilter(t *testing.T) {
	m, _ := savedBoard(t)
	names := func(m Model) []string {
		m, _ = m.reduce(pressKey(":"))
		var out []string
		for _, e := range m.paletteEntries() {
			out = append(out, e.Name)
		}
		return out
	}
	got := names(m)
	if !slices.Contains(got, "saved filters") || slices.Contains(got, "save the filter") {
		t.Fatalf("with no filter the palette offers %v", got)
	}
	if got := names(m.applyFilterText("status:todo")); !slices.Contains(got, "save the filter") {
		t.Errorf("with a filter the palette does not offer saving it: %v", got)
	}
}

func TestMeIsTheReader(t *testing.T) {
	m := boardModel(t)
	mine, theirs := task("a", "todo", 1, core.PriorityNormal), task("b", "todo", 2, core.PriorityNormal)
	mine.AssigneeActorID, theirs.AssigneeActorID = m.actorID(), "u-other"
	mine.ClaimedByActorID = m.actorID()
	m.tasks = []core.Task{mine, theirs}
	cases := []struct {
		expr string
		want []string
	}{
		{"assignee:me", []string{"a"}},
		{"assignee:ME", []string{"a"}},
		{"-assignee:me", []string{"b"}},
		{"claimed-by:me", []string{"a"}},
		{"assignee:u-other", []string{"b"}},
	}
	for _, tc := range cases {
		t.Run(tc.expr, func(t *testing.T) {
			next := m.applyFilterText(tc.expr)
			var got []string
			for _, task := range next.visibleTasks(next.tasks) {
				got = append(got, task.ID)
			}
			if !slices.Equal(got, tc.want) {
				t.Fatalf("%s kept %v, want %v", tc.expr, got, tc.want)
			}
		})
	}
	parsed, err := ParseFilter("assignee:me")
	if err != nil {
		t.Fatal(err)
	}
	bound := BindMe(parsed, "u-full")
	if parsed.AssigneeIDs[0] != "me" || bound.AssigneeIDs[0] != "u-full" {
		t.Errorf("BindMe wrote through the parsed filter: %v -> %v", parsed.AssigneeIDs, bound.AssigneeIDs)
	}
}
