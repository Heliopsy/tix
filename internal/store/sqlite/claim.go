// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// ClaimTask takes a lease on one task. The whole decision is a single
// conditional UPDATE, so two processes racing for the same task cannot both win.
func (t *tx) ClaimTask(ctx context.Context, in store.ClaimRow) (bool, error) {
	now := sqlb.TimeText(in.Now)
	b := t.builder("tasks").
		Where("tasks.id = ?", in.TaskID).
		Where("tasks.deleted_at IS NULL").
		Where(unclaimedPredicate, now).
		Set("claimed_by_actor_id", in.ActorID).
		Set("claimed_at", now).
		Set("lease_expires_at", sqlb.TimeText(in.Until)).
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
// dependency blocks, in one conditional UPDATE.
func (t *tx) ClaimNextTask(ctx context.Context, in store.ClaimNextRow) (string, bool, error) {
	now := sqlb.TimeText(in.Now)

	inner := t.builder("tasks").
		Select("tasks.id").
		Where("tasks.deleted_at IS NULL").
		Where(unclaimedPredicate, now)
	blocked, blockedArgs := blockedByDependency(in.Terminal)
	inner.Where("NOT EXISTS "+blocked, blockedArgs...)
	finished, finishedArgs := terminalPredicate("tasks.status", "tasks.project_id", in.Terminal)
	inner.Where("NOT "+finished, finishedArgs...)
	if len(in.ProjectIDs) > 0 {
		inner.WhereIn("tasks.project_id", in.ProjectIDs)
	}
	if len(in.Statuses) > 0 {
		inner.WhereIn("tasks.status", in.Statuses)
	}
	if len(in.Tags) > 0 {
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(in.Tags)), ", ")
		args := make([]any, 0, len(in.Tags))
		for _, l := range in.Tags {
			args = append(args, l)
		}
		inner.Where("EXISTS (SELECT 1 FROM task_tags tl JOIN tags l"+
			" ON l.id = tl.tag_id AND l.tenant_id = tl.tenant_id"+
			" WHERE tl.tenant_id = tasks.tenant_id AND tl.task_id = tasks.id AND l.name IN ("+marks+"))",
			args...)
	}
	inner.OrderBy("tasks.priority", core.Ascending).
		OrderBy("tasks.created_at", core.Ascending).
		OrderBy("tasks.id", core.Ascending).
		Limit(1)

	sub, subArgs := inner.SelectQuery()

	b := t.builder("tasks").
		Where("tasks.id IN ("+sub+")", subArgs...).
		Where(unclaimedPredicate, now).
		Set("claimed_by_actor_id", in.ActorID).
		Set("claimed_at", now).
		Set("lease_expires_at", sqlb.TimeText(in.Until)).
		Set("lease_token", in.LeaseToken).
		Set("lease_expired_at", nil).
		Set("lease_expired_by", nil).
		Set("updated_at", now).
		SetExpr("claim_count", "claim_count + 1")
	n, err := t.execUpdate(ctx, b, "claiming the next eligible task")
	if err != nil {
		return "", false, err
	}
	if n == 0 {
		return "", false, nil
	}

	find := t.builder("tasks").
		Select("tasks.id").
		Where("tasks.lease_token = ?", in.LeaseToken).
		Limit(1)
	q, args := find.SelectQuery()
	var taskID string
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&taskID); err != nil {
		return "", false, mapErr(err, "reading back the claimed task")
	}
	return taskID, true, nil
}

// terminalPredicate renders "this row's status finishes work under the workflow
// that governs its project". The workflow is resolved from the row rather than
// assumed, because a state name that is terminal in one workflow is a waypoint
// in the next, and the rows this judges span every project in the tenant.
func terminalPredicate(status, projectID string, terminal []store.WorkflowTerminal) (string, []any) {
	parts := make([]string, 0, len(terminal))
	args := make([]any, 0, len(terminal)*2)
	for _, wf := range terminal {
		if len(wf.States) == 0 {
			continue
		}
		marks := strings.TrimSuffix(strings.Repeat("?, ", len(wf.States)), ", ")
		parts = append(parts, "(wp.workflow_id = ? AND "+status+" IN ("+marks+"))")
		args = append(args, wf.WorkflowID)
		for _, state := range wf.States {
			args = append(args, state)
		}
	}
	if len(parts) == 0 {
		return "(1 = 0)", nil
	}
	return "(EXISTS (SELECT 1 FROM projects wp WHERE wp.id = " + projectID +
		" AND wp.tenant_id = tasks.tenant_id AND (" + strings.Join(parts, " OR ") + ")))", args
}

