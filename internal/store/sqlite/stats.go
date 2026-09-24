// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// terminalActorExpr names the actor who made the move that terminated a task.
// The tasks table records when a task reached a terminal state and never by
// whom, so the audit trail is the only record of it, matched on the instant
// both rows were written with. A task whose transition has aged out of
// retention reports no actor rather than the wrong one.
const terminalActorExpr = `(SELECT ae.actor_id FROM audit_entries ae ` +
	`WHERE ae.tenant_id = tasks.tenant_id AND ae.subject_type = 'task' ` +
	`AND ae.subject_id = tasks.id AND ae.action = 'task.transition' ` +
	`AND ae.occurred_at = tasks.completed_at ORDER BY ae.seq DESC LIMIT 1)`

// TaskStats answers one statistics read.
func (t *tx) TaskStats(ctx context.Context, q store.StatsQuery) (*store.StatsRows, error) {
	out := &store.StatsRows{
		Completed:    []store.CompletedTask{},
		StatusCounts: map[string]int{},
		Oldest:       []core.Task{},
	}
	var err error
	if out.Completed, err = t.statsCompleted(ctx, q); err != nil {
		return nil, err
	}
	if out.Created, err = t.statsCreated(ctx, q); err != nil {
		return nil, err
	}
	for _, status := range q.Statuses {
		b := t.builder("tasks").Where("tasks.deleted_at IS NULL").Where("tasks.status = ?", status)
		statsProject(b, q)
		n, err := t.count(ctx, b, "counting tasks in status %q", status)
		if err != nil {
			return nil, err
		}
		out.StatusCounts[status] = n
	}
	if out.Oldest, err = t.statsOldest(ctx, q); err != nil {
		return nil, err
	}
	return out, nil
}

// statsProject narrows a statistic to one project when the read named one.
func statsProject(b *sqlb.Builder, q store.StatsQuery) {
	if q.ProjectID != "" {
		b.Where("tasks.project_id = ?", q.ProjectID)
	}
}

func (t *tx) statsCompleted(ctx context.Context, q store.StatsQuery) ([]store.CompletedTask, error) {
	b := t.builder("tasks").
		Select("tasks.id", "tasks.project_id", "tasks.created_at", "tasks.completed_at", terminalActorExpr).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.completed_at IS NOT NULL").
		Where("tasks.completed_at >= ?", sqlb.TimeText(q.Since)).
		Where("tasks.completed_at <= ?", sqlb.TimeText(q.Until)).
		OrderBy("tasks.completed_at", core.Ascending)
	statsProject(b, q)

	rows, err := t.query(ctx, b, "reading completed tasks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []store.CompletedTask{}
	for rows.Next() {
		var (
			row       store.CompletedTask
			created   sql.NullString
			completed sql.NullString
			actor     sql.NullString
		)
		if err := rows.Scan(&row.TaskID, &row.ProjectID, &created, &completed, &actor); err != nil {
			return nil, mapErr(err, "scanning completed task")
		}
		row.ActorID = sqlb.Text(actor)
		if row.CreatedAt, err = sqlb.ScanTime(created); err != nil {
			return nil, err
		}
		if row.CompletedAt, err = sqlb.ScanTime(completed); err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	return out, mapRowsErr(rows, "reading completed tasks")
}

func (t *tx) statsCreated(ctx context.Context, q store.StatsQuery) (int, error) {
	b := t.builder("tasks").
		Where("tasks.deleted_at IS NULL").
		Where("tasks.created_at >= ?", sqlb.TimeText(q.Since)).
		Where("tasks.created_at <= ?", sqlb.TimeText(q.Until))
	statsProject(b, q)
	return t.count(ctx, b, "counting created tasks")
}

// statsOldest returns the longest-waiting tasks that have not reached a
// terminal state. A terminal state is the only thing that sets completed_at and
// leaving one clears it again, so a null there is exactly "not terminal" and
// costs no join against the workflow.
func (t *tx) statsOldest(ctx context.Context, q store.StatsQuery) ([]core.Task, error) {
	if q.Oldest <= 0 {
		return []core.Task{}, nil
	}
	b := t.builder("tasks").
		Select(taskColumns...).
		Join(projectJoin).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.completed_at IS NULL").
		OrderBy("tasks.created_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(q.Oldest)
	statsProject(b, q)

	rows, err := t.query(ctx, b, "reading the oldest open tasks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Task{}
	for rows.Next() {
		task, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, task)
	}
	return out, mapRowsErr(rows, "reading the oldest open tasks")
}
