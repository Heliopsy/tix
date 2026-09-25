// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// skipLocked lets a second worker step over the row a first worker is already
// claiming instead of blocking on it and then finding it taken.
const skipLocked = " FOR UPDATE SKIP LOCKED"

// ClaimTask takes a lease on one task. The whole decision is a single
// conditional UPDATE, so two processes racing for the same task cannot both win.
func (t *tx) ClaimTask(ctx context.Context, in store.ClaimRow) (bool, error) {
	now := timeArg(in.Now)
	b := t.builder("tasks").
		Where("tasks.id = ?", in.TaskID).
		Where("tasks.deleted_at IS NULL").
		Where(unclaimedPredicate, now).
		Set("claimed_by_actor_id", in.ActorID).
		Set("claimed_at", now).
		Set("lease_expires_at", timeArg(in.Until)).
		Set("lease_token", in.LeaseToken).
		Set("lease_expired_at", nil).
		Set("lease_expired_by", nil).
		Set("updated_at", now).
		SetExpr("claim_count", "claim_count + 1")
	n, err := t.execUpdate(ctx, b, "claiming task %q", in.TaskID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClaimNextTask claims the highest-priority eligible task that no unfinished
// dependency blocks. The candidate is picked and locked in its own statement so
// a second worker steps over it rather than queueing behind it.
func (t *tx) ClaimNextTask(ctx context.Context, in store.ClaimNextRow) (string, bool, error) {
	now := timeArg(in.Now)

	pick := t.builder("tasks").
		Select("tasks.id").
		Where("tasks.deleted_at IS NULL").
		Where(unclaimedPredicate, now).
		Where("NOT EXISTS "+blockedByDependency(in.TerminalStates), terminalArgs(in.TerminalStates)...)
	if len(in.TerminalStates) > 0 {
		pick.Where(notTerminalPredicate(in.TerminalStates), terminalArgs(in.TerminalStates)...)
	}
	if len(in.ProjectIDs) > 0 {
		pick.WhereIn("tasks.project_id", in.ProjectIDs)
	}
	if len(in.Statuses) > 0 {
		pick.WhereIn("tasks.status", in.Statuses)
	}
	if len(in.Tags) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(in.Tags)), ", ")
		args := make([]any, 0, len(in.Tags))
		for _, l := range in.Tags {
			args = append(args, l)
		}
		pick.Where("EXISTS (SELECT 1 FROM task_tags tl JOIN tags l"+
			" ON l.id = tl.tag_id AND l.tenant_id = tl.tenant_id"+
			" WHERE tl.tenant_id = tasks.tenant_id AND tl.task_id = tasks.id AND l.name IN ("+marks+"))",
			args...)
	}
	pick.OrderBy("tasks.priority", core.Ascending).
		OrderBy("tasks.created_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(1)

	q, args := pick.SelectQuery()
	var taskID string
	switch err := t.ex.QueryRowContext(ctx, q+skipLocked, args...).Scan(&taskID); {
	case errors.Is(err, sql.ErrNoRows):
		return "", false, nil
	case err != nil:
		return "", false, mapErr(err, "choosing the next eligible task")
	}

	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where(unclaimedPredicate, now).
		Set("claimed_by_actor_id", in.ActorID).
		Set("claimed_at", now).
		Set("lease_expires_at", timeArg(in.Until)).
		Set("lease_token", in.LeaseToken).
		Set("lease_expired_at", nil).
		Set("lease_expired_by", nil).
		Set("updated_at", now).
		SetExpr("claim_count", "claim_count + 1")
	n, err := t.execUpdate(ctx, b, "claiming task %q", taskID)
	if err != nil {
		return "", false, err
	}
	if n == 0 {
		return "", false, nil
	}
	return taskID, true, nil
}

func terminalArgs(terminal []string) []any {
	out := make([]any, 0, len(terminal))
	for _, s := range terminal {
		out = append(out, s)
	}
	return out
}

// notTerminalPredicate renders the predicate excluding a task that is already
// in one of the workflow's terminal states. Finished work is not queue work.
func notTerminalPredicate(terminal []string) string {
	marks := strings.TrimSuffix(strings.Repeat("?, ", len(terminal)), ", ")
	return "tasks.status NOT IN (" + marks + ")"
}

// blockedByDependency renders the predicate matching an unfinished dependency.
// With no terminal states named, every dependency counts as unfinished.
func blockedByDependency(terminal []string) string {
	cond := "dep.deleted_at IS NULL"
	if len(terminal) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(terminal)), ", ")
		cond += " AND dep.status NOT IN (" + marks + ")"
	}
	return `(SELECT 1 FROM task_deps d JOIN tasks dep
   ON dep.id = d.depends_on AND dep.tenant_id = d.tenant_id
 WHERE d.tenant_id = tasks.tenant_id AND d.task_id = tasks.id
   AND ` + cond + `)`
}

// RenewLease extends a lease only while the caller's token is the current one.
// An expired lease cannot be renewed: lazy expiry means such a task already
// reads as unclaimed and may have been handed to another worker.
func (t *tx) RenewLease(ctx context.Context, taskID, token string, until time.Time) (bool, error) {
	if token == "" {
		return false, core.Invalid("lease token must not be empty")
	}
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.lease_token = ?", token).
		Where("tasks.lease_expires_at > ?", t.now()).
		Set("lease_expires_at", timeArg(until)).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "renewing the lease on task %q", taskID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ReleaseLease clears a claim only while the caller holds a live lease.
func (t *tx) ReleaseLease(ctx context.Context, taskID, token string) (bool, error) {
	if token == "" {
		return false, core.Invalid("lease token must not be empty")
	}
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.lease_token = ?", token).
		Where("tasks.lease_expires_at > ?", t.now()).
		Set("claimed_by_actor_id", nil).
		Set("claimed_at", nil).
		Set("lease_expires_at", nil).
		Set("lease_token", nil).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "releasing the lease on task %q", taskID)
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClearClaim drops a claim regardless of its token, for the sweeper, and
// records who held it and when it lapsed in the same statement so the evidence
// cannot disagree with the lease it describes.
func (t *tx) ClearClaim(ctx context.Context, in store.ExpireClaimRow) error {
	b := t.builder("tasks").
		Where("tasks.id = ?", in.TaskID).
		Set("claimed_by_actor_id", nil).
		Set("claimed_at", nil).
		Set("lease_expires_at", nil).
		Set("lease_token", nil).
		Set("lease_expired_at", timeArg(in.At)).
		Set("lease_expired_by", nullText(in.HolderID)).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "clearing the claim on task %q", in.TaskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q", in.TaskID)
	}
	return nil
}

// ExpiredLeases returns tasks whose lease has passed, for the sweeper.
func (t *tx) ExpiredLeases(ctx context.Context, now time.Time, limit int) ([]core.Task, error) {
	if limit <= 0 {
		limit = core.DefaultPageLimit
	}
	b := t.builder("tasks").
		Select(taskColumns...).
		Join(projectJoin).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.lease_expires_at IS NOT NULL").
		Where("tasks.lease_expires_at <= ?", timeArg(now)).
		OrderBy("tasks.lease_expires_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(limit)
	rows, err := t.query(ctx, b, "listing expired leases")
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
	if err := mapRowsErr(rows, "listing expired leases"); err != nil {
		return nil, err
	}
	return out, t.fillTaskRelations(ctx, out)
}
