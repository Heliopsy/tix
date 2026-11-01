// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/capability"
	"github.com/heliopsy/tix/internal/core"
)

// templateBoard is the three task board with task a selected and two
// templates the board's project can use.
func templateBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	svc.tpl.list = []core.TaskTemplate{
		{Name: "standup", Task: core.TemplateTask{Title: "Standup {N}"}, Uses: 1},
		{Name: "release", ProjectKey: "infra", DueIn: core.Duration(72 * time.Hour),
			Task: core.TemplateTask{Title: "Ship", Subtasks: []core.TemplateTask{{Title: "notes"}}}},
	}
	m.svc = svc
	m.sel = PreserveSelection(m.columns, m.sel, "a")
	return m, svc
}

// openTemplates presses the template key and delivers the listing it read.
func openTemplates(t *testing.T, m Model) Model {
	t.Helper()
	next, cmd := m.reduce(pressKey("B"))
	if cmd == nil {
		return next
	}
	next, _ = next.reduce(run(t, cmd))
	return next
}

func TestEveryTemplateOperationHasAKeyThatReachesIt(t *testing.T) {
	bound := []string{"ListTemplates", "GetTemplate", "SaveTemplate", "UpdateTemplate", "DeleteTemplate", "UseTemplate"}
	k := DefaultKeyMap()
	for _, method := range bound {
		op, ok := capability.ByMethod(method)
		if !ok {
			t.Fatalf("%s is not in the registry", method)
		}
		if op.TUI != viewName(viewBoard) {
			t.Errorf("%s: bound to view %q, not the board", op.Name, op.TUI)
		}
		if _, exempted := op.ExemptionFor(capability.SurfaceTUI); exempted {
			t.Errorf("%s: the registry both binds and exempts the terminal", op.Name)
		}
		found := false
		for _, action := range k.taskBindings() {
			if slices.Contains(action.gate.methods, method) || slices.Contains(action.gate.needs, method) {
				found = true
			}
		}
		if !found {
			t.Errorf("%s is bound in the registry and no key acts through it", method)
		}
	}
}

func TestTheTemplateKeyListsTheBoardsProjectsTemplates(t *testing.T) {
	m, svc := templateBoard(t)
	next := openTemplates(t, m)
	if next.form.Kind != formTemplates {
		t.Fatalf("B opened form %v (err %q), want the template form", next.form.Kind, next.err)
	}
	if len(svc.tpl.filters) != 1 || svc.tpl.filters[0].ProjectRef != "infra" {
		t.Errorf("listing asked for %+v, want the board's project", svc.tpl.filters)
	}
	if got := next.form.Fields[0].Options; !slices.Equal(got, []string{"release", "standup"}) {
		t.Errorf("templates offered = %v", got)
	}
	if got := next.form.Fields[1].Options; !slices.Equal(got, []string{templateUse, templateShow, templateRename, templateDelete}) {
		t.Errorf("actions offered = %v", got)
	}
	if len(svc.tpl.used)+len(svc.tpl.deleted)+len(svc.tpl.renamed) != 0 {
		t.Fatal("opening the form changed something")
	}
}

func TestTheTemplateFormPerformsWhatItWasAsked(t *testing.T) {
	three := core.Duration(72 * time.Hour)
	cases := []struct {
		name   string
		answer map[string]string
		check  func(t *testing.T, svc *fakeService, status string)
	}{
		{"create as offered", nil, func(t *testing.T, svc *fakeService, status string) {
			want := templateUseCall{ref: "release", in: core.UseTemplateInput{ProjectRef: "infra"}}
			if len(svc.tpl.used) != 1 || svc.tpl.used[0].ref != want.ref || svc.tpl.used[0].in.ProjectRef != "infra" ||
				svc.tpl.used[0].in.DueIn != nil || svc.tpl.used[0].in.Title != "" {
				t.Fatalf("uses = %+v, want %+v", svc.tpl.used, want)
			}
			if status != "created INFRA-77 from release" {
				t.Errorf("status = %q", status)
			}
		}},
		{"create with a title and an offset", map[string]string{"title": " run {N} ", "due": "3d"},
			func(t *testing.T, svc *fakeService, _ string) {
				if len(svc.tpl.used) != 1 || svc.tpl.used[0].in.Title != "run {N}" ||
					svc.tpl.used[0].in.DueIn == nil || *svc.tpl.used[0].in.DueIn != three {
					t.Fatalf("uses = %+v", svc.tpl.used)
				}
			}},
		{"show", map[string]string{"action": templateShow}, func(t *testing.T, svc *fakeService, status string) {
			if !slices.Equal(svc.tpl.read, []string{"release"}) {
				t.Fatalf("reads = %v", svc.tpl.read)
			}
			want := `release: INFRA, 2 tasks, due in 3d, used 0 times, title "Ship"`
			if status != want {
				t.Errorf("status = %q, want %q", status, want)
			}
		}},
		{"rename", map[string]string{"action": templateRename, "name": " ship "},
			func(t *testing.T, svc *fakeService, status string) {
				if len(svc.tpl.renamed) != 1 || svc.tpl.renamed[0] != [2]string{"release", "ship"} {
					t.Fatalf("renames = %v", svc.tpl.renamed)
				}
				if status != "renamed template release to ship" {
					t.Errorf("status = %q", status)
				}
			}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := templateBoard(t)
			m = openTemplates(t, m)
			for key, value := range tc.answer {
				m.form = m.form.SetValue(key, value)
			}
			next, cmd := m.submitForm()
			after, _ := next.reduce(run(t, cmd))
			if after.err != "" || after.form.Open() {
				t.Fatalf("err %q form %v", after.err, after.form.Kind)
			}
			tc.check(t, svc, after.status)
		})
	}
}

