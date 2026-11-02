// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// cleanupWorkflow has one done-category and one cancelled-category terminal.
func cleanupWorkflow() core.WorkflowDefinition {
	return core.WorkflowDefinition{
		Initial: "todo",
		States: []core.State{
			{Key: "todo", Label: "Todo", Category: core.CategoryTodo},
			{Key: "done", Label: "Done", Terminal: true, Category: core.CategoryDone},
			{Key: "dropped", Label: "Dropped", Terminal: true, Category: core.CategoryCancelled},
		},
		Transitions: []core.Transition{{From: "todo", To: "done"}, {From: "todo", To: "dropped"}},
	}
}

type cleanupFixture struct {
	l   *Local
	clk *clock.Fake
	ctx context.Context
	sys context.Context
}

func newCleanupFixture(t *testing.T, keys ...string) *cleanupFixture {
	t.Helper()
	l, clk, scope, actor := newLocal(t)
	for _, k := range keys {
		seedTaskProjectWith(t, l, scope, k, cleanupWorkflow())
	}
	sys := core.WithSource(core.WithActor(context.Background(), core.SystemActor(scope.TenantID)), core.SourceSystem)
	return &cleanupFixture{l: l, clk: clk, ctx: taskContext(actor), sys: sys}
}

// finished creates a task in project and moves it to status.
func (f *cleanupFixture) finished(t *testing.T, project, status string) *core.Task {
	t.Helper()
	task := mustCreateTask(t, f.l, f.ctx, core.CreateTaskInput{Title: status + " work", ProjectRef: project})
	if status == "todo" {
		return task
	}
	moved, err := f.l.TransitionTask(f.ctx, core.TaskRef{ID: task.ID}, core.TransitionInput{To: status})
	if err != nil {
		t.Fatalf("moving to %s: %v", status, err)
	}
	return moved
}

func (f *cleanupFixture) put(t *testing.T, in core.HousekeepingInput) *core.HousekeepingView {
	t.Helper()
	v, err := f.l.PutHousekeeping(f.ctx, in)
	if err != nil {
		t.Fatalf("PutHousekeeping(%+v): %v", in, err)
	}
	return v
}

func (f *cleanupFixture) run(t *testing.T, limit int) *core.HousekeepingResult {
	t.Helper()
	res, err := f.l.RunHousekeeping(f.sys, core.HousekeepingRunInput{Limit: limit})
	if err != nil {
		t.Fatalf("RunHousekeeping: %v", err)
	}
	return res
}

// state reports whether a task is live, trashed or gone.
func (f *cleanupFixture) state(t *testing.T, id string) string {
	t.Helper()
	page, err := f.l.ListTasks(f.ctx, core.TaskFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListTasks: %v", err)
	}
	for _, got := range page.Tasks {
		if got.ID == id {
			if got.Deleted() {
				return "trashed"
			}
			return "live"
		}
	}
	return "gone"
}

func (f *cleanupFixture) days(n int) { f.clk.Advance(time.Duration(n) * 24 * time.Hour) }

func TestHousekeepingIsOffByDefault(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	task := f.finished(t, "ops", "done")
	f.days(400)
	if res := f.run(t, 0); res.Total() != 0 {
		t.Errorf("a tenant that set nothing cleaned up %+v", res)
	}
	if got := f.state(t, task.ID); got != "live" {
		t.Errorf("task is %s, want live", got)
	}
	v, err := f.l.GetHousekeeping(f.ctx, "")
	if err != nil {
		t.Fatalf("GetHousekeeping: %v", err)
	}
	if v.Effective.Days(core.RuleDone) != 0 || v.Pending.Total() != 0 || !v.Tenant.Empty() {
		t.Errorf("default view = %+v, want everything off", v)
	}
}

