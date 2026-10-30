// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// StarTask sets the caller's own star on a task. A task already starred is
// left as it is, with nothing written.
func (l *Local) StarTask(ctx context.Context, ref core.TaskRef) error {
	return l.setStar(ctx, ref, true)
}

// UnstarTask clears the caller's own star on a task. A task not starred is
// left as it is, with nothing written.
func (l *Local) UnstarTask(ctx context.Context, ref core.TaskRef) error {
	return l.setStar(ctx, ref, false)
}

func (l *Local) setStar(ctx context.Context, ref core.TaskRef, starred bool) error {
	actor, err := l.authorize(ctx, authz.ActionTaskStar, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskStar, task); err != nil {
			return err
		}
		change, action, typ := m.tx.UnstarTask, "task.unstar", core.EventTaskUnstarred
		if starred {
			change, action, typ = m.tx.StarTask, "task.star", core.EventTaskStarred
		}
		changed, err := change(ctx, actor.ID, task.ID)
		if err != nil || !changed {
			return err
		}
		row := map[string]any{"task_id": task.ID, "actor_id": actor.ID}
		before, after := any(nil), any(row)
		if !starred {
			before, after = row, nil
		}
		return m.Record(action, typ, "task", task.ID, task.ProjectID, before, after,
			map[string]any{"ref": task.Ref, "starred": starred})
	})
}

// markStarred sets each task's Starred from the reader's own stars.
func markStarred(ctx context.Context, tx store.Tx, actorID string, tasks []core.Task) error {
	if actorID == "" || len(tasks) == 0 {
		return nil
	}
	ids := make([]string, len(tasks))
	for i := range tasks {
		ids[i] = tasks[i].ID
	}
	starred, err := tx.StarredTasks(ctx, actorID, ids)
	if err != nil {
		return err
	}
	for i := range tasks {
		tasks[i].Starred = starred[tasks[i].ID]
	}
	return nil
}
