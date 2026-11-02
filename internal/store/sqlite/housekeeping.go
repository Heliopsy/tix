// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// noLiveChild keeps a task whose children are still live out of a selection.
const noLiveChild = "NOT EXISTS (SELECT 1 FROM tasks c WHERE c.tenant_id = tasks.tenant_id " +
	"AND c.parent_id = tasks.id AND c.deleted_at IS NULL)"

// HousekeepingPolicies returns every stored housekeeping level.
func (t *tx) HousekeepingPolicies(ctx context.Context) ([]core.HousekeepingPolicy, error) {
	b := t.builder("housekeeping_policies").
		Select("project_id", "done_after", "cancelled_after", "trash_after").
		OrderBy("project_id", core.Ascending)
	rows, err := t.query(ctx, b, "listing housekeeping policies")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.HousekeepingPolicy{}
	for rows.Next() {
		var p core.HousekeepingPolicy
		var done, cancelled, trash sql.NullInt64
		if err := rows.Scan(&p.ProjectID, &done, &cancelled, &trash); err != nil {
			return nil, mapErr(err, "scanning housekeeping policy")
		}
		p.Done, p.Cancelled, p.Trash = nullDays(done), nullDays(cancelled), nullDays(trash)
		out = append(out, p)
	}
	return out, mapRowsErr(rows, "listing housekeeping policies")
}

// PutHousekeepingPolicy stores one housekeeping level.
func (t *tx) PutHousekeepingPolicy(ctx context.Context, p *core.HousekeepingPolicy) error {
	if p.ProjectID != "" && p.Empty() {
		del := t.builder("housekeeping_policies").Where("project_id = ?", p.ProjectID)
		_, err := t.execDelete(ctx, del, "clearing housekeeping override")
		return err
	}
	upd := t.builder("housekeeping_policies").
		Where("project_id = ?", p.ProjectID).
		Set("done_after", daysArg(p.Done)).
		Set("cancelled_after", daysArg(p.Cancelled)).
		Set("trash_after", daysArg(p.Trash)).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, upd, "updating housekeeping policy")
	if err != nil || n > 0 {
		return err
	}
	ins := t.insert("housekeeping_policies").
		Set("project_id", p.ProjectID).
		Set("done_after", daysArg(p.Done)).
		Set("cancelled_after", daysArg(p.Cancelled)).
		Set("trash_after", daysArg(p.Trash)).
		Set("updated_at", t.now())
	_, err = t.execInsert(ctx, ins, "creating housekeeping policy")
	return err
}

// finished builds the selection of one finished-task rule.
func (t *tx) finished(q store.FinishedQuery) *sqlb.Builder {
	b := t.builder("tasks").
		Where("tasks.project_id = ?", q.ProjectID).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.completed_at IS NOT NULL").
		Where("tasks.completed_at < ?", sqlb.TimeText(q.Before)).
		Where(noLiveChild)
	return b.WhereIn("tasks.status", q.Statuses)
}

// FinishedTasks returns the finished tasks one rule would move, oldest first.
func (t *tx) FinishedTasks(ctx context.Context, q store.FinishedQuery, limit int) ([]core.Task, error) {
	if len(q.Statuses) == 0 {
		return []core.Task{}, nil
	}
	b := t.finished(q).
		Select(taskColumns...).
		Join(projectJoin).
		OrderBy("tasks.completed_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(limit)
	return t.housekeepingTasks(ctx, b, "listing finished tasks")
}

// CountFinishedTasks counts the finished tasks one rule would move.
func (t *tx) CountFinishedTasks(ctx context.Context, q store.FinishedQuery) (int, error) {
	if len(q.Statuses) == 0 {
		return 0, nil
	}
	return t.count(ctx, t.finished(q), "counting finished tasks")
}

// TrashFinished soft deletes a task if it still matches the rule.
func (t *tx) TrashFinished(ctx context.Context, taskID string, q store.FinishedQuery) (bool, error) {
	if len(q.Statuses) == 0 {
		return false, nil
	}
	b := t.finished(q).
		Where("tasks.id = ?", taskID).
		Set("deleted_at", t.now()).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "moving finished task %q to the trash", taskID)
	return n > 0, err
}

// expiredTrash builds the selection of the trash rule.
func (t *tx) expiredTrash(projectID string, before time.Time) *sqlb.Builder {
	return t.builder("tasks").
		Where("tasks.project_id = ?", projectID).
		Where("tasks.deleted_at IS NOT NULL").
		Where("tasks.deleted_at < ?", sqlb.TimeText(before))
}

// ExpiredTrash returns tasks deleted before the cutoff, oldest first.
func (t *tx) ExpiredTrash(ctx context.Context, projectID string, before time.Time, limit int) ([]core.Task, error) {
	b := t.expiredTrash(projectID, before).
		Select(taskColumns...).
		Join(projectJoin).
		OrderBy("tasks.deleted_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(limit)
	return t.housekeepingTasks(ctx, b, "listing expired trash")
}

// CountExpiredTrash counts tasks deleted before the cutoff.
func (t *tx) CountExpiredTrash(ctx context.Context, projectID string, before time.Time) (int, error) {
	return t.count(ctx, t.expiredTrash(projectID, before), "counting expired trash")
}

// TrashedChildren returns a task's children deleted before the cutoff.
func (t *tx) TrashedChildren(ctx context.Context, parentID string, before time.Time) ([]core.Task, error) {
	b := t.builder("tasks").
		Select(taskColumns...).
		Join(projectJoin).
		Where("tasks.parent_id = ?", parentID).
		Where("tasks.deleted_at IS NOT NULL").
		Where("tasks.deleted_at < ?", sqlb.TimeText(before)).
		OrderBy("tasks.id", core.Ascending)
	return t.housekeepingTasks(ctx, b, "listing trashed children")
}

// PurgeTrashed permanently deletes a task still in the trash since before
// the cutoff.
func (t *tx) PurgeTrashed(ctx context.Context, taskID string, before time.Time) (bool, error) {
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.deleted_at IS NOT NULL").
		Where("tasks.deleted_at < ?", sqlb.TimeText(before))
	n, err := t.execDelete(ctx, b, "purging task %q", taskID)
	return n > 0, err
}

// housekeepingTasks runs a task selection.
func (t *tx) housekeepingTasks(ctx context.Context, b *sqlb.Builder, what string) ([]core.Task, error) {
	rows, err := t.query(ctx, b, "%s", what)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.Task{}
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, what)
}

// nullDays reads a nullable rule.
func nullDays(v sql.NullInt64) *int {
	if !v.Valid {
		return nil
	}
	n := int(v.Int64)
	return &n
}

// daysArg writes a nullable rule.
func daysArg(d *int) any {
	if d == nil {
		return nil
	}
	return *d
}
