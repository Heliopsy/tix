// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestEnterOnACardReadsTheTaskFromTheService(t *testing.T) {
	b := newLiveBoard(t)
	m := press(t, b.model, "enter")
	if m.view != viewDetail || m.detail == nil {
		t.Fatalf("enter on a card opened view %s", viewName(m.view))
	}
	if m.detail.task.ID != b.task.ID || m.detail.task.Title != "walk me" {
		t.Errorf("the detail holds %+v, want the card's task", m.detail.task)
	}
}

func TestOpeningABoardReadsItsWorkflowAndTasks(t *testing.T) {
	b := newLiveBoard(t)
	project := b.model.project
	m := runCmd(t, b.model, b.model.loadBoard(project))
	if m.err != "" {
		t.Fatalf("opening the board failed: %s", m.err)
	}
	if countTasks(m.columns) != 1 || len(m.columns) == 0 {
		t.Errorf("the board holds %d tasks in %d columns", countTasks(m.columns), len(m.columns))
	}

	project.WorkflowID = "no-such-workflow"
	m = runCmd(t, b.model, b.model.loadBoard(project))
	if !strings.Contains(m.err, "not readable") {
		t.Errorf("a project on a workflow nobody can read opened with error %q", m.err)
	}
}

func TestTickingAChecklistItemWritesTheTask(t *testing.T) {
	b := newLiveBoard(t)
	task, err := b.local.CreateTask(b.ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "list",
		Body: "- [ ] one\n- [ ] two\n"})
	if err != nil {
		t.Fatalf("creating the task: %v", err)
	}
	m, _ := b.model.reduce(boardMsg{project: b.model.project, workflow: b.model.workflow, tasks: []core.Task{*task}})

	tick := func(m Model, text string) Model {
		t.Helper()
		m, _ = m.reduce(pressKey("["))
		if m.prompt != promptCheck {
			t.Fatal("[ opened no checklist prompt")
		}
		m.input.SetValue(text)
		m, cmd := m.reduce(pressKey("enter"))
		if cmd == nil {
			t.Fatalf("answering %q started nothing; error %q", text, m.err)
		}
		m, _ = m.reduce(cmd())
		return m
	}

	m = tick(m, "9")
	if !strings.Contains(m.err, "must be a number from 1 to 2") {
		t.Errorf("an item that does not exist was answered with %q", m.err)
	}
	m = tick(m, "2")
	if m.err != "" {
		t.Fatalf("ticking item 2 failed: %s", m.err)
	}
	got, err := b.local.GetTask(b.ctx, core.TaskRef{ID: task.ID})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "- [ ] one") || !strings.Contains(got.Body, "- [x] two") {
		t.Errorf("the body after ticking item 2:\n%s", got.Body)
	}
}

func TestTheProjectScreenScrollsWithinItsLinesAndEscLeaves(t *testing.T) {
	b := newLiveBoard(t)
	m := b.model
	m.height = 12
	m = press(t, m, "w")
	if m.view != viewProject || m.setup == nil {
		t.Fatalf("w opened view %s", viewName(m.view))
	}
	lines, rows := m.setupBody()
	if len(lines) <= rows {
		t.Fatalf("the project screen fits a %d row terminal (%d lines), so scrolling proves nothing", m.height, len(lines))
	}
	for _, step := range []struct {
		key  string
		want int
	}{
		{"G", len(lines) - rows}, {"g", 0}, {"k", 0}, {"j", 1}, {"k", 0},
	} {
		m, _ = m.reduce(pressKey(step.key))
		if m.setupOff != step.want {
			t.Fatalf("after %s the project screen is scrolled to %d, want %d", step.key, m.setupOff, step.want)
		}
	}
	m, _ = m.reduce(pressKey("esc"))
	if m.view == viewProject {
		t.Error("esc did not leave the project screen")
	}
}
