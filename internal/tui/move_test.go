// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestTheMoveKeyOffersEveryOtherProject(t *testing.T) {
	m, svc := copyBoard(t)
	task, _ := m.selectedTask()
	next, _ := m.reduce(pressKey("d"))
	if next.form.Kind != formMove {
		t.Fatalf("d opened form %v, want the move form", next.form.Kind)
	}
	var want []string
	for _, p := range m.projects {
		if p.ID != task.ProjectID {
			want = append(want, strings.ToUpper(p.Key))
		}
	}
	slices.Sort(want)
	if got := next.form.Fields[0].Options; !slices.Equal(got, want) {
		t.Errorf("projects offered = %v, want %v, every project but the task's own", got, want)
	}
	if len(svc.moves) != 0 {
		t.Fatalf("opening the form moved: %v", svc.moves)
	}
}

func TestApplyingTheMoveFormMovesAndNamesTheNewReference(t *testing.T) {
	m, svc := copyBoard(t)
	m, _ = m.reduce(pressKey("d"))
	m.form = m.form.SetValue("project", "WEB")
	next, cmd := m.submitForm()
	msg := run(t, cmd)
	want := core.MoveTaskInput{ProjectRef: "web"}
	if len(svc.moves) != 1 || svc.moves[0].ref != (core.TaskRef{ID: "a"}) || svc.moves[0].in != want {
		t.Fatalf("moves = %+v, want one of a to web", svc.moves)
	}
	after, _ := next.reduce(msg)
	if w := "moved infra-a to WEB-7, now todo"; after.status != w || after.err != "" {
		t.Errorf("status = %q (err %q), want %q", after.status, after.err, w)
	}
}
