// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// copyBoard is the three task board with two projects to copy into and task a
// selected.
func copyBoard(t *testing.T) (Model, *fakeService) {
	t.Helper()
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m.projects = []core.Project{{ID: "p2", Key: "web"}, {ID: "p1", Key: "infra"}}
	m.sel = PreserveSelection(m.columns, m.sel, "a")
	return m, svc
}

func TestTheCopyKeyOpensAFormOnTheBoardsProject(t *testing.T) {
	m, svc := copyBoard(t)
	next, _ := m.reduce(pressKey("Y"))
	if next.form.Kind != formCopy {
		t.Fatalf("Y opened form %v, want the copy form", next.form.Kind)
	}
	want := map[string]string{
		"project": "infra", "title": "task a", "keep_status": noValue, "comments": noValue,
		"subtasks": noValue, "deps": noValue, "link": yesValue,
	}
	for key, value := range want {
		if got := next.form.Value(key); got != value {
			t.Errorf("%s = %q, want %q", key, got, value)
		}
	}
	if field := next.form.Fields[0]; !slices.Equal(field.Options, []string{"infra", "web"}) {
		t.Errorf("projects offered = %v, want every project", field.Options)
	}
	if len(svc.copies) != 0 {
		t.Fatalf("opening the form copied: %v", svc.copies)
	}
}

func TestApplyingTheCopyFormCopiesAndNamesTheCopy(t *testing.T) {
	cases := []struct {
		name   string
		answer map[string]string
		want   core.CopyTaskInput
	}{
		{"as offered", nil, core.CopyTaskInput{ProjectRef: "infra", Link: true}},
		{"elsewhere with everything", map[string]string{
			"project": "web", "title": "again", "keep_status": yesValue, "comments": yesValue,
			"subtasks": yesValue, "deps": yesValue, "link": noValue,
		}, core.CopyTaskInput{ProjectRef: "web", Title: "again", KeepStatus: true, WithComments: true,
			WithSubtasks: true, WithDependencies: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, svc := copyBoard(t)
			m, _ = m.reduce(pressKey("Y"))
			for key, value := range tc.answer {
				m.form = m.form.SetValue(key, value)
			}
			next, cmd := m.submitForm()
			msg := run(t, cmd)
			if len(svc.copies) != 1 || svc.copies[0].ref != (core.TaskRef{ID: "a"}) || svc.copies[0].in != tc.want {
				t.Fatalf("copies = %+v, want one of a with %+v", svc.copies, tc.want)
			}
			after, reload := next.reduce(msg)
			if want := "copied infra-a to " + tc.want.ProjectRef + "-99"; after.status != want || after.err != "" {
				t.Errorf("status = %q (err %q), want %q", after.status, after.err, want)
			}
			if reload == nil || after.form.Open() {
				t.Error("the copy neither reloaded the board nor closed the form")
			}
		})
	}
}

func TestTheCopyKeyIsNeitherOfferedNorSentWithoutTheAuthority(t *testing.T) {
	m, svc := copyBoard(t)
	m.actor = &core.Actor{ID: "u-ro", TenantID: "t", Kind: core.ActorAgent,
		Scopes: []core.Scope{core.ScopeProjectRead, core.ScopeTaskRead}}
	m.allowed = resolveActions(m.actor)
	if m.mayPerform("CopyTask") {
		t.Fatal("the reader may copy, so this test proves nothing")
	}
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "Y" }) {
		t.Errorf("a reader who may not copy is told about Y: %+v", help)
	}
	next, cmd := m.reduce(pressKey("Y"))
	if cmd != nil {
		next, _ = next.reduce(run(t, cmd))
	}
	if next.form.Open() || len(svc.copies) != 0 {
		t.Fatalf("a reader who may not copy got form %v and calls %v", next.form.Kind, svc.copies)
	}
}

func TestTheCopyKeyIsOfferedInHelpAndPalette(t *testing.T) {
	m, _ := copyBoard(t)
	help := m.keys.ViewHelp(viewBoard, m.permits())
	if !slices.ContainsFunc(help, func(e HelpEntry) bool { return e.Keys == "Y" && e.Desc == "copy task" }) {
		t.Errorf("help does not list the copy key: %+v", help)
	}
	m, _ = m.reduce(pressKey(":"))
	if !slices.ContainsFunc(m.paletteEntries(), func(e PaletteEntry) bool { return e.Name == "copy task" }) {
		t.Errorf("the palette does not offer copy: %+v", m.paletteEntries())
	}
}

func TestCopyFormWithoutProjectsIsNotOpened(t *testing.T) {
	m, svc := copyBoard(t)
	m.projects = nil
	next, _ := m.reduce(pressKey("Y"))
	if next.form.Open() || next.err == "" || len(svc.copies) != 0 {
		t.Fatalf("form %v err %q calls %v, want a refusal and no form", next.form.Kind, next.err, svc.copies)
	}
}
