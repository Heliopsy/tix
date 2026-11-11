// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

func TestMoveKeepsTheTaskAndRenumbersIt(t *testing.T) {
	l, ctx, scope, me, _ := newTaskFixture(t)
	ops := seedTaskProject(t, l, scope, "ops")
	infraID := projectIDOf(t, l, scope, "infra")
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: infraID, Key: "env", Label: "Env", Type: core.FieldString})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: infraID, Key: "team", Label: "Team", Type: core.FieldString})
	seedFieldDef(t, l, scope, core.FieldDef{ProjectID: ops.ID, Key: "team", Label: "Team", Type: core.FieldString})
	mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "ops", Title: "already there"})
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{
		ProjectRef: "infra", Title: "rotate keys", Status: "doing", Tags: []string{"sec"},
		AssigneeActorID: me.ID, CustomFields: map[string]any{"env": "prod", "team": "core"},
	})
	if _, err := l.AddComment(ctx, core.TaskRef{ID: src.ID}, "note"); err != nil {
		t.Fatal(err)
	}

	moved, err := l.MoveTask(ctx, core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: "ops"})
	if err != nil {
		t.Fatalf("moving: %v", err)
	}
	got := moved.Task
	if got.ID != src.ID || got.Ref != "ops-2" || got.ProjectID != ops.ID {
		t.Fatalf("moved = %s %s in %s, want the same id as ops-2", got.ID, got.Ref, got.ProjectID)
	}
	if moved.FromRef != src.Ref || moved.FromProject != "infra" || moved.Project != "ops" {
		t.Errorf("result = %+v, want from %s infra to ops", moved, src.Ref)
	}
	if got.Status != "doing" || !moved.StatusKept || moved.FromStatus != "doing" {
		t.Errorf("status = %q kept=%v, want doing kept", got.Status, moved.StatusKept)
	}
	if got.AssigneeActorID != me.ID || !slices.Equal(got.Tags, []string{"sec"}) {
		t.Errorf("assignee %q tags %v did not travel", got.AssigneeActorID, got.Tags)
	}
	if got.CustomFields["team"] != "core" || got.CustomFields["env"] != nil {
		t.Errorf("fields = %v, want team kept and env dropped", got.CustomFields)
	}
	if !slices.Equal(moved.DroppedFields, []string{"env"}) {
		t.Errorf("dropped = %v, want [env]", moved.DroppedFields)
	}
	if comments := commentsOf(t, l, ctx, &got); len(comments) != 1 {
		t.Errorf("comments = %d, want the one that was there", len(comments))
	}
	old, _ := core.ParseTaskRef(src.Ref)
	again, err := l.GetTask(ctx, old)
	if err != nil || again.ID != src.ID || again.Ref != "ops-2" {
		t.Fatalf("old reference %s = %v, %v; want the moved task", src.Ref, again, err)
	}
	next := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "after"})
	if next.Ref == src.Ref {
		t.Fatalf("infra reissued %s", src.Ref)
	}
}

