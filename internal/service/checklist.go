// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/markdown"
)

// CheckTaskItem ticks or clears one checklist item of a task's description.
// Only that item's marker changes. An item already in the asked-for state
// writes nothing and returns the task as it is.
func (l *Local) CheckTaskItem(ctx context.Context, ref core.TaskRef, in core.CheckItemInput) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, task); err != nil {
			return err
		}
		if err := checkVersion(task, in.Version); err != nil {
			return err
		}
		body, err := markdown.SetItem(task.Body, in.Item, in.Done)
		if errors.Is(err, markdown.ErrNoItem) {
			_, total := markdown.Checklist(task.Body)
			return core.Invalid("task %q has no checklist item %d; its description has %d", task.Ref, in.Item, total).
				WithDetail("total", total)
		}
		if err != nil {
			return err
		}
		if body == task.Body {
			out = task
			return nil
		}
		before := *task
		task.Body = body
		if err := m.tx.UpdateTask(ctx, task); err != nil {
			return err
		}
		payload := map[string]any{"ref": task.Ref, "version": task.Version, "fields": []string{"body"},
			"checklist_item": in.Item, "done": in.Done}
		if err := m.Record("task.update", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			before, task, payload); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	markChecklists(out)
	return out, nil
}

// markChecklists sets each task's checklist count from its description.
func markChecklists(tasks ...*core.Task) {
	for _, t := range tasks {
		if t == nil {
			continue
		}
		t.Checklist = nil
		if done, total := markdown.Checklist(t.Body); total > 0 {
			t.Checklist = &core.Checklist{Done: done, Total: total}
		}
	}
}

// markChecklistsIn is markChecklists over a slice.
func markChecklistsIn(tasks []core.Task) {
	for i := range tasks {
		markChecklists(&tasks[i])
	}
}
