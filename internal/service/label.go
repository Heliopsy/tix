package service

import (
	"context"
	"strings"

	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
)

// AddLabel attaches a label to a task, creating the label when it is new.
func (l *Local) AddLabel(ctx context.Context, ref core.TaskRef, label string) error {
	name := strings.TrimSpace(label)
	if name == "" {
		return core.Invalid("label name is required")
	}
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
		lab, err := attachTaskLabel(ctx, m.tx, task.ID, name)
		if err != nil {
			return err
		}
		return m.Record("label.add", core.EventLabelAdded, "task", task.ID, task.ProjectID,
			nil, map[string]any{"task_id": task.ID, "label": lab.Name},
			map[string]any{"ref": task.Ref, "label": lab.Name})
	})
}

// RemoveLabel detaches a label from a task.
func (l *Local) RemoveLabel(ctx context.Context, ref core.TaskRef, label string) error {
	name := strings.TrimSpace(label)
	if name == "" {
		return core.Invalid("label name is required")
	}
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
		lab, err := labelByName(ctx, m.tx, name)
		if err != nil {
			return err
		}
		if err := m.tx.DetachLabel(ctx, task.ID, lab.ID); err != nil {
			return err
		}
		return m.Record("label.remove", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			map[string]any{"task_id": task.ID, "label": lab.Name}, nil,
			map[string]any{"ref": task.Ref, "label": lab.Name, "removed": true})
	})
}

// ListLabels returns every label defined in the tenant.
func (l *Local) ListLabels(ctx context.Context) ([]core.Label, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Label
	err = l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		out, err = tx.ListLabels(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// attachTaskLabel puts a named label on a task, idempotently.
func attachTaskLabel(ctx context.Context, tx store.Tx, taskID, name string) (*core.Label, error) {
	lab := &core.Label{Name: strings.TrimSpace(name)}
	if lab.Name == "" {
		return nil, core.Invalid("label name is required")
	}
	if err := tx.PutLabel(ctx, lab); err != nil {
		return nil, err
	}
	if err := tx.AttachLabel(ctx, taskID, lab.ID); err != nil {
		return nil, err
	}
	return lab, nil
}

// labelByName finds a tenant label by its name.
func labelByName(ctx context.Context, tx store.Tx, name string) (*core.Label, error) {
	labels, err := tx.ListLabels(ctx)
	if err != nil {
		return nil, err
	}
	for i := range labels {
		if labels[i].Name == name {
			return &labels[i], nil
		}
	}
	return nil, core.NotFound("label %q", name)
}

// replaceTaskLabels makes a task's labels exactly the supplied set.
func replaceTaskLabels(ctx context.Context, tx store.Tx, task *core.Task, names []string) error {
	want := make(map[string]bool, len(names))
	for _, n := range names {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		want[n] = true
		if _, err := attachTaskLabel(ctx, tx, task.ID, n); err != nil {
			return err
		}
	}
	for _, existing := range task.Labels {
		if want[existing] {
			continue
		}
		lab, err := labelByName(ctx, tx, existing)
		if err != nil {
			return err
		}
		if err := tx.DetachLabel(ctx, task.ID, lab.ID); err != nil {
			return err
		}
	}
	return nil
}
