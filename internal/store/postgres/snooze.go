// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// SetSnooze writes a task's wake time and who set it, or clears both. It
// leaves the version alone, like a claim, so an edit read before the snooze
// still saves and cannot clobber it.
func (t *tx) SetSnooze(ctx context.Context, taskID string, until *time.Time, by string) error {
	if until == nil {
		by = ""
	}
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.deleted_at IS NULL").
		Set("snoozed_until", nullTimeArg(until)).
		Set("snoozed_by", nullText(by)).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "snoozing task %q", taskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q", taskID)
	}
	return nil
}

// ExpiredSnoozes returns live tasks whose wake time has passed, oldest first.
func (t *tx) ExpiredSnoozes(ctx context.Context, now time.Time, limit int) ([]core.Task, error) {
	if limit <= 0 {
		limit = core.DefaultPageLimit
	}
	b := t.builder("tasks").
		Select(taskColumns...).
		Join(projectJoin).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.snoozed_until IS NOT NULL").
		Where("tasks.snoozed_until <= ?", timeArg(now)).
		OrderBy("tasks.snoozed_until", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(limit)
	rows, err := t.query(ctx, b, "listing passed snoozes")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Task{}
	for rows.Next() {
		v, err := scanTask(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := mapRowsErr(rows, "listing passed snoozes"); err != nil {
		return nil, err
	}
	return out, nil
}

// WakeSnooze clears a passed snooze only if the row still carries the wake
// time the caller read, so a task snoozed again since is left asleep.
func (t *tx) WakeSnooze(ctx context.Context, taskID string, until time.Time) (bool, error) {
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.snoozed_until = ?", timeArg(until)).
		Set("snoozed_until", nil).
		Set("snoozed_by", nil)
	n, err := t.execUpdate(ctx, b, "waking task %q", taskID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}
