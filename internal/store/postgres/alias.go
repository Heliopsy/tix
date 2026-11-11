// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
)

// AddProjectAlias keeps a key a project was renamed from.
func (t *tx) AddProjectAlias(ctx context.Context, key, projectID string) error {
	ins := t.insert("project_key_aliases").
		Set("key", key).
		Set("project_id", projectID).
		Set("created_at", timeArg(t.now()))
	_, err := t.execInsert(ctx, ins, "keeping key %q as an alias of project %q", key, projectID)
	return err
}

// DeleteProjectAlias drops a kept key, reporting whether there was one.
func (t *tx) DeleteProjectAlias(ctx context.Context, key string) (bool, error) {
	b := t.builder("project_key_aliases").Where("key = ?", key)
	n, err := t.execDelete(ctx, b, "dropping project key alias %q", key)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ProjectAliasTarget returns the project a kept key belongs to.
func (t *tx) ProjectAliasTarget(ctx context.Context, key string) (string, error) {
	b := t.builder("project_key_aliases").Select("project_id").Where("key = ?", key).Limit(1)
	q, args := b.SelectQuery()
	var projectID string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&projectID); err != nil {
		return "", mapErr(err, "reading project key alias %q", key)
	}
	return projectID, nil
}

// AddTaskAlias keeps the number a task had in a project it moved out of.
func (t *tx) AddTaskAlias(ctx context.Context, projectID string, seq int64, taskID string) error {
	ins := t.insert("task_key_aliases").
		Set("project_id", projectID).
		Set("seq", seq).
		Set("task_id", taskID).
		Set("created_at", timeArg(t.now()))
	_, err := t.execInsert(ctx, ins, "keeping %d of project %q as an alias of task %q", seq, projectID, taskID)
	return err
}

// taskByAlias reads a task whose reference names a key its project was renamed
// from, or a number the task had in a project it moved out of, so a reference
// written before the rename or the move still reaches the task.
func (t *tx) taskByAlias(ctx context.Context, ref core.TaskRef) (core.Task, error) {
	project, err := t.GetProject(ctx, ref.ProjectKey)
	if err != nil {
		return core.Task{}, err
	}
	b := t.builder("tasks").Select(taskColumns...).Join(projectJoin).
		Where("tasks.project_id = ?", project.ID).Where("tasks.seq = ?", ref.Seq).Limit(1)
	q, args := b.SelectQuery()
	task, err := scanTask(t.ex.QueryRowContext(ctx, q, args...))
	if err == nil || !core.IsKind(err, core.KindNotFound) {
		return task, err
	}
	a := t.builder("task_key_aliases").Select("task_id").
		Where("project_id = ?", project.ID).Where("seq = ?", ref.Seq).Limit(1)
	q, args = a.SelectQuery()
	var taskID string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&taskID); err != nil {
		return core.Task{}, mapErr(err, "reading task alias %s", ref.String())
	}
	b = t.builder("tasks").Select(taskColumns...).Join(projectJoin).Where("tasks.id = ?", taskID).Limit(1)
	q, args = b.SelectQuery()
	return scanTask(t.ex.QueryRowContext(ctx, q, args...))
}
