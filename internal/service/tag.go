package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// AddTag attaches a tag to a task, creating the tag when it is new.
func (l *Local) AddTag(ctx context.Context, ref core.TaskRef, tag string) error {
	name := strings.TrimSpace(tag)
	if name == "" {
		return core.Invalid("tag name is required")
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
		tag, err := attachTaskLabel(ctx, m.tx, task.ID, name)
		if err != nil {
			return err
		}
		return m.Record("tag.add", core.EventLabelAdded, "task", task.ID, task.ProjectID,
			nil, map[string]any{"task_id": task.ID, "tag": tag.Name},
			map[string]any{"ref": task.Ref, "tag": tag.Name})
	})
}

// RemoveTag detaches a tag from a task.
func (l *Local) RemoveTag(ctx context.Context, ref core.TaskRef, tag string) error {
	name := strings.TrimSpace(tag)
	if name == "" {
		return core.Invalid("tag name is required")
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
		tag, err := tagByName(ctx, m.tx, name)
		if err != nil {
			return err
		}
		if err := m.tx.DetachTag(ctx, task.ID, tag.ID); err != nil {
			return err
		}
		return m.Record("tag.remove", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			map[string]any{"task_id": task.ID, "tag": tag.Name}, nil,
			map[string]any{"ref": task.Ref, "tag": tag.Name, "removed": true})
	})
}

// ListTags returns every tag defined in the tenant.
func (l *Local) ListTags(ctx context.Context) ([]core.Tag, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.Tag
	err = l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		out, err = tx.ListTags(ctx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// attachTaskLabel puts a named tag on a task, idempotently.
func attachTaskLabel(ctx context.Context, tx store.Tx, taskID, name string) (*core.Tag, error) {
	tag := &core.Tag{Name: strings.TrimSpace(name)}
	if tag.Name == "" {
		return nil, core.Invalid("tag name is required")
	}
	if err := tx.PutTag(ctx, tag); err != nil {
		return nil, err
	}
	if err := tx.AttachTag(ctx, taskID, tag.ID); err != nil {
		return nil, err
	}
	return tag, nil
}

// tagByName finds a tenant tag by its name.
func tagByName(ctx context.Context, tx store.Tx, name string) (*core.Tag, error) {
	tags, err := tx.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	for i := range tags {
		if tags[i].Name == name {
			return &tags[i], nil
		}
	}
	return nil, core.NotFound("tag %q", name)
}

// replaceTaskLabels makes a task's tags exactly the supplied set.
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
	for _, existing := range task.Tags {
		if want[existing] {
			continue
		}
		tag, err := tagByName(ctx, tx, existing)
		if err != nil {
			return err
		}
		if err := tx.DetachTag(ctx, task.ID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}
