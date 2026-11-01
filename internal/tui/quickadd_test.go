// SPDX-License-Identifier: AGPL-3.0-or-later

package tui

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/quickadd"
)

// The board's present is Thursday 1 January 2026, so due:thu is a week away.
func TestTheNewTaskPromptReadsQuickAddSyntax(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := typeInto(t, m, "n", "Fix login !high #auth @raj due:thu +web ^web-3 status:doing")
	if m.prompt != promptNone || cmd == nil {
		t.Fatalf("an acceptable line left the prompt %v with command %v", m.prompt, cmd)
	}
	cmd()
	if len(svc.created) != 1 {
		t.Fatalf("created = %+v", svc.created)
	}
	in := svc.created[0]
	if in.Title != "Fix login" || in.Priority != core.PriorityHigh || !slices.Equal(in.Tags, []string{"auth"}) ||
		in.AssigneeActorID != "raj" || in.ProjectRef != "web" || in.ParentRef != "web-3" || in.Status != "doing" {
		t.Errorf("created %+v", in)
	}
	if in.DueAt == nil || !in.DueAt.Equal(time.Date(2026, 1, 8, 0, 0, 0, 0, time.UTC)) {
		t.Errorf("due:thu from Thursday 1 January gave %v, want 8 January", in.DueAt)
	}
}

func TestTheNewTaskPromptDefaultsToTheOpenProject(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	_, cmd := typeInto(t, m, "n", "plain title #a")
	cmd()
	if len(svc.created) != 1 || svc.created[0].ProjectRef != "infra" || svc.created[0].Title != "plain title" {
		t.Errorf("created %+v", svc.created)
	}
}

func TestARefusedLineKeepsTheNewTaskPromptOpen(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	m.svc = svc
	m, cmd := typeInto(t, m, "n", "Fix !high !low")
	if m.prompt != promptNewTask {
		t.Fatalf("a refused line closed the prompt")
	}
	if cmd != nil {
		cmd()
	}
	if len(svc.created) != 0 {
		t.Fatalf("a refused line was sent: %+v", svc.created)
	}
	if got := m.input.Value(); got != "Fix !high !low" {
		t.Errorf("the prompt holds %q, not the line", got)
	}
	if rows := strings.Join(m.promptPanel().Rows, "\n"); !strings.Contains(rows, "!low") {
		t.Errorf("the panel does not say why:\n%s", rows)
	}
	m, _ = m.reduce(pressKey("x"))
	if m.quickErr != "" {
		t.Errorf("editing the line left the old refusal standing: %q", m.quickErr)
	}
}

func TestAServiceRefusalReopensThePromptOnTheLine(t *testing.T) {
	m := boardModel(t)
	svc := newFakeService()
	svc.createErr = core.NotFound("no actor with handle or id %q", "nobody")
	m.svc = svc
	m, cmd := typeInto(t, m, "n", "Fix login @nobody")
	if cmd == nil {
		t.Fatal("the line was not sent")
	}
	m, _ = m.reduce(cmd())
	if m.prompt != promptNewTask {
		t.Fatalf("the refusal left the prompt %v", m.prompt)
	}
	if got := m.input.Value(); got != "Fix login @nobody" {
		t.Errorf("the reopened prompt holds %q", got)
	}
	if rows := strings.Join(m.promptPanel().Rows, "\n"); !strings.Contains(rows, "nobody") {
		t.Errorf("the panel does not name the handle:\n%s", rows)
	}
}

func TestTheNewTaskPromptPreviewsWhatTheLineSets(t *testing.T) {
	m := boardModel(t)
	m.svc = newFakeService()
	m, _ = m.reduce(pressKey("n"))
	m.input.SetValue("Fix login !high #auth due:tomorrow")
	rows := strings.Join(m.promptPanel().Rows, "\n")
	if !strings.Contains(rows, "sets priority high, tag auth, due 2026-01-02") {
		t.Errorf("the panel does not preview the line:\n%s", rows)
	}
	m.input.SetValue("just a title")
	if rows := m.promptPanel().Rows; len(rows) != 1 {
		t.Errorf("a line setting nothing drew %d rows:\n%s", len(rows), strings.Join(rows, "\n"))
	}
}

func TestQuickSummary(t *testing.T) {
	r, err := quickadd.Parse("x @raj +web", time.Now())
	if err != nil {
		t.Fatal(err)
	}
	if got := QuickSummary(r); got != "assignee raj, project web" {
		t.Errorf("QuickSummary = %q", got)
	}
}
