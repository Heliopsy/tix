// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"errors"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// bulkActionAuthority is the policy action each bulk action needs, checked
// once before any task is touched.
var bulkActionAuthority = map[core.BulkAction]authz.Action{
	core.BulkMove:      authz.ActionTaskTransition,
	core.BulkPriority:  authz.ActionTaskUpdate,
	core.BulkTagAdd:    authz.ActionTaskUpdate,
	core.BulkTagRemove: authz.ActionTaskUpdate,
	core.BulkAssign:    authz.ActionTaskUpdate,
	core.BulkStar:      authz.ActionTaskStar,
	core.BulkUnstar:    authz.ActionTaskStar,
	core.BulkCopy:      authz.ActionTaskCreate,
	core.BulkDelete:    authz.ActionTaskDelete,
	core.BulkRestore:   authz.ActionTaskDelete,
}

// BulkTasks applies one action to many tasks.
//
// Each task goes through the single-task method, so each is its own
// transaction with its own audit entry and event, and one refusal leaves the
// rest standing. A batch that rolled back whole on any refusal would need a
// second copy of every rule and would turn one locked task into nothing done.
func (l *Local) BulkTasks(ctx context.Context, in core.BulkTaskInput) (*core.BulkResult, error) {
	in, err := in.Normalize()
	if err != nil {
		return nil, err
	}
	if _, err := l.authorize(ctx, bulkActionAuthority[in.Action], authz.Resource{}); err != nil {
		return nil, err
	}
	out := &core.BulkResult{Action: in.Action, Outcomes: make([]core.BulkOutcome, 0, len(in.Refs))}
	for _, raw := range in.Refs {
		outcome := core.BulkOutcome{Ref: raw}
		copyRef, err := l.bulkOne(ctx, in, raw)
		if err != nil {
			outcome.Code, outcome.Error = string(core.KindOf(err)), outcomeMessage(err)
			out.Refused++
		} else {
			outcome.Applied, outcome.CopyRef = true, copyRef
			out.Applied++
		}
		out.Outcomes = append(out.Outcomes, outcome)
	}
	return out, nil
}

// bulkOne applies the action to one task, answering the new task's reference
// for a copy.
func (l *Local) bulkOne(ctx context.Context, in core.BulkTaskInput, raw string) (string, error) {
	ref, err := core.ParseTaskRef(raw)
	if err != nil {
		return "", err
	}
	switch in.Action {
	case core.BulkMove:
		return "", l.bulkMove(ctx, ref, in.To)
	case core.BulkPriority:
		priority := in.Priority
		_, err = l.UpdateTask(ctx, ref, core.UpdateTaskInput{Priority: &priority})
	case core.BulkAssign:
		assignee := in.AssigneeActorID
		_, err = l.UpdateTask(ctx, ref, core.UpdateTaskInput{AssigneeActorID: &assignee})
	case core.BulkTagAdd:
		err = l.AddTag(ctx, ref, in.Tag)
	case core.BulkTagRemove:
		err = l.RemoveTag(ctx, ref, in.Tag)
	case core.BulkStar:
		err = l.StarTask(ctx, ref)
	case core.BulkUnstar:
		err = l.UnstarTask(ctx, ref)
	case core.BulkDelete:
		err = l.DeleteTask(ctx, ref, core.DeleteTaskInput{Cascade: in.Cascade})
	case core.BulkRestore:
		_, err = l.RestoreTask(ctx, ref)
	case core.BulkCopy:
		copied, err := l.CopyTask(ctx, ref, core.CopyTaskInput{ProjectRef: in.ProjectRef, Link: true})
		if err != nil {
			return "", err
		}
		return copied.Ref, nil
	}
	return "", err
}

// bulkMove walks one task to a status along the shortest route its own
// workflow has. Where two routes are equally short the first one found is
// taken: a batch cannot stop to ask which, and either one is a legal journey
// the audit trail records hop by hop.
func (l *Local) bulkMove(ctx context.Context, ref core.TaskRef, to string) error {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return err
	}
	var task *core.Task
	var def core.WorkflowDefinition
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := liveTask(ctx, tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskTransition, found); err != nil {
			return err
		}
		task = found
		def, err = workflowForTask(ctx, tx, found)
		return err
	}); err != nil {
		return err
	}
	route, err := shortestRoute(def, task.Status, to)
	if err != nil {
		return err
	}
	result, err := l.TransitionRoute(ctx, core.TaskRef{ID: task.ID}, core.RouteInput{Route: route})
	if err != nil {
		return err
	}
	if result.Partial() {
		return core.Precondition("stopped at %q on the way to %q: %s", result.Stopped, to, result.Reason)
	}
	return nil
}

// shortestRoute is the first of the shortest routes from one state to another.
func shortestRoute(def core.WorkflowDefinition, from, to string) ([]string, error) {
	if from == to {
		return nil, core.Precondition("task is already in %q", to)
	}
	if !def.HasState(to) {
		return nil, core.Invalid("status %q is not a state of this project's workflow", to)
	}
	for _, r := range core.Routes(def, from) {
		if r.To.Key == to {
			return r.Keys(), nil
		}
	}
	return nil, core.Precondition("no route leads from %q to %q", from, to)
}

// outcomeMessage is what an outcome says about a refusal: the domain message,
// never an internal cause, which may name storage details a caller has no use
// for.
func outcomeMessage(err error) string {
	var domain *core.Error
	if errors.As(err, &domain) && domain.Kind != core.KindInternal {
		return domain.Message
	}
	return "internal error"
}
