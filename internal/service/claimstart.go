// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// startClaim moves a task just claimed to the state its work starts in, under
// the claim's own token and in the claim's transaction, so a refusal leaves
// nothing claimed rather than a worker holding a task it cannot start.
func (l *Local) startClaim(ctx context.Context, m *mutation, claim *core.Claim, named string) error {
	def, err := workflowForTask(ctx, m.tx, claim.Task)
	if err != nil {
		return err
	}
	to, err := startTarget(def, claim.Task, named)
	if err != nil || to == "" {
		return err
	}
	from := claim.Task.Status
	task := *claim.Task
	moved, err := l.applyTransition(ctx, m, &task, core.TransitionInput{To: to, LeaseToken: claim.LeaseToken})
	if err != nil {
		return err
	}
	claim.Task, claim.StartedFrom = moved, from
	return nil
}

// startTarget is the state a starting claim moves the task to, or empty when
// it is already there. A named state is taken as given and its edge is judged
// by the transition; with none named, a task already in progress stays put.
func startTarget(def core.WorkflowDefinition, task *core.Task, named string) (string, error) {
	if named != "" {
		if !def.HasState(named) {
			return "", def.UnknownState(named)
		}
		if named == task.Status {
			return "", nil
		}
		return named, nil
	}
	if def.CategoryOf(task.Status) == core.CategoryInProgress {
		return "", nil
	}
	if to, ok := def.StartState(task.Status); ok {
		return to, nil
	}
	return "", core.Precondition("task %q has no in-progress state to start in from %q; "+
		"allowed targets are %s, name one as the start status", task.Ref, task.Status,
		strings.Join(allowedTargets(def, task.Status), ", "))
}
