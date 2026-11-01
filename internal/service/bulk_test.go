// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

func mustBulk(t *testing.T, l *Local, ctx context.Context, in core.BulkTaskInput) *core.BulkResult {
	t.Helper()
	result, err := l.BulkTasks(ctx, in)
	if err != nil {
		t.Fatalf("bulk %s: %v", in.Action, err)
	}
	return result
}

func TestBulkMoveReportsEachTask(t *testing.T) {
	l, ctx, _, _, _ := newTaskFixture(t)
	open := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "open"})
	stuck := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "stuck"})
	if _, err := l.TransitionTask(ctx, core.TaskRef{ID: stuck.ID}, core.TransitionInput{To: "cancelled"}); err != nil {
		t.Fatal(err)
	}

	result := mustBulk(t, l, ctx, core.BulkTaskInput{
		Action: core.BulkMove, To: "done", Refs: []string{open.Ref, stuck.Ref, "infra-99", open.Ref},
	})

	if result.Applied != 1 || result.Refused != 2 || len(result.Outcomes) != 3 {
		t.Fatalf("result = %+v, want one applied and two refused over three distinct refs", result)
	}
	want := []struct {
		ref     string
		applied bool
		code    core.Kind
	}{
		{open.Ref, true, ""},
		{stuck.Ref, false, core.KindPrecondition},
		{"infra-99", false, core.KindNotFound},
	}
	for i, w := range want {
		o := result.Outcomes[i]
		if o.Ref != w.ref || o.Applied != w.applied || core.Kind(o.Code) != w.code {
			t.Errorf("outcome %d = %+v, want ref %s applied %v code %q", i, o, w.ref, w.applied, w.code)
		}
		if !o.Applied && o.Error == "" {
			t.Errorf("outcome %d is refused without a reason", i)
		}
	}
	for ref, status := range map[string]string{open.Ref: "done", stuck.Ref: "cancelled"} {
		got, err := l.GetTask(ctx, mustRef(t, ref))
		if err != nil || got.Status != status {
			t.Errorf("%s status = %v (%v), want %s", ref, got, err, status)
		}
	}
}

func mustRef(t *testing.T, s string) core.TaskRef {
	t.Helper()
	ref, err := core.ParseTaskRef(s)
	if err != nil {
		t.Fatal(err)
	}
	return ref
}

func TestBulkWritesOneAuditEntryAndEventPerTask(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	var refs []string
	for i := range 3 {
		refs = append(refs, mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: fmt.Sprint(i)}).Ref)
	}
	events, audits := countRows(t, l, scope)
	before := countOf(auditActions(t, l, scope), "task.update")

	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkPriority, Priority: core.PriorityHighest, Refs: refs})

	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-events != 3 || afterAudits-audits != 3 {
		t.Fatalf("events +%d audits +%d, want +3 each", afterEvents-events, afterAudits-audits)
	}
	if n := countOf(auditActions(t, l, scope), "task.update") - before; n != 3 {
		t.Fatalf("task.update entries +%d, want 3", n)
	}
	for _, ref := range refs {
		got, _ := l.GetTask(ctx, mustRef(t, ref))
		if got.Priority != core.PriorityHighest {
			t.Errorf("%s priority = %d", ref, got.Priority)
		}
	}
}

func TestBulkActionsApplyTheSingleTaskOperation(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "a"})
	b := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "b"})
	refs := []string{a.Ref, b.Ref}
	read := func(ref string) *core.Task {
		t.Helper()
		got, err := l.GetTask(ctx, mustRef(t, ref))
		if err != nil {
			t.Fatal(err)
		}
		return got
	}

	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkTagAdd, Tag: "ops", Refs: refs})
	if !slices.Contains(read(a.Ref).Tags, "ops") || !slices.Contains(read(b.Ref).Tags, "ops") {
		t.Error("tag_add left a task untagged")
	}
	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkTagRemove, Tag: "ops", Refs: refs[:1]})
	if slices.Contains(read(a.Ref).Tags, "ops") || !slices.Contains(read(b.Ref).Tags, "ops") {
		t.Error("tag_remove reached the wrong tasks")
	}

	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkAssign, AssigneeActorID: me.ID, Refs: refs})
	if read(a.Ref).AssigneeActorID != me.ID {
		t.Error("assign did not assign")
	}
	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkAssign, Refs: refs})
	if read(a.Ref).AssigneeActorID != "" {
		t.Error("assign with no actor did not clear")
	}

	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkStar, Refs: refs})
	if !read(b.Ref).Starred {
		t.Error("star did not star")
	}
	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkUnstar, Refs: refs})
	if read(b.Ref).Starred {
		t.Error("unstar did not unstar")
	}

	copied := mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkCopy, ProjectRef: "ops", Refs: refs})
	for _, o := range copied.Outcomes {
		c := read(o.CopyRef)
		if !strings.HasPrefix(c.Ref, "ops-") || c.CopiedFromID == "" {
			t.Errorf("copy of %s = %s copied from %q, want an ops task linked to it", o.Ref, c.Ref, c.CopiedFromID)
		}
	}

	mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkDelete, Refs: refs})
	if _, err := l.GetTask(ctx, mustRef(t, a.Ref)); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("deleted task still reads: %v", err)
	}
	restored := mustBulk(t, l, ctx, core.BulkTaskInput{Action: core.BulkRestore, Refs: refs})
	if restored.Applied != 2 || read(a.Ref).DeletedAt != nil {
		t.Errorf("restore = %+v", restored)
	}
}

