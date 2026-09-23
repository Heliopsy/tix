// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestLabelsAttachDetachAndList(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "tagged"})
	ref := core.TaskRef{ID: task.ID}

	beforeEvents, beforeAudits := countRows(t, l, scope)
	if err := l.AddTag(ctx, ref, "urgent"); err != nil {
		t.Fatalf("AddTag: %v", err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents != beforeEvents+1 || afterAudits != beforeAudits+1 {
		t.Errorf("adding a tag wrote %d events and %d audit entries, want one of each",
			afterEvents-beforeEvents, afterAudits-beforeAudits)
	}

	if err := l.AddTag(ctx, ref, "urgent"); err != nil {
		t.Fatalf("attaching the same tag twice: %v", err)
	}
	got, err := l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if strings.Join(got.Tags, ",") != "urgent" {
		t.Errorf("tags = %v, want exactly one", got.Tags)
	}

	page, err := l.ListTasks(ctx, core.TaskFilter{Tags: []string{"urgent"}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 1 {
		t.Errorf("filtering by tag returned %d tasks, want one", len(page.Tasks))
	}

	tags, err := l.ListTags(ctx)
	if err != nil {
		t.Fatalf("ListTags: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "urgent" {
		t.Errorf("tags = %+v, want the one tag", tags)
	}

	if err := l.RemoveTag(ctx, ref, "urgent"); err != nil {
		t.Fatalf("RemoveTag: %v", err)
	}
	got, err = l.GetTask(ctx, ref)
	if err != nil {
		t.Fatalf("GetTask: %v", err)
	}
	if len(got.Tags) != 0 {
		t.Errorf("tags = %v, want none", got.Tags)
	}
	page, err = l.ListTasks(ctx, core.TaskFilter{Tags: []string{"urgent"}})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	if len(page.Tasks) != 0 {
		t.Errorf("a detached tag still matches the filter: %v", taskTitles(page.Tasks))
	}
}

func TestLabelOperationsRejectBadInput(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{Title: "t"})
	ref := core.TaskRef{ID: task.ID}

	if err := l.AddTag(ctx, ref, "   "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("AddTag with an empty name = %v, want invalid", err)
	}
	if err := l.RemoveTag(ctx, ref, " "); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("RemoveTag with an empty name = %v, want invalid", err)
	}
	if err := l.RemoveTag(ctx, ref, "never-used"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("RemoveTag of an unknown tag = %v, want not found", err)
	}
	if err := l.AddTag(ctx, core.MustParseTaskRef("infra-99"), "x"); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("AddTag on a missing task = %v, want not found", err)
	}
	if err := l.AddTag(t.Context(), ref, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("AddTag unauthenticated = %v", err)
	}
	if err := l.RemoveTag(t.Context(), ref, "x"); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("RemoveTag unauthenticated = %v", err)
	}
	if _, err := l.ListTags(t.Context()); !core.IsKind(err, core.KindUnauthenticated) {
		t.Errorf("ListTags unauthenticated = %v", err)
	}
}