func TestMoveRecordsOneAuditEntryAndAnEventInEachProject(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	ops := seedTaskProject(t, l, scope, "ops")
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	events, audits := countRows(t, l, scope)
	if _, err := l.MoveTask(ctx, core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: "ops"}); err != nil {
		t.Fatal(err)
	}
	afterEvents, afterAudits := countRows(t, l, scope)
	if afterEvents-events != 2 || afterAudits-audits != 1 {
		t.Fatalf("events +%d audits +%d, want +2 and +1", afterEvents-events, afterAudits-audits)
	}
	if n := countOf(auditActions(t, l, scope), "task.move"); n != 1 {
		t.Errorf("task.move entries = %d, want 1", n)
	}
	projects := map[string]bool{}
	if err := l.store.View(context.Background(), scope, func(tx store.Tx) error {
		evs, err := tx.ReadEvents(context.Background(), 0, 1000)
		for _, e := range evs {
			if e.SubjectID == src.ID && e.Payload["from_ref"] == src.Ref {
				projects[e.ProjectID] = true
			}
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if !projects[ops.ID] || !projects[src.ProjectID] || len(projects) != 2 {
		t.Errorf("move events reached %v, want both projects", projects)
	}
}

func TestMoveStatusFallsBackToTheTargetsInitialState(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProjectWith(t, l, scope, "ship", shippingWorkflow())
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x", Status: "done"})
	moved, err := l.MoveTask(ctx, core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: "ship"})
	if err != nil {
		t.Fatal(err)
	}
	if moved.Task.Status != "backlog" || moved.StatusKept || moved.Task.CompletedAt != nil {
		t.Fatalf("status = %q kept=%v completed=%v, want backlog, not kept, not completed",
			moved.Task.Status, moved.StatusKept, moved.Task.CompletedAt != nil)
	}
}

func TestMoveRefusals(t *testing.T) {
	l, ctx, scope, _, infra := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	seedTaskProject(t, l, scope, "old")
	if err := l.ArchiveProject(ctx, "old"); err != nil {
		t.Fatal(err)
	}
	src := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "x"})
	confined := seedTaskActor(t, l, scope, "pinned", core.RoleMember, core.ScopeAll)
	confined.ProjectID = infra.ID
	viewer := seedTaskActor(t, l, scope, "vee", core.RoleViewer, core.ScopeTaskRead)

	cases := []struct {
		name    string
		ctx     context.Context
		project string
		want    core.Kind
	}{
		{"no target", ctx, " ", core.KindInvalid},
		{"its own project", ctx, "infra", core.KindInvalid},
		{"an unknown project", ctx, "nope", core.KindNotFound},
		{"an archived project", ctx, "old", core.KindPrecondition},
		{"read only", taskContext(viewer), "ops", core.KindForbidden},
		{"confined to its project", taskContext(confined), "ops", core.KindForbidden},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			events, audits := countRows(t, l, scope)
			_, err := l.MoveTask(tc.ctx, core.TaskRef{ID: src.ID}, core.MoveTaskInput{ProjectRef: tc.project})
			if !core.IsKind(err, tc.want) {
				t.Fatalf("err = %v, want %s", err, tc.want)
			}
			if e, a := countRows(t, l, scope); e != events || a != audits {
				t.Fatalf("a refused move wrote rows")
			}
		})
	}
}

func TestBulkMoveProjectNamesEachNewReference(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	a := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "a"})
	result, err := l.BulkTasks(ctx, core.BulkTaskInput{
		Action: core.BulkMoveProject, ProjectRef: "ops", Refs: []string{a.Ref, "infra-999"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Applied != 1 || result.Refused != 1 || result.Outcomes[0].MovedRef != "ops-1" {
		t.Fatalf("result = %+v, want a moved to ops-1 and the unknown one refused", result)
	}
}

func TestMoveRefusesATaskSomebodyElseHolds(t *testing.T) {
	l, ctx, scope, _, _ := newTaskFixture(t)
	seedTaskProject(t, l, scope, "ops")
	worker := seedTaskActor(t, l, scope, "worker", core.RoleMember, core.ScopeAll)
	held := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "held"})
	mine := mustCreateTask(t, l, ctx, core.CreateTaskInput{ProjectRef: "infra", Title: "mine"})
	if _, err := l.ClaimTask(taskContext(worker), core.TaskRef{ID: held.ID}, core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
	if _, err := l.ClaimTask(ctx, core.TaskRef{ID: mine.ID}, core.ClaimInput{}); err != nil {
		t.Fatal(err)
	}
	_, err := l.MoveTask(ctx, core.TaskRef{ID: held.ID}, core.MoveTaskInput{ProjectRef: "ops"})
	if !core.IsKind(err, core.KindConflict) || !strings.Contains(err.Error(), "claimed") {
		t.Fatalf("err = %v, want a conflict naming the claim", err)
	}
	moved, err := l.MoveTask(ctx, core.TaskRef{ID: mine.ID}, core.MoveTaskInput{ProjectRef: "ops"})
	if err != nil {
		t.Fatalf("moving my own claimed task: %v", err)
	}
	if moved.Task.ClaimedByActorID == "" {
		t.Errorf("my claim did not move with the task")
	}
}
