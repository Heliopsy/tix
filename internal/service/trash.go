// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// trashOff reports whether the tenant the transaction is scoped to deletes
// tasks permanently.
func trashOff(ctx context.Context, tx store.Tx) (bool, error) {
	t, err := tx.GetTenant(ctx)
	if err != nil {
		return false, err
	}
	return t.TrashOff, nil
}

// TrashStatus says whether the tenant keeps deleted tasks and how many it
// holds. Any reader may ask, because every surface that draws a Delete needs
// to know whether it can be undone.
func (l *Local) TrashStatus(ctx context.Context) (*core.TrashStatus, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := &core.TrashStatus{}
	err = l.read(ctx, actor, func(tx store.Tx) error {
		off, err := trashOff(ctx, tx)
		if err != nil {
			return err
		}
		out.Enabled = !off
		project := ""
		if actor.ScopedToProject() {
			project = actor.ProjectID
		}
		out.Held, err = tx.CountTrashed(ctx, project)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// trashWord is how an audit entry names the trash setting.
func trashWord(on bool) string {
	if on {
		return "on"
	}
	return "off"
}

// PurgeTask permanently deletes a task in the trash and its trashed
// subtasks, through the same subtree walk, delete and record the trash rule
// uses, so a purge on request leaves what an expiry leaves.
func (l *Local) PurgeTask(ctx context.Context, ref core.TaskRef) error {
	actor, err := l.authorize(ctx, authz.ActionTaskDelete, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		task, err := m.tx.GetTask(ctx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskDelete, task); err != nil {
			return err
		}
		if task.DeletedAt == nil {
			return core.Precondition("task %q is not in the trash; delete it first", task.Ref).
				WithDetail("reason", core.ErrNotTrashed)
		}
		if off, err := trashOff(ctx, m.tx); err != nil {
			return err
		} else if off {
			return core.TrashIsOff("nothing can be purged from it")
		}
		subtree, err := trashedSubtree(ctx, m.tx, *task, time.Time{})
		if err != nil {
			return err
		}
		for i := range subtree {
			t := &subtree[i]
			if err := l.authorizeTask(ctx, authz.ActionTaskDelete, t); err != nil {
				return err
			}
			ok, err := m.tx.PurgeTrashed(ctx, t.ID, time.Time{})
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			if err := m.Record("task.delete", core.EventTaskDeleted, "task", t.ID, t.ProjectID,
				t, nil, map[string]any{"ref": t.Ref, "hard": true, "purged": true}); err != nil {
				return err
			}
		}
		return nil
	})
}
