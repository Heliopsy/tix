// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// statsStatuses is the vocabulary a statistics read counts by. The store is
// given it rather than deriving it, because which statuses exist is a workflow
// question.
var statsStatuses = []string{"todo", "doing", "done"}

// finishTask marks a task terminal at an instant and records the transition
// that did it, which is the only place the actor behind a completion is kept.
func (f fixture) finishTask(t *testing.T, task core.Task, at time.Time, actorID string) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		live, err := tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		live.Status = "done"
		live.CompletedAt = &at
		if err := tx.UpdateTask(ctx, live); err != nil {
			return err
		}
		return tx.AppendAudit(ctx, &core.AuditEntry{
			ActorID: actorID, Action: "task.transition", SubjectType: "task",
			SubjectID: task.ID, Source: core.SourceCLI, OccurredAt: at,
		})
	}); err != nil {
		t.Fatalf("finishing %q: %v", task.Title, err)
	}
}

func (f fixture) softDelete(t *testing.T, task core.Task) {
	t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		return tx.DeleteTask(ctx, task.ID, false)
	}); err != nil {
		t.Fatalf("deleting %q: %v", task.Title, err)
	}
}

func (f fixture) taskStats(t *testing.T, q store.StatsQuery) *store.StatsRows {
	t.Helper()
	ctx := context.Background()
	var rows *store.StatsRows
	if err := f.store.View(ctx, f.scope, func(tx store.Tx) error {
		got, err := tx.TaskStats(ctx, q)
		rows = got
		return err
	}); err != nil {
		t.Fatalf("TaskStats: %v", err)
	}
	return rows
}

// seedStats lays down four tasks: two finished inside the window, one left
// open, and one finished but soft deleted.
func seedStats(t *testing.T, f fixture, base time.Time) store.StatsQuery {
	t.Helper()
	done := f.newTask(t, "done early", core.PriorityNormal)
	late := f.newTask(t, "done late", core.PriorityNormal)
	f.newTask(t, "still open", core.PriorityNormal)
	gone := f.newTask(t, "deleted", core.PriorityNormal)

	f.finishTask(t, done, base.Add(24*time.Hour), f.actor.ID)
	f.finishTask(t, late, base.Add(96*time.Hour), f.actor.ID)
	f.finishTask(t, gone, base.Add(48*time.Hour), f.actor.ID)
	f.softDelete(t, gone)

	return store.StatsQuery{
		Since: base, Until: base.Add(120 * time.Hour),
		Statuses: statsStatuses, Oldest: 5,
	}
}

func TestTaskStatsCountsTheWindowAndExcludesDeleted(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	base := clk.Now()
	q := seedStats(t, f, base)

	rows := f.taskStats(t, q)

	if len(rows.Completed) != 2 {
		t.Fatalf("completed = %+v, want the two live finished tasks", rows.Completed)
	}
	if !rows.Completed[0].CompletedAt.Equal(base.Add(24 * time.Hour)) {
		t.Errorf("completed[0] at %s, want the earliest first", rows.Completed[0].CompletedAt)
	}
	for _, row := range rows.Completed {
		if row.ActorID != f.actor.ID {
			t.Errorf("completion %q names actor %q, want %q", row.TaskID, row.ActorID, f.actor.ID)
		}
		if row.ProjectID != f.project.ID {
			t.Errorf("completion %q names project %q, want %q", row.TaskID, row.ProjectID, f.project.ID)
		}
		if row.CreatedAt.IsZero() {
			t.Errorf("completion %q has no creation time, so no lead time can be derived", row.TaskID)
		}
	}
	if rows.Created != 3 {
		t.Errorf("created = %d, want 3; the soft deleted task is still counted", rows.Created)
	}
	if rows.StatusCounts["done"] != 2 || rows.StatusCounts["todo"] != 1 {
		t.Errorf("status counts = %v, want two done and one todo", rows.StatusCounts)
	}
	if len(rows.Oldest) != 1 || rows.Oldest[0].Title != "still open" {
		t.Errorf("oldest = %+v, want only the open task", rows.Oldest)
	}
	if rows.Oldest[0].Ref == "" {
		t.Error("an ageing task was returned without its reference")
	}
}