func TestHousekeepingMovesFinishedTasksToTheTrash(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	done := f.finished(t, "ops", "done")
	dropped := f.finished(t, "ops", "dropped")
	open := f.finished(t, "ops", "todo")
	f.days(10)
	fresh := f.finished(t, "ops", "done")
	f.put(t, core.HousekeepingInput{Done: "7", Cancelled: "30d"})

	preview, err := f.l.RunHousekeeping(f.ctx, core.HousekeepingRunInput{DryRun: true})
	if err != nil {
		t.Fatalf("preview: %v", err)
	}
	if preview.Done != 1 || preview.Cancelled != 0 || f.state(t, done.ID) != "live" {
		t.Fatalf("preview = %+v and wrote %s, want one done task and no change", preview, f.state(t, done.ID))
	}

	events, audits := countRows(t, f.l, core.TenantScope{TenantID: done.TenantID})
	if res := f.run(t, 0); res.Done != 1 || res.Cancelled != 0 {
		t.Fatalf("run = %+v, want one done task", res)
	}
	for id, want := range map[string]string{done.ID: "trashed", dropped.ID: "live", open.ID: "live", fresh.ID: "live"} {
		if got := f.state(t, id); got != want {
			t.Errorf("task %s is %s, want %s", id, got, want)
		}
	}
	e2, a2 := countRows(t, f.l, core.TenantScope{TenantID: done.TenantID})
	if e2-events != 1 || a2-audits != 1 {
		t.Errorf("one cleanup wrote %d events and %d audit entries, want one of each", e2-events, a2-audits)
	}
	entries, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{SubjectID: done.ID, Actions: []string{"task.delete"}})
	if err != nil || len(entries) != 1 {
		t.Fatalf("audit of the cleanup = %v, %v", entries, err)
	}
	if entries[0].ActorID != core.SystemActor(done.TenantID).ID || entries[0].Source != core.SourceSystem {
		t.Errorf("cleanup attributed to %q from %q, want the system actor", entries[0].ActorID, entries[0].Source)
	}

	f.days(25)
	if res := f.run(t, 0); res.Cancelled != 1 || res.Done != 1 {
		t.Errorf("second run = %+v, want the dropped task and the fresh done one", res)
	}
	if res := f.run(t, 0); res.Total() != 0 {
		t.Errorf("a repeat run acted again: %+v", res)
	}
}

func TestHousekeepingLeavesAFinishedParentWithOpenChildren(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	parent := f.finished(t, "ops", "done")
	child := mustCreateTask(t, f.l, f.ctx, core.CreateTaskInput{Title: "child", ProjectRef: "ops", ParentRef: parent.Ref})
	f.put(t, core.HousekeepingInput{Done: "1"})
	f.days(5)
	f.run(t, 0)
	if got := f.state(t, parent.ID); got != "live" {
		t.Fatalf("a done parent with an open child is %s, want live", got)
	}
	if _, err := f.l.TransitionTask(f.ctx, core.TaskRef{ID: child.ID}, core.TransitionInput{To: "done"}); err != nil {
		t.Fatal(err)
	}
	f.days(5)
	f.run(t, 0)
	f.run(t, 0)
	if f.state(t, parent.ID) != "trashed" || f.state(t, child.ID) != "trashed" {
		t.Errorf("parent %s child %s, want both trashed once both are done", f.state(t, parent.ID), f.state(t, child.ID))
	}
}

func TestHousekeepingProjectOverridesTheTenant(t *testing.T) {
	f := newCleanupFixture(t, "ops", "web")
	ops := f.finished(t, "ops", "done")
	web := f.finished(t, "web", "done")
	f.put(t, core.HousekeepingInput{Done: "30"})
	v := f.put(t, core.HousekeepingInput{Project: "ops", Done: "off"})
	if v.Project != "ops" || v.Effective.Days(core.RuleDone) != 0 || v.Tenant.Days(core.RuleDone) != 30 {
		t.Fatalf("override view = %+v", v)
	}
	f.days(40)
	f.run(t, 0)
	if f.state(t, ops.ID) != "live" || f.state(t, web.ID) != "trashed" {
		t.Errorf("ops %s web %s, want ops kept by its override", f.state(t, ops.ID), f.state(t, web.ID))
	}

	v = f.put(t, core.HousekeepingInput{Project: "ops", Done: "inherit"})
	if v.Override == nil || v.Override.Done != nil || v.Pending.Done != 1 {
		t.Errorf("inheriting again = %+v, want the tenant's rule and one pending", v)
	}
	if _, err := f.l.PutHousekeeping(f.ctx, core.HousekeepingInput{Done: "inherit"}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("the tenant inheriting = %v, want invalid", err)
	}
	if _, err := f.l.PutHousekeeping(f.ctx, core.HousekeepingInput{}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("an empty change = %v, want invalid", err)
	}
}

