// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// ForceReclaim ends the lease another actor holds, for an administrator whose
// worker has died or wandered off.
//
// It is a reclaim and nothing else. The task keeps its status, including a
// terminal one, and the workflow's revert-on-lease-expiry rule is deliberately
// not consulted here although the sweeper consults it for a lapse. Reopening
// finished work is a transition an administrator makes on purpose; folding it
// into this call would record a status change in the trail under an action
// whose name does not mention one, and leave the operator unable to tell the
// move they chose from the move the reclaim made for them.
func (l *Local) ForceReclaim(ctx context.Context, ref core.TaskRef, in core.ForceReclaimInput) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskReclaim, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := l.reclaimTarget(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if !task.ClaimedAtTime(m.now) {
			return core.Conflict("task %q holds no live lease to reclaim", task.Ref)
		}
		holder := task.ClaimedByActorID

		ok, err := m.tx.ForceReclaim(ctx, store.ForceReclaimRow{
			TaskID:   task.ID,
			HolderID: holder,
			At:       m.now,
		})
		if err != nil {
			return err
		}
		if !ok {
			return reclaimRefused(task, holder)
		}

		reclaimed, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if err := m.Record("task.reclaim", core.EventTaskReclaimed, "task", task.ID, task.ProjectID,
			task, reclaimed, l.reclaimPayload(ctx, m, holder, reclaimed.Status, in.Reason)); err != nil {
			return err
		}
		out = reclaimed
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// reclaimPayload is what the audit entry and the event carry. forced is the
// fact that separates this from a lapse: the columns a sweep leaves behind say
// a holder stopped answering, which is how a repeatedly dying agent is found,
// and an operator's decision written into that shape would read as the failure
// it is not. The status travels too, so the entry states in its own payload
// that the task did not move.
func (l *Local) reclaimPayload(ctx context.Context, m *mutation, holder, status, reason string) map[string]any {
	payload := map[string]any{
		"forced":          true,
		"previous_holder": holder,
		"status":          status,
	}
	if previous, err := lookupActor(ctx, m.tx, holder); err == nil {
		payload["previous_holder_handle"] = previous.Handle
	}
	if reason != "" {
		payload["reason"] = reason
	}
	return payload
}

// reclaimRefused reports a compare-and-swap that matched nothing: between the
// read and the write the lease stopped being the one the caller was looking at.
func reclaimRefused(task *core.Task, holder string) error {
	return core.Conflict("the lease on task %q is no longer the one held by %q; read it again",
		task.Ref, holder).WithDetail("claimed_by", holder)
}

// reclaimTarget loads the task a forced reclaim addresses, under the reclaim
// authority rather than the claim authority claimTarget checks: holding
// task:reclaim over a project does not imply holding task:claim there.
func (l *Local) reclaimTarget(ctx context.Context, tx store.Tx, ref core.TaskRef) (*core.Task, error) {
	task, err := tx.GetTask(ctx, ref)
	if err != nil {
		return nil, err
	}
	if task.Deleted() {
		return nil, core.NotFound("task %q", ref.String())
	}
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := l.policy.Can(actor, authz.ActionTaskReclaim, authz.Resource{
		TenantID:  actor.TenantID,
		ProjectID: task.ProjectID,
		OwnerID:   task.CreatorActorID,
	}); err != nil {
		return nil, err
	}
	return task, nil
}
