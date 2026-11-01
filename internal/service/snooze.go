// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
)

// SnoozeTask hides a task from default listings until in.Until, for every
// reader. Snoozing a snoozed task moves its wake time.
func (l *Local) SnoozeTask(ctx context.Context, ref core.TaskRef, in core.SnoozeInput) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Until.IsZero() {
		return nil, core.Invalid("a snooze needs a time to wake")
	}
	until := in.Until.UTC().Truncate(time.Second)

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		if !until.After(m.now) {
			return core.Invalid("a snooze must wake in the future, and %s has passed", until.Format(time.RFC3339))
		}
		task, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, task); err != nil {
			return err
		}
		if err := m.tx.SetSnooze(ctx, task.ID, &until, actor.ID); err != nil {
			return err
		}
		after, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		out = after
		return m.Record("task.snooze", core.EventTaskSnoozed, "task", task.ID, task.ProjectID,
			snoozeRow(task, m.now), snoozeRow(after, m.now),
			map[string]any{"ref": task.Ref, "snoozed_until": until, "snoozed_by": actor.ID})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// UnsnoozeTask clears a task's snooze. A task that is not snoozed is returned
// as it is, with nothing written.
func (l *Local) UnsnoozeTask(ctx context.Context, ref core.TaskRef) (*core.Task, error) {
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
		out = task
		if !task.SnoozedAt(m.now) {
			return nil
		}
		if err := m.tx.SetSnooze(ctx, task.ID, nil, ""); err != nil {
			return err
		}
		after, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		out = after
		return m.Record("task.unsnooze", core.EventTaskUnsnoozed, "task", task.ID, task.ProjectID,
			snoozeRow(task, m.now), snoozeRow(after, m.now),
			map[string]any{"ref": task.Ref, "snoozed_until": task.SnoozedUntil})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// WakeSnoozed clears up to limit snoozes whose time has passed, announcing
// each with task.woke. The tasks were already back in every listing; this
// writes down what a read concludes and tells subscribers.
func (l *Local) WakeSnoozed(ctx context.Context, limit int) (int, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return 0, err
	}
	woken := 0
	err = l.write(ctx, actor, func(m *mutation) error {
		due, err := m.tx.ExpiredSnoozes(ctx, m.now, limit)
		if err != nil {
			return err
		}
		for i := range due {
			task := due[i]
			ok, err := m.tx.WakeSnooze(ctx, task.ID, *task.SnoozedUntil)
			if err != nil {
				return err
			}
			if !ok {
				continue
			}
			woken++
			if err := m.Record("task.wake", core.EventTaskWoke, "task", task.ID, task.ProjectID,
				snoozeRow(&task, time.Time{}), map[string]any{"id": task.ID, "ref": task.Ref},
				map[string]any{"ref": task.Ref, "snoozed_until": task.SnoozedUntil, "snoozed_by": task.SnoozedByActorID}); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return woken, nil
}

// snoozeRow is what an audit entry for a snooze records: the task and its
// wake time, or no wake time when it is not snoozed at now.
func snoozeRow(t *core.Task, now time.Time) map[string]any {
	row := map[string]any{"id": t.ID, "ref": t.Ref}
	if t.SnoozedUntil != nil && t.SnoozedUntil.After(now) {
		row["snoozed_until"] = t.SnoozedUntil
		row["snoozed_by"] = t.SnoozedByActorID
	}
	return row
}
