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

// taskByAlias reads a task whose reference names a key its project was renamed
// from, so a reference written before the rename still reaches the task.
func (t *tx) taskByAlias(ctx context.Context, ref core.TaskRef) (core.Task, error) {
	projectID, err := t.ProjectAliasTarget(ctx, ref.ProjectKey)
	if err != nil {
		return core.Task{}, err
	}
	b := t.builder("tasks").Select(taskColumns...).Join(projectJoin).
		Where("tasks.project_id = ?", projectID).Where("tasks.seq = ?", ref.Seq).Limit(1)
	q, args := b.SelectQuery()
	return scanTask(t.ex.QueryRowContext(ctx, q, args...))
}
