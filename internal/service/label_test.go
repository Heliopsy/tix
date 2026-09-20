package service

import (
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
)

func TestLabelsAttachDetachAndList(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "labelled"})
	ref := core.TaskRef{ID: task.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.AddLabel(ctx, ref, "urgent"); err != nil {
		t.Fatalf("AddLabel: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("adding a label wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	if err := l.AddLabel(ctx, ref, "urgent"); err != nil {
		t.Fatalf("attaching the same label twice: %v", err)
	}
	got, err := l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if strings.Join(got.Labels, ",") != "urgent" {
		t.Errorf("labels = %v, want exactly one", got.Labels)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{Labels: []string{"urgent"}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Errorf("filtering by label returned %d tasks, want one", len(page.Tasks))
	}

	labels, err := l.ListLabels(ctx)
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	if len(labels) != 1 || labels[0].Name != "urgent" {
		t.Errorf("labels = %+v, want the one label", labels)
	}

	if err := l.RemoveLabel(ctx, ref, "urgent"); err != nil {
		t.Fatalf("RemoveLabel: %v", err)
	}
	got, err = l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if len(got.Labels) != 0 {
		t.Errorf("labels = %v, want none", got.Labels)
	}
	page, err = l.ListTasks(ctx, core.TaskFilter{Labels: []string{"urgent"}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("a detached label still matches the filter: %v", taskTitles(page.Tasks))
	}
}

func TestLabelOperationsRejectBadInput(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}

	if err := l.AddLabel(ctx, ref, "   "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("AddLabel with an empty name = %v, want invalid", err)
	}
	if err := l.RemoveLabel(ctx, ref, " "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("RemoveLabel with an empty name = %v, want invalid", err)
	}
	if err := l.RemoveLabel(ctx, ref, "never-used"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("RemoveLabel of an unknown label = %v, want not found", err)
	}
	if err := l.AddLabel(ctx, core.MustParseTaskRef("infra-99"), "x"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("AddLabel on a missing task = %v, want not found", err)
	}
	if err := l.AddLabel(t.Context(), ref, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("AddLabel unauthenticated = %v", err)
	}
	if err := l.RemoveLabel(t.Context(), ref, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("RemoveLabel unauthenticated = %v", err)
	}
	if _, err := l.ListLabels(t.Context()); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListLabels unauthenticated = %v", err)
	}
}
