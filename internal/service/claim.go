// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/store"
)

// claimNextAttempts is how many times claim next retries its compare-and-swap
// before reporting an empty queue. A lost race is not an empty queue.
const claimNextAttempts = 3

// ClaimTask takes a lease on one task.
func (l *Local) ClaimTask(ctx context.Context, ref core.TaskRef, in core.ClaimInput) (*core.Claim, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskClaim, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Claim
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := l.claimTarget(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		wf, err := claimWorkflow(ctx, m.tx, task.ProjectID)
		if err != nil {
			return err
		}
		if wf.Definition.IsTerminal(task.Status) {
			return finishedTask(task)
		}
		ttl, err := lease.Resolve(in.TTL, wf.Definition.DefaultLease)
		if err != nil {
			return err
		}
		holder, err := claimHolder(ctx, m.tx, actor, in.ActorID)
		if err != nil {
			return err
		}
		token, err := lease.NewToken()
		if err != nil {
			return err
		}

		until := m.now.Add(ttl)
		ok, err := m.tx.ClaimTask(ctx, store.ClaimRow{
			TaskID:     task.ID,
			ActorID:    holder,
			Now:        m.now,
			Until:      until,
			LeaseToken: token,
		})
		if err != nil {
			return err
		}
		if !ok {
			return claimRefused(task, holder, m.now)
		}

		out, err = l.recordClaim(ctx, m, task, token, until)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ClaimNext claims the highest-priority unblocked task matching the filter.
func (l *Local) ClaimNext(ctx context.Context, in core.ClaimNextInput) (*core.Claim, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskClaim, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if _, err := lease.Resolve(in.TTL, 0); err != nil {
		return nil, err
	}

	var out *core.Claim
	for attempt := 0; attempt < claimNextAttempts && out == nil; attempt++ {
		if err := l.write(ctx, actor, func(m *mutation) error {
			claimed, err := l.claimNextOnce(ctx, m, actor, in)
			out = claimed
			return err
		}); err != nil {
			return nil, err
		}
	}
	if out == nil {
		return nil, core.NoTaskAvailable("no eligible task is available to claim")
	}
	return out, nil
}

// claimNextOnce performs one compare-and-swap over the queue. It returns a nil
// claim, and no error, when another worker won the row.
func (l *Local) claimNextOnce(ctx context.Context, m *mutation, actor *core.Actor, in core.ClaimNextInput) (*core.Claim, error) {
	projectIDs, err := l.claimScope(ctx, m.tx, actor, in.ProjectRefs)
	if err != nil {
		return nil, err
	}
	terminal, err := claimTerminalStates(ctx, m.tx, projectIDs)
	if err != nil {
		return nil, err
	}
	holder, err := claimHolder(ctx, m.tx, actor, in.ActorID)
	if err != nil {
		return nil, err
	}
	token, err := lease.NewToken()
	if err != nil {
		return nil, err
	}
	ttl, err := lease.Resolve(in.TTL, 0)
	if err != nil {
		return nil, err
	}

	until := m.now.Add(ttl)
	taskID, ok, err := m.tx.ClaimNextTask(ctx, store.ClaimNextRow{
		ProjectIDs:     projectIDs,
		Tags:           in.Tags,
		Statuses:       in.Statuses,
		TerminalStates: terminal,
		ActorID:        holder,
		Now:            m.now,
		Until:          until,
		LeaseToken:     token,
	})
	if err != nil || !ok {
		return nil, err
	}

	task, err := m.tx.GetTask(ctx, core.TaskRef{ID: taskID})
	if err != nil {
		return nil, err
	}
	until, err = l.applyWorkflowTTL(ctx, m, task, in.TTL, token, until)
	if err != nil {
		return nil, err
	}
	return l.recordClaim(ctx, m, task, token, until)
}

// applyWorkflowTTL narrows a queue claim's lease to the workflow's own default,
// which is only known once the compare-and-swap has picked a task.
func (l *Local) applyWorkflowTTL(ctx context.Context, m *mutation, task *core.Task, perClaim core.Duration, token string, until time.Time) (time.Time, error) {
	if perClaim != 0 {
		return until, nil
	}
	wf, err := claimWorkflow(ctx, m.tx, task.ProjectID)
	if err != nil {
		return until, err
	}
	if wf.Definition.DefaultLease == 0 {
		return until, nil
	}
	ttl, err := lease.Resolve(0, wf.Definition.DefaultLease)
	if err != nil {
		return until, err
	}
	revised := m.now.Add(ttl)
	if _, err := m.tx.RenewLease(ctx, task.ID, token, revised); err != nil {
		return until, err
	}
	return revised, nil
}

// RenewLease extends a live lease held under the caller's token.
func (l *Local) RenewLease(ctx context.Context, ref core.TaskRef, token string, ttl core.Duration) (*core.Claim, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskClaim, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Claim
	err = l.write(ctx, actor, func(m *mutation) error {
		task, err := l.claimTarget(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if token == "" {
			return core.Invalid("renewing the lease on task %q requires its lease token", task.Ref)
		}
		wf, err := claimWorkflow(ctx, m.tx, task.ProjectID)
		if err != nil {
			return err
		}
		resolved, err := lease.Resolve(ttl, wf.Definition.DefaultLease)
		if err != nil {
			return err
		}

		until := m.now.Add(resolved)
		ok, err := m.tx.RenewLease(ctx, task.ID, token, until)
		if err != nil {
			return err
		}
		if !ok {
			return staleLease(task)
		}

		renewed, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		if err := m.Record("task.lease_renew", core.EventTaskUpdated, "task", task.ID, task.ProjectID,
			task, renewed, map[string]any{"lease_expires_at": until}); err != nil {
			return err
		}
		out = &core.Claim{Task: renewed, LeaseToken: token, LeaseExpiresAt: until}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ReleaseLease gives up a lease, optionally with a final status and a result.
func (l *Local) ReleaseLease(ctx context.Context, ref core.TaskRef, token string, in core.ReleaseInput) error {
	actor, err := l.authorize(ctx, authz.ActionTaskClaim, authz.Resource{})
	if err != nil {
		return err
	}
	if in.Status != "" {
		if _, err := l.authorize(ctx, authz.ActionTaskTransition, authz.Resource{}); err != nil {
			return err
		}
	}
	if len(in.Result) > 0 {
		if _, err := l.authorize(ctx, authz.ActionArtifactWrite, authz.Resource{}); err != nil {
			return err
		}
	}

	return l.write(ctx, actor, func(m *mutation) error {
		task, err := l.claimTarget(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if token == "" {
			return core.Invalid("releasing task %q requires its lease token", task.Ref)
		}
		wf, err := claimWorkflow(ctx, m.tx, task.ProjectID)
		if err != nil {
			return err
		}
		if err := releasableStatus(wf, task, actor, in); err != nil {
			return err
		}

		ok, err := m.tx.ReleaseLease(ctx, task.ID, token)
		if err != nil {
			return err
		}
		if !ok {
			return staleLease(task)
		}
		if err := l.applyRelease(ctx, m, task, wf, in); err != nil {
			return err
		}

		released, err := m.tx.GetTask(ctx, core.TaskRef{ID: task.ID})
		if err != nil {
			return err
		}
		payload := map[string]any{"status": released.Status}
		if in.Status != "" {
			payload["final_status"] = in.Status
		}
		return m.Record("task.release", core.EventTaskReleased, "task", task.ID, task.ProjectID,
			task, released, payload)
	})
}

// applyRelease writes the final status, the result artifact and the closing
// comment a release carries.
func (l *Local) applyRelease(ctx context.Context, m *mutation, task *core.Task, wf *core.Workflow, in core.ReleaseInput) error {
	if in.Comment != "" {
		if err := m.tx.CreateComment(ctx, &core.Comment{
			TaskID:        task.ID,
			AuthorActorID: m.actor.ID,
			Body:          in.Comment,
		}); err != nil {
			return err
		}
	}
	if len(in.Result) > 0 {
		if err := m.tx.PutArtifact(ctx, &core.Artifact{
			TaskID:  task.ID,
			ActorID: m.actor.ID,
			Kind:    core.ArtifactResult,
			Name:    "result",
			Payload: in.Result,
		}); err != nil {
			return err
		}
	}
	if in.Status == "" {
		return nil
	}

	updated := *task
	updated.Status = in.Status
	if wf.Definition.IsTerminal(in.Status) {
		completed := m.now
		updated.CompletedAt = &completed
	}
	if err := m.tx.UpdateTask(ctx, &updated); err != nil {
		return err
	}
	return m.Record("task.transition", core.EventTaskTransitioned, "task", task.ID, task.ProjectID,
		task, updated, map[string]any{"from": task.Status, "to": in.Status})
}

// SweepLeases materializes expired leases, reporting how many it cleared.
func (l *Local) SweepLeases(ctx context.Context, limit int) (int, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return 0, err
	}

	swept := 0
	err = l.write(ctx, actor, func(m *mutation) error {
		expired, err := m.tx.ExpiredLeases(ctx, m.now, limit)
		if err != nil {
			return err
		}
		workflows := make(map[string]*core.Workflow, 2)
		for i := range expired {
			if err := l.sweepOne(ctx, m, expired[i], workflows); err != nil {
				return err
			}
			swept++
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return swept, nil
}

// sweepOne clears one expired claim and reverts the status when the workflow's
// state asks for it.
func (l *Local) sweepOne(ctx context.Context, m *mutation, before core.Task, workflows map[string]*core.Workflow) error {
	if err := m.tx.ClearClaim(ctx, before.ID); err != nil {
		return err
	}

	wf, ok := workflows[before.ProjectID]
	if !ok {
		loaded, err := claimWorkflow(ctx, m.tx, before.ProjectID)
		if err != nil {
			return err
		}
		workflows[before.ProjectID] = loaded
		wf = loaded
	}

	payload := map[string]any{"previous_holder": before.ClaimedByActorID}
	if state, found := wf.Definition.State(before.Status); found && state.RevertOnLeaseExpiry {
		to := state.RevertTo
		if to == "" {
			to = wf.Definition.Initial
		}
		if to != before.Status && wf.Definition.HasState(to) {
			reverted := before
			reverted.Status = to
			reverted.StartedAt = nil
			if err := m.tx.UpdateTask(ctx, &reverted); err != nil {
				return err
			}
			payload["reverted_to"] = to
		}
	}

	after, err := m.tx.GetTask(ctx, core.TaskRef{ID: before.ID})
	if err != nil {
		return err
	}
	return m.Record("task.lease_expire", core.EventTaskLeaseExpired, "task", before.ID, before.ProjectID,
		before, after, payload)
}

// requireLeaseToken rejects an operation on a claimed task whose token is not
// the current one. A task whose lease has expired reads as unclaimed and needs
// no token, which is what makes lazy expiry authoritative.
func requireLeaseToken(ctx context.Context, tx store.Tx, task *core.Task, token string, now time.Time) error {
	if !task.ClaimedAtTime(now) {
		if token != "" {
			return staleLease(task)
		}
		return nil
	}
	if token == "" {
		return core.Invalid("task %q is claimed; supply its lease token", task.Ref)
	}
	ok, err := tx.RenewLease(ctx, task.ID, token, *task.LeaseExpiresAt)
	if err != nil {
		return err
	}
	if !ok {
		return staleLease(task)
	}
	return nil
}

// claimTarget loads the task a lease operation addresses.
func (l *Local) claimTarget(ctx context.Context, tx store.Tx, ref core.TaskRef) (*core.Task, error) {
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
	if err := l.policy.Can(actor, authz.ActionTaskClaim, authz.Resource{
		TenantID:  actor.TenantID,
		ProjectID: task.ProjectID,
		OwnerID:   task.CreatorActorID,
	}); err != nil {
		return nil, err
	}
	return task, nil
}

// claimScope resolves the projects a queue claim may draw from.
func (l *Local) claimScope(ctx context.Context, tx store.Tx, actor *core.Actor, refs []string) ([]string, error) {
	if len(refs) == 0 {
		if actor.ScopedToProject() {
			return []string{actor.ProjectID}, nil
		}
		return nil, nil
	}

	ids := make([]string, 0, len(refs))
	for _, ref := range refs {
		project, err := lookupProject(ctx, tx, ref)
		if err != nil {
			return nil, err
		}
		if err := l.policy.Can(actor, authz.ActionTaskClaim, authz.Resource{
			TenantID:  actor.TenantID,
			ProjectID: project.ID,
		}); err != nil {
			return nil, err
		}
		ids = append(ids, project.ID)
	}
	return ids, nil
}

// claimTerminalStates returns the states that satisfy a dependency across every
// workflow a queue claim can reach.
func claimTerminalStates(ctx context.Context, tx store.Tx, projectIDs []string) ([]string, error) {
	workflows, err := claimWorkflows(ctx, tx, projectIDs)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool, 4)
	out := make([]string, 0, 4)
	for _, wf := range workflows {
		for _, state := range wf.Definition.TerminalStates() {
			if seen[state] {
				continue
			}
			seen[state] = true
			out = append(out, state)
		}
	}
	return out, nil
}

// claimWorkflows returns the workflows behind the named projects, or every
// workflow in the tenant when no project narrows the queue.
func claimWorkflows(ctx context.Context, tx store.Tx, projectIDs []string) ([]core.Workflow, error) {
	if len(projectIDs) == 0 {
		return tx.ListWorkflows(ctx)
	}
	out := make([]core.Workflow, 0, len(projectIDs))
	for _, id := range projectIDs {
		wf, err := claimWorkflow(ctx, tx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *wf)
	}
	return out, nil
}

// claimWorkflow returns the workflow governing a project.
func claimWorkflow(ctx context.Context, tx store.Tx, projectID string) (*core.Workflow, error) {
	project, err := tx.GetProject(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return tx.GetWorkflowByID(ctx, project.WorkflowID)
}

// claimHolder resolves who the lease is taken for. A claim on another actor's
// behalf is only accepted for an actor of this tenant.
func claimHolder(ctx context.Context, tx store.Tx, actor *core.Actor, onBehalfOf string) (string, error) {
	onBehalfOf = strings.TrimSpace(onBehalfOf)
	if onBehalfOf == "" || onBehalfOf == actor.ID {
		return actor.ID, nil
	}
	a, err := lookupActor(ctx, tx, onBehalfOf)
	if err != nil {
		return "", err
	}
	return a.ID, nil
}

// recordClaim reloads the claimed task and records the claim.
func (l *Local) recordClaim(ctx context.Context, m *mutation, before *core.Task, token string, until time.Time) (*core.Claim, error) {
	claimed, err := m.tx.GetTask(ctx, core.TaskRef{ID: before.ID})
	if err != nil {
		return nil, err
	}
	if err := m.Record("task.claim", core.EventTaskClaimed, "task", claimed.ID, claimed.ProjectID,
		before, claimed, map[string]any{
			"claimed_by":       claimed.ClaimedByActorID,
			"lease_expires_at": until,
		}); err != nil {
		return nil, err
	}
	return &core.Claim{Task: claimed, LeaseToken: token, LeaseExpiresAt: until}, nil
}

// claimRefused explains why a compare-and-swap did not take the lease.
func claimRefused(task *core.Task, holder string, now time.Time) error {
	if !task.ClaimedAtTime(now) {
		return core.Conflict("task %q was claimed by another worker first", task.Ref)
	}
	if task.ClaimedByActorID == holder {
		return core.Conflict("task %q is already held by this worker; renew the lease instead", task.Ref).
			WithDetail("claimed_by", task.ClaimedByActorID)
	}
	return core.Conflict("task %q is held by %q until %s", task.Ref, task.ClaimedByActorID,
		task.LeaseExpiresAt.Format(time.RFC3339)).
		WithDetail("claimed_by", task.ClaimedByActorID).
		WithDetail("lease_expires_at", task.LeaseExpiresAt)
}

// finishedTask refuses a lease on work the workflow already considers done.
func finishedTask(task *core.Task) error {
	return core.Conflict("task %q is already finished in status %q", task.Ref, task.Status).
		WithDetail("status", task.Status)
}

// staleLease reports a token that is no longer current, which a worker answers
// by claiming again rather than by retrying.
func staleLease(task *core.Task) error {
	return core.LeaseExpired("the lease on task %q is no longer held under that token; claim it again", task.Ref)
}

// releasableStatus validates the final status a release carries.
func releasableStatus(wf *core.Workflow, task *core.Task, actor *core.Actor, in core.ReleaseInput) error {
	if in.Status == "" {
		return nil
	}
	if !wf.Definition.HasState(in.Status) {
		return core.Invalid("status %q is not defined by workflow %q", in.Status, wf.Key)
	}
	if in.Status == task.Status {
		return core.Invalid("task %q is already in status %q", task.Ref, in.Status)
	}
	transition, ok := wf.Definition.CanTransition(task.Status, in.Status)
	if !ok {
		return core.Invalid("workflow %q has no transition from %q to %q", wf.Key, task.Status, in.Status)
	}
	if transition.RequiresComment && in.Comment == "" {
		return core.Invalid("moving to %q requires a comment", in.Status)
	}
	if transition.RequiresScope != "" && !actor.HasScope(transition.RequiresScope) {
		return core.Forbidden("moving to %q requires scope %q", in.Status, transition.RequiresScope)
	}
	return nil
}
