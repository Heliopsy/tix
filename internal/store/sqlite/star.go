// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"

	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// StarTask records an actor's star on a task, reporting whether it was new.
// It reads before it writes rather than swallowing a conflict, because on
// PostgreSQL a failed insert aborts the transaction the audit entry is in.
func (t *tx) StarTask(ctx context.Context, actorID, taskID string) (bool, error) {
	n, err := t.count(ctx, t.starRow(actorID, taskID), "reading star on task %q", taskID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return false, nil
	}
	ins := t.insert("task_stars").
		Set("actor_id", actorID).
		Set("task_id", taskID).
		Set("created_at", t.now())
	if _, err := t.execInsert(ctx, ins, "starring task %q", taskID); err != nil {
		return false, err
	}
	return true, nil
}

// UnstarTask removes an actor's star on a task, reporting whether there was one.
func (t *tx) UnstarTask(ctx context.Context, actorID, taskID string) (bool, error) {
	n, err := t.execDelete(ctx, t.starRow(actorID, taskID), "unstarring task %q", taskID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// StarredTasks reports which of the given tasks the actor has starred.
func (t *tx) StarredTasks(ctx context.Context, actorID string, taskIDs []string) (map[string]bool, error) {
	out := map[string]bool{}
	if actorID == "" || len(taskIDs) == 0 {
		return out, nil
	}
	b := t.builder("task_stars").
		Select("task_stars.task_id", "task_stars.actor_id").
		Where("task_stars.actor_id = ?", actorID)
	b.WhereIn("task_stars.task_id", taskIDs)
	err := t.eachPair(ctx, b, "reading stars", func(taskID, _ string) { out[taskID] = true })
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (t *tx) starRow(actorID, taskID string) *sqlb.Builder {
	return t.builder("task_stars").
		Where("task_stars.actor_id = ?", actorID).
		Where("task_stars.task_id = ?", taskID)
}