func TestTaskStatsExcludesWorkOutsideTheWindow(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	base := clk.Now()
	q := seedStats(t, f, base)
	q.Since = base.Add(48 * time.Hour)

	rows := f.taskStats(t, q)

	if len(rows.Completed) != 1 {
		t.Fatalf("completed = %+v, want only the task finished inside the window", rows.Completed)
	}
	if !rows.Completed[0].CompletedAt.Equal(base.Add(96 * time.Hour)) {
		t.Errorf("completed at %s, want the later finish", rows.Completed[0].CompletedAt)
	}
	if rows.Created != 0 {
		t.Errorf("created = %d, want 0; every task predates the window", rows.Created)
	}
}

func TestTaskStatsNarrowsToOneProject(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	base := clk.Now()
	q := seedStats(t, f, base)
	q.ProjectID = "nosuchproject"

	rows := f.taskStats(t, q)

	if len(rows.Completed) != 0 || rows.Created != 0 || len(rows.Oldest) != 0 {
		t.Errorf("a project with nothing in it reported %+v", rows)
	}
	for status, n := range rows.StatusCounts {
		if n != 0 {
			t.Errorf("status %q = %d, want 0", status, n)
		}
	}
}

func TestTaskStatsAnEmptyWindowIsNotAnError(t *testing.T) {
	s, clk := newStore(t)
	f := seed(t, s, clk, "acme")
	base := clk.Now()

	rows := f.taskStats(t, store.StatsQuery{
		Since: base, Until: base.Add(time.Hour), Statuses: statsStatuses, Oldest: 5,
	})

	if len(rows.Completed) != 0 || rows.Created != 0 || len(rows.Oldest) != 0 {
		t.Errorf("an empty tenant reported %+v", rows)
	}
	if rows.StatusCounts == nil {
		t.Error("status counts are nil, which a caller cannot tell from a failure")
	}
}

// Two tenants seeded to look alike must not see each other's rows in any field.
func TestTaskStatsNeverCrossesATenant(t *testing.T) {
	s, clk := newStore(t)
	mine := seed(t, s, clk, "acme")
	theirs := seed(t, s, clk, "other")
	base := clk.Now()

	q := seedStats(t, mine, base)
	seedStats(t, theirs, base)
	theirs.newTask(t, "extra", core.PriorityNormal)

	rows := mine.taskStats(t, q)

	if len(rows.Completed) != 2 {
		t.Fatalf("completed = %d, want 2; the other tenant leaked in", len(rows.Completed))
	}
	for _, row := range rows.Completed {
		if row.ProjectID != mine.project.ID {
			t.Errorf("completion names project %q, which is not this tenant's", row.ProjectID)
		}
		if row.ActorID != mine.actor.ID {
			t.Errorf("completion names actor %q, which is not this tenant's", row.ActorID)
		}
	}
	if rows.Created != 3 {
		t.Errorf("created = %d, want 3", rows.Created)
	}
	if rows.StatusCounts["done"] != 2 || rows.StatusCounts["todo"] != 1 {
		t.Errorf("status counts = %v, want this tenant's alone", rows.StatusCounts)
	}
	if len(rows.Oldest) != 1 {
		t.Errorf("oldest = %+v, want this tenant's one open task", rows.Oldest)
	}
	for _, task := range rows.Oldest {
		if task.TenantID != mine.tenant.ID {
			t.Errorf("ageing task belongs to tenant %q, want %q", task.TenantID, mine.tenant.ID)
		}
	}

	q.Statuses = statsStatuses
	theirRows := theirs.taskStats(t, store.StatsQuery{
		Since: base, Until: base.Add(120 * time.Hour), Statuses: statsStatuses, Oldest: 5,
	})
	if len(theirRows.Oldest) != 2 || theirRows.Created != 4 {
		t.Errorf("the other tenant reads back wrong: oldest %d, created %d",
			len(theirRows.Oldest), theirRows.Created)
	}
}
