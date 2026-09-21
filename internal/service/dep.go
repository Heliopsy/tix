package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// AddDependency records that one task waits for another to finish.
func (l *Local) AddDependency(ctx context.Context, ref, dependsOn core.TaskRef) error {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, task); err != nil {
			return err
		}
		dep, err := liveTask(ctx, m.tx, dependsOn)
		if err != nil {
			return err
		}
		if err := linkDependency(ctx, m.tx, task, dep); err != nil {
			return err
		}
		return m.Record("dependency.add", core.EventDependencyAdded, "task", task.ID, task.ProjectID,
			nil, map[string]any{"task_id": task.ID, "depends_on": dep.ID},
			map[string]any{"ref": task.Ref, "depends_on": dep.Ref})
	})
}

// RemoveDependency drops one dependency edge.
func (l *Local) RemoveDependency(ctx context.Context, ref, dependsOn core.TaskRef) error {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, task); err != nil {
			return err
		}
		dep, err := m.tx.GetTask(ctx, dependsOn)
		if err != nil {
			return err
		}
		if err := m.tx.RemoveDependency(ctx, task.ID, dep.ID); err != nil {
			return err
		}
		return m.Record("dependency.remove", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			map[string]any{"task_id": task.ID, "depends_on": dep.ID}, nil,
			map[string]any{"ref": task.Ref, "depends_on": dep.Ref, "removed": true})
	})
}

// ListDependencies returns the edges leaving a task.
func (l *Local) ListDependencies(ctx context.Context, ref core.TaskRef) ([]core.Dependency, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Dependency
	err = l.read(ctx, actor, func(tx store.Tx) error {
		task, err := liveTask(ctx, tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, task); err != nil {
			return err
		}
		out, err = tx.ListDependencies(ctx, task.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// linkDependency adds an edge after refusing a self edge and any cycle.
func linkDependency(ctx context.Context, tx store.Tx, task, dep *core.Task) error {
	if task.ID == dep.ID {
		return core.Invalid("a task cannot depend on itself")
	}
	cycles, err := tx.DependencyPathExists(ctx, dep.ID, task.ID)
	if err != nil {
		return err
	}
	if cycles {
		return core.Invalid("task %q already depends on %q, so this edge would close a cycle", dep.Ref, task.Ref)
	}
	return tx.AddDependency(ctx, &core.Dependency{TaskID: task.ID, DependsOn: dep.ID})
}