func TestTheTemplateFormRefusesBeforeCalling(t *testing.T) {
	cases := []struct {
		name   string
		answer map[string]string
		want   string
	}{
		{"an offset that does not read", map[string]string{"due": "soon"}, "due in"},
		{"a rename to nothing", map[string]string{"action": templateRename, "name": "  "}, "a new name is required"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := templateBoard(t)
			m = openTemplates(t, m)
			for key, value := range tc.answer {
				m.form = m.form.SetValue(key, value)
			}
			next, cmd := m.submitForm()
			if cmd != nil || !strings.Contains(next.err, tc.want) || len(svc.tpl.used)+len(svc.tpl.renamed) != 0 {
				t.Fatalf("cmd %v err %q uses %v renames %v", cmd != nil, next.err, svc.tpl.used, svc.tpl.renamed)
			}
		})
	}
}

func TestDeletingATemplateAsksFirst(t *testing.T) {
	m, svc := templateBoard(t)
	m = openTemplates(t, m)
	m.form = m.form.SetValue("action", templateDelete)
	m, cmd := m.submitForm()
	if cmd != nil || m.confirm.Question() != "delete template release?" {
		t.Fatalf("question %q, cmd %v", m.confirm.Question(), cmd != nil)
	}
	if len(svc.tpl.deleted) != 0 {
		t.Fatal("deleted before the reader agreed")
	}
	refused, cmd := m.reduce(pressKey("esc"))
	if cmd != nil || refused.confirm.Open() || len(svc.tpl.deleted) != 0 {
		t.Fatal("escape deleted or left the question open")
	}
	agreed, cmd := m.reduce(pressKey("y"))
	after, _ := agreed.reduce(run(t, cmd))
	if !slices.Equal(svc.tpl.deleted, []string{"release"}) || after.status != "deleted template release" {
		t.Fatalf("deleted %v status %q", svc.tpl.deleted, after.status)
	}
}

func TestNoTemplatesSaysHowToMakeOne(t *testing.T) {
	m, svc := templateBoard(t)
	svc.tpl.list = nil
	next := openTemplates(t, m)
	if next.form.Open() || !strings.Contains(next.err, "E saves") {
		t.Fatalf("form %v err %q", next.form.Kind, next.err)
	}
}

func TestTheSaveKeyKeepsTheTaskAsATemplate(t *testing.T) {
	cases := []struct {
		name   string
		answer map[string]string
		want   core.SaveTemplateInput
	}{
		{"as offered", nil, core.SaveTemplateInput{Name: "task a", TaskRef: "infra-a", WithSubtasks: true}},
		{"tenant-wide without subtasks", map[string]string{"name": "daily", "scope": scopeTenant, "subtasks": noValue},
			core.SaveTemplateInput{Name: "daily", TaskRef: "infra-a", TenantWide: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := templateBoard(t)
			m, _ = m.reduce(pressKey("E"))
			if m.form.Kind != formSaveTemplate {
				t.Fatalf("E opened form %v", m.form.Kind)
			}
			for key, value := range tc.answer {
				m.form = m.form.SetValue(key, value)
			}
			next, cmd := m.submitForm()
			after, _ := next.reduce(run(t, cmd))
			if len(svc.tpl.saved) != 1 || svc.tpl.saved[0] != tc.want {
				t.Fatalf("saved %+v, want %+v", svc.tpl.saved, tc.want)
			}
			if want := "saved infra-a as template " + tc.want.Name; after.status != want {
				t.Errorf("status = %q, want %q", after.status, want)
			}
		})
	}
}

func TestTemplateKeysAreNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := templateBoard(t)
	m.actor = &core.Actor{ID: "u-ro", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("ListTemplates") || m.mayPerform("SaveTemplate") {
		t.Fatal("the reader may use templates, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "B" || e.Keys == "E" }) {
		t.Errorf("a reader without the authority is told about B or E: %+v", help)
	}
	next := openTemplates(t, m)
	next, cmd := next.reduce(pressKey("E"))
	if cmd != nil || next.form.Open() || len(svc.tpl.filters)+len(svc.tpl.saved) != 0 {
		t.Fatalf("form %v listings %v saves %v", next.form.Kind, svc.tpl.filters, svc.tpl.saved)
	}
}

func TestTemplateKeysAreOfferedInHelpAndPalette(t *testing.T) {
	m, _ := templateBoard(t)
	help := m.keys.ViewHelp(viewBoard, m.permits())
	for _, want := range []HelpEntry{{"B", "templates"}, {"E", "save as template"}} {
		if !slices.Contains(help, want) {
			t.Errorf("help does not list %+v", want)
		}
	}
	m, _ = m.reduce(pressKey(":"))
	for _, name := range []string{"templates", "save as template"} {
		if !slices.ContainsFunc(m.paletteEntries(), func(e PaletteEntry) bool { return e.Name == name }) {
			t.Errorf("the palette does not offer %q", name)
		}
	}
}

func TestTemplateFormOffersOnlyThePermittedActions(t *testing.T) {
	tpls := []core.TaskTemplate{{Name: "a"}}
	only := func(methods ...string) func(string) bool {
		return func(m string) bool { return slices.Contains(methods, m) }
	}
	if f := TemplatesForm(tpls, only("GetTemplate")); !slices.Equal(f.Fields[1].Options, []string{templateShow}) {
		t.Errorf("show only = %v", f.Fields[1].Options)
	}
	if f := TemplatesForm(tpls, only()); f.Open() {
		t.Error("a form with no action was opened")
	}
	if f := TemplatesForm(nil, only("UseTemplate")); f.Open() {
		t.Error("a form with no template was opened")
	}
}