func TestHousekeepingNeverDeletesFinishedTasksForGood(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	setTrash(t, f.l, f.ctx, false)
	if _, err := f.l.PutHousekeeping(f.ctx, core.HousekeepingInput{Done: "30"}); !isTrashRefusal(err) {
		t.Fatalf("turning on the done rule with the trash off = %v, want the trash refusal", err)
	}
	if _, err := f.l.PutHousekeeping(f.ctx, core.HousekeepingInput{Project: "ops", Cancelled: "30"}); !isTrashRefusal(err) {
		t.Fatalf("a project override with the trash off = %v, want the trash refusal", err)
	}
	f.put(t, core.HousekeepingInput{Done: "off", Trash: "30"})

	setTrash(t, f.l, f.ctx, true)
	task := f.finished(t, "ops", "done")
	f.put(t, core.HousekeepingInput{Done: "1"})
	setTrash(t, f.l, f.ctx, false)
	f.days(5)
	res := f.run(t, 0)
	if !res.Paused || res.Total() != 0 || f.state(t, task.ID) != "live" {
		t.Errorf("with the trash turned off later: %+v, task %s; want paused and kept", res, f.state(t, task.ID))
	}
}

func TestHousekeepingEmptiesTheTrash(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	parent := f.finished(t, "ops", "todo")
	child := mustCreateTask(t, f.l, f.ctx, core.CreateTaskInput{Title: "child", ProjectRef: "ops", ParentRef: parent.Ref})
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: parent.ID}, core.DeleteTaskInput{Cascade: true}); err != nil {
		t.Fatal(err)
	}
	f.days(40)
	recent := f.finished(t, "ops", "todo")
	if err := f.l.DeleteTask(f.ctx, core.TaskRef{ID: recent.ID}, core.DeleteTaskInput{}); err != nil {
		t.Fatal(err)
	}
	f.put(t, core.HousekeepingInput{Trash: "30"})

	if res := f.run(t, 0); res.Trash != 2 {
		t.Errorf("run = %+v, want the parent and its child purged", res)
	}
	for id, want := range map[string]string{parent.ID: "gone", child.ID: "gone", recent.ID: "trashed"} {
		if got := f.state(t, id); got != want {
			t.Errorf("task %s is %s, want %s", id, got, want)
		}
	}
	entries, _, err := f.l.ListAudit(f.ctx, core.AuditFilter{SubjectID: child.ID, Actions: []string{"task.delete"}})
	if err != nil || len(entries) != 2 {
		t.Errorf("the child's deletes = %d entries (%v), want the cascade and the purge", len(entries), err)
	}
}

func TestHousekeepingStopsAtTheLimit(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	for range 3 {
		f.finished(t, "ops", "done")
	}
	f.put(t, core.HousekeepingInput{Done: "1"})
	f.days(2)
	if res := f.run(t, 2); res.Done != 2 {
		t.Errorf("first run = %+v, want the limit of 2", res)
	}
	if res := f.run(t, 2); res.Done != 1 {
		t.Errorf("second run = %+v, want the one left", res)
	}
	if _, err := f.l.RunHousekeeping(f.sys, core.HousekeepingRunInput{Limit: -1}); !core.IsKind(err, core.KindInvalid) {
		t.Errorf("a negative limit = %v, want invalid", err)
	}
}

func TestHousekeepingNeedsATenantAdmin(t *testing.T) {
	f := newCleanupFixture(t, "ops")
	admin, _ := core.ActorFrom(f.ctx)
	member := seedTaskActor(t, f.l, core.TenantScope{TenantID: admin.TenantID}, "bob", core.RoleMember,
		core.ScopeTaskRead, core.ScopeTaskWrite, core.ScopeTaskDelete, core.ScopeProjectWrite)
	ctx := taskContext(member)
	if _, err := f.l.GetHousekeeping(ctx, ""); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a member reading = %v, want forbidden", err)
	}
	if _, err := f.l.PutHousekeeping(ctx, core.HousekeepingInput{Done: "1"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a member setting = %v, want forbidden", err)
	}
}
