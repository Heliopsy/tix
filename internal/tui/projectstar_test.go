// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// starProjectsModel is the projects view over three projects, web starred.
func starProjectsModel(t *testing.T) (Model, *fakeService) {
	t.Helper()
	svc := newFakeService()
	svc.projects = []core.Project{
		{ID: "p1", Key: "infra", Name: "Infrastructure"},
		{ID: "p2", Key: "web", Name: "Website", Starred: true},
		{ID: "p3", Key: "ops", Name: "Operations"},
	}
	m := New(Config{Access: fullAccess(), Actor: fullActor(), Service: svc, Environ: []string{"NO_COLOR=1"},
		Now: func() time.Time { return time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC) }})
	m.width, m.height = 120, 40
	m, _ = m.reduce(run(t, m.loadProjects()))
	return m, svc
}

// projectRowFor is the one body line naming a project's key.
func projectRowFor(t *testing.T, m Model, key string) string {
	t.Helper()
	for _, line := range m.projectLines() {
		if strings.Contains(line, strings.ToUpper(key)+" ") {
			return line
		}
	}
	return ""
}

func TestTheProjectsViewMarksAndTogglesAStar(t *testing.T) {
	m, svc := starProjectsModel(t)
	if row := projectRowFor(t, m, "web"); !strings.Contains(row, "★") {
		t.Errorf("the starred project carries no star: %q", row)
	}
	if row := projectRowFor(t, m, "infra"); strings.Contains(row, "★") {
		t.Errorf("an unstarred project carries a star: %q", row)
	}

	m.projectSel = 0
	next, cmd := m.reduce(pressKey("*"))
	msg := run(t, cmd)
	if !slices.Equal(svc.projectStars, []string{"infra"}) || !svc.projects[0].Starred {
		t.Fatalf("star reached %v, want infra starred", svc.projectStars)
	}
	after, reload := next.reduce(msg)
	if after.err != "" || !strings.Contains(after.status, "starred INFRA") {
		t.Fatalf("status %q err %q after starring", after.status, after.err)
	}
	after, _ = after.reduce(run(t, reload))
	if row := projectRowFor(t, after, "infra"); !strings.Contains(row, "★") {
		t.Errorf("the reload does not show the new star: %q", row)
	}

	after.projectSel = 0
	next, cmd = after.reduce(pressKey("*"))
	next.reduce(run(t, cmd))
	if svc.projects[0].Starred {
		t.Fatal("pressing the star on a starred project did not unstar it")
	}
}

func TestTheProjectsViewFiltersToStarred(t *testing.T) {
	m, _ := starProjectsModel(t)
	m.projectSel = 2
	m, _ = m.reduce(pressKey("/"))
	if len(m.projects) != 1 || m.projects[0].Key != "web" {
		t.Fatalf("starred only lists %+v, want web alone", m.projects)
	}
	if head := m.projectLines()[0]; !strings.Contains(head, "1 starred projects") {
		t.Errorf("the heading does not say the list is starred only: %q", head)
	}
	if m.projectSel != 0 {
		t.Errorf("the selection is past the list: %d", m.projectSel)
	}
	m, _ = m.reduce(pressKey("/"))
	if len(m.projects) != 3 {
		t.Fatalf("the filter did not come off: %d projects", len(m.projects))
	}

	help := m.keys.ViewHelp(viewProjects, m.permits())
	for _, want := range []HelpEntry{{Keys: "*", Desc: "star project"}, {Keys: "/", Desc: "starred only"}} {
		if !slices.Contains(help, want) {
			t.Errorf("help lacks %+v: %+v", want, help)
		}
	}
}

func TestTheStarredFilterSaysWhenNothingIsStarred(t *testing.T) {
	m, svc := starProjectsModel(t)
	svc.projects[1].Starred = false
	m, _ = m.reduce(run(t, m.loadProjects()))
	m, _ = m.reduce(pressKey("/"))
	if body := strings.Join(m.projectLines(), "\n"); !strings.Contains(body, "No starred projects.") {
		t.Fatalf("an empty starred list says nothing useful:\n%s", body)
	}
}