// blockedByDependency renders the predicate matching an unfinished dependency.
// With no terminal states named, every dependency counts as unfinished.
func blockedByDependency(terminal []store.WorkflowTerminal) (string, []any) {
	cond, args := terminalPredicate("dep.status", "dep.project_id", terminal)
	return `(SELECT 1 FROM task_deps d JOIN tasks dep
   ON dep.id = d.depends_on AND dep.tenant_id = d.tenant_id
 WHERE d.tenant_id = tasks.tenant_id AND d.task_id = tasks.id
   AND dep.deleted_at IS NULL AND NOT ` + cond + `)`, args
}

// RenewLease extends a lease only while the caller's token is the current one.
// An expired lease cannot be renewed. Lazy expiry means such a task already
// reads as unclaimed and may have been handed to another worker, so the holder
// must re-claim rather than resurrect its old lease.
func (t *tx) RenewLease(ctx context.Context, taskID, token string, until time.Time) (bool, error) {
	if token == "" {
		return false, core.Invalid("lease token must not be empty")
	}
	b := t.builder("tasks").
		Where("tasks.id = ?", taskID).
		Where("tasks.lease_token = ?", token).
		Where("tasks.lease_expires_at > ?", t.now()).
		Set("lease_expires_at", sqlb.TimeText(until)).
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

// ClearClaim drops a lapsed claim regardless of its token, for the sweeper, and
// records who held it and when it lapsed in the same statement so the evidence
// cannot disagree with the lease it describes. The lapse is re-asserted inside
// the statement, so a lease taken while the sweeper was reading survives.
func (t *tx) ClearClaim(ctx context.Context, in store.ExpireClaimRow) (bool, error) {
	b := t.builder("tasks").
		Where("tasks.id = ?", in.TaskID).
		Where("tasks.lease_expires_at IS NOT NULL").
		Where("tasks.lease_expires_at <= ?", sqlb.TimeText(in.At)).
		Set("claimed_by_actor_id", nil).
		Set("claimed_at", nil).
		Set("lease_expires_at", nil).
		Set("lease_token", nil).
		Set("lease_expired_at", sqlb.TimeText(in.At)).
		Set("lease_expired_by", sqlb.NullText(in.HolderID)).
		Set("updated_at", t.now())
	n, err := t.execUpdate(ctx, b, "clearing the claim on task %q", in.TaskID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	return false, t.claimStillThere(ctx, in.TaskID)
}

// ForceReclaim ends a live lease without its token. The holder the caller read
// is re-asserted in the statement, so an administrator can only take the lease
// they were looking at: a second administrator forcing the same task, and a
// holder that released and was replaced in between, both match nothing and are
// told so rather than being handed a success over somebody else's claim.
//
// It sets no expiry evidence. lease_expired_at and lease_expired_by say "the
// holder stopped answering", which is how a repeatedly dying agent is spotted,
// and an operator's decision written into those columns would read as exactly
// the failure it is not. What happened lives in the audit entry and the event.
func (t *tx) ForceReclaim(ctx context.Context, in store.ForceReclaimRow) (bool, error) {
	now := sqlb.TimeText(in.At)
	b := t.builder("tasks").
		Where("tasks.id = ?", in.TaskID).
		Where("tasks.deleted_at IS NULL").
		Where("tasks.claimed_by_actor_id = ?", in.HolderID).
		Where("tasks.lease_expires_at IS NOT NULL").
		Where("tasks.lease_expires_at > ?", now).
		Set("claimed_by_actor_id", nil).
		Set("claimed_at", nil).
		Set("lease_expires_at", nil).
		Set("lease_token", nil).
		Set("updated_at", now)
	n, err := t.execUpdate(ctx, b, "reclaiming the lease on task %q", in.TaskID)
	if err != nil {
		return false, err
	}
	if n > 0 {
		return true, nil
	}
	return false, t.claimStillThere(ctx, in.TaskID)
}

// claimStillThere separates the two ways the conditional clear matches nothing:
// a task this tenant does not have, and a task whose lease is no longer the
// lapsed one the sweeper read.
func (t *tx) claimStillThere(ctx context.Context, taskID string) error {
	b := t.builder("tasks").Where("tasks.id = ?", taskID)
	n, err := t.count(ctx, b, "checking the claim on task %q", taskID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("task %q", taskID)
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
		Where("tasks.lease_expires_at <= ?", sqlb.TimeText(now)).
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