func TestBulkInputIsCheckedBeforeAnyTaskChanges(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	tooMany := make([]string, core.MaxBulkRefs+1)
	for i := range tooMany {
		tooMany[i] = fmt.Sprintf("infra-%d", i+1)
	}
	tooMany[0] = task.Ref
	cases := []struct {
		name  string
		in    core.BulkTaskInput
		field string
	}{
		{"unknown action", core.BulkTaskInput{Action: "explode", Refs: []string{task.Ref}}, "action"},
		{"no refs", core.BulkTaskInput{Action: core.BulkStar}, "refs"},
		{"too many refs", core.BulkTaskInput{Action: core.BulkPriority, Priority: core.PriorityLow, Refs: tooMany}, "refs"},
		{"move without a status", core.BulkTaskInput{Action: core.BulkMove, Refs: []string{task.Ref}}, "to"},
		{"priority out of range", core.BulkTaskInput{Action: core.BulkPriority, Priority: 9, Refs: []string{task.Ref}}, "priority"},
		{"tag without a name", core.BulkTaskInput{Action: core.BulkTagAdd, Tag: " ", Refs: []string{task.Ref}}, "tag"},
		{"copy without a project", core.BulkTaskInput{Action: core.BulkCopy, Refs: []string{task.Ref}}, "project_ref"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events, audits := countRows(t, l, scope)
			_, err := l.BulkTasks(ctx, tc.in)
			if !core.IsKind(err, core.KindInvalid) || core.FieldOf(err) != tc.field {
				t.Fatalf("err = %v (field %q), want invalid on %q", err, core.FieldOf(err), tc.field)
			}
			if e, a := countRows(t, l, scope); e != events || a != audits {
				t.Fatal("a refused bulk request wrote")
			}
		})
	}
	if got, _ := l.GetTask(ctx, core.TaskRef{ID: task.ID}); got.Priority != core.PriorityNormal {
		t.Errorf("priority = %d after a refused request", got.Priority)
	}
}

func TestBulkRefusesTheWholeRequestWithoutTheScope(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	task := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	viewer := taskContext(seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead))
	cases := []struct {
		in core.BulkTaskInput
		ok bool
	}{
		{core.BulkTaskInput{Action: core.BulkDelete, Refs: []string{task.Ref}}, false},
		{core.BulkTaskInput{Action: core.BulkMove, To: "doing", Refs: []string{task.Ref}}, false},
		{core.BulkTaskInput{Action: core.BulkPriority, Priority: core.PriorityLow, Refs: []string{task.Ref}}, false},
		{core.BulkTaskInput{Action: core.BulkCopy, ProjectRef: "infra", Refs: []string{task.Ref}}, false},
		{core.BulkTaskInput{Action: core.BulkStar, Refs: []string{task.Ref}}, true},
	}
	for _, tc := range cases {
		t.Run(string(tc.in.Action), func(t *testing.T) {
			result, err := l.BulkTasks(viewer, tc.in)
			switch {
			case tc.ok && (err != nil || result.Applied != 1):
				t.Fatalf("result = %+v, err = %v, want applied", result, err)
			case !tc.ok && !core.IsKind(err, core.KindForbidden):
				t.Fatalf("err = %v, want the whole request forbidden", err)
			}
		})
	}
	if got, err := l.GetTask(ctx, core.TaskRef{ID: task.ID}); err != nil || got.Status != "todo" {
		t.Fatalf("task after refused requests: %+v, %v", got, err)
	}
}
