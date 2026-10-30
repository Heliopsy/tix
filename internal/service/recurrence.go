// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"maps"

	"github.com/heliopsy/tix/internal/core"
)

// spawnsOnMove reports whether moving task into state to finishes an
// occurrence that has not yet produced its successor. Cancelling does not:
// a terminal state whose category is not done abandons the series.
func spawnsOnMove(def core.WorkflowDefinition, task *core.Task, to string) bool {
	return task.Recurrence != "" && task.RecurrenceNextID == "" && def.CompletesWork(to)
}

// spawnOccurrence inserts the next occurrence of a task this mutation is
// finishing, inside the mutation's transaction, and links the finished task
// to it. It returns nil when the move does not finish an occurrence or the
// rule has run out. The caller writes the finished task, which carries the
// link, and then records the successor with recordOccurrence, so the stream
// reads the finish before the creation it caused.
func spawnOccurrence(ctx context.Context, m *mutation, def core.WorkflowDefinition, task *core.Task, to string) (*core.Task, error) {
	if !spawnsOnMove(def, task, to) {
		return nil, nil
	}
	rule, err := core.ParseRecurrence(task.Recurrence)
	if err != nil {
		return nil, core.Internal("task %q stores a recurrence rule that does not parse", task.Ref).Wrap(err)
	}
	due, ok := rule.Next(task.DueAt, m.now)
	if !ok {
		return nil, nil
	}
	next := &core.Task{
		ProjectID:       task.ProjectID,
		ParentID:        task.ParentID,
		Title:           task.Title,
		Body:            task.Body,
		Status:          def.Initial,
		Priority:        task.Priority,
		AssigneeActorID: task.AssigneeActorID,
		CreatorActorID:  m.actor.ID,
		DueAt:           &due,
		CustomFields:    maps.Clone(task.CustomFields),
		Recurrence:      rule.Successor().String(),
	}
	if def.IsTerminal(next.Status) {
		completed := m.now
		next.CompletedAt = &completed
	}
	if err := m.tx.CreateTask(ctx, next); err != nil {
		return nil, err
	}
	for _, name := range task.Tags {
		if _, err := attachTaskLabel(ctx, m.tx, next.ID, name); err != nil {
			return nil, err
		}
	}
	task.RecurrenceNextID = next.ID
	return next, nil
}

// recordOccurrence writes the audit entry and the ordinary task.created event
// for a spawned occurrence, naming the task it recurs from.
func recordOccurrence(m *mutation, from, next *core.Task) error {
	if next == nil {
		return nil
	}
	return m.Record("task.create", core.EventTaskCreated, "task", next.ID, next.ProjectID, nil, next,
		map[string]any{"ref": next.Ref, "title": next.Title, "status": next.Status, "recurs_from": from.Ref})
}

// withNext adds the successor's reference to a finishing move's payload.
func withNext(payload map[string]any, next *core.Task) map[string]any {
	if next != nil {
		payload["next"] = next.Ref
	}
	return payload
}
