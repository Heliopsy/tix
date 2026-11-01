// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func TestCheckTaskItemRewritesOneMarkerAndRecordsIt(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "list",
		Body: "- [ ] one\n- [ ] two\n"})
	ref := core.TaskRef{ID: task.ID}

	got, err := l.CheckTaskItem(ctx, ref, core.CheckItemInput{Item: 2, Done: true, Version: task.Version})
	if err != nil {
		t.Fatalf("ticking: %v", err)
	}
	if got.Body != "- [ ] one\n- [x] two\n" || got.Version != task.Version+1 {
		t.Fatalf("after ticking: body %q version %d", got.Body, got.Version)
	}
	if got.Checklist == nil || *got.Checklist != (core.Checklist{Done: 1, Total: 2}) {
		t.Fatalf("checklist = %+v, want 1/2", got.Checklist)
	}
	entries, _, err := l.ListAudit(ctx, core.AuditFilter{SubjectType: "task", SubjectID: task.ID,
		Actions: []string{"task.update"}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("audit entries = %d (%v), want the one tick", len(entries), err)
	}

	again, err := l.CheckTaskItem(ctx, ref, core.CheckItemInput{Item: 2, Done: true})
	if err != nil || again.Version != got.Version {
		t.Fatalf("ticking a ticked item wrote: version %d -> %d (%v)", got.Version, again.Version, err)
	}

	_, err = l.CheckTaskItem(ctx, ref, core.CheckItemInput{Item: 1, Done: true, Version: task.Version})
	if !core.IsKind(err, core.KindConflict) {
		t.Fatalf("a stale version = %v, want a conflict", err)
	}
	_, err = l.CheckTaskItem(ctx, ref, core.CheckItemInput{Item: 3, Done: true})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("an item past the end = %v, want invalid", err)
	}

	reader := taskContext(seedTaskActor(t, l, scope, "reader", core.RoleViewer, core.RoleViewer.Scopes()...))
	_, err = l.CheckTaskItem(reader, ref, core.CheckItemInput{Item: 1, Done: true})
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("a reader without task:write = %v, want permission denied", err)
	}

	read, err := l.GetTask(ctx, ref)
	if err != nil || read.Body != got.Body {
		t.Fatalf("refused ticks changed the text: %q (%v)", read.Body, err)
	}
}
