// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"cmp"
	"context"
	"slices"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// maxProjectTasks bounds how many tasks one project deletion moves or deletes
// inside its single write transaction.
const maxProjectTasks = maxSubtreeSize

// deletionScope is a project deletion resolved against the store: the project,
// its tasks in sequence order, and where they go.
type deletionScope struct {
	project *core.Project
	tasks   []core.Task
	target  *core.Project
	from    core.WorkflowDefinition
	to      core.WorkflowDefinition
}

// PlanProjectDeletion reports what DeleteProject would do with the same input,
// writing nothing.
func (l *Local) PlanProjectDeletion(ctx context.Context, ref string, in core.DeleteProjectInput) (*core.ProjectDeletion, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out *core.ProjectDeletion
	err = l.read(ctx, actor, func(tx store.Tx) error {
		s, err := l.resolveDeletion(ctx, tx, ref, in, authz.ActionProjectRead)
		if err != nil {
			return err
		}
		out = s.summary(in)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteProject removes a project permanently. A project holding tasks, soft
// deleted ones included, needs the input to say whether they move to another
// project or go with it; every task moved or deleted is recorded on its own.
func (l *Local) DeleteProject(ctx context.Context, ref string, in core.DeleteProjectInput) (*core.ProjectDeletion, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out *core.ProjectDeletion
	err = l.write(ctx, actor, func(m *mutation) error {
		s, err := l.resolveDeletion(ctx, m.tx, ref, in, authz.ActionProjectWrite)
		if err != nil {
			return err
		}
		out = s.summary(in)
		if len(s.tasks) > 0 {
			switch {
			case s.target != nil:
				err = l.moveProjectTasks(ctx, m, s)
			case in.DeleteTasks:
				err = recordProjectTaskDeletions(m, s)
			default:
				err = core.Precondition("project %q holds %d task(s); move them to another project or delete them with it",
					s.project.Key, len(s.tasks)).WithDetail("tasks", len(s.tasks))
			}
			if err != nil {
				return err
			}
		}
		if err := m.tx.DeleteProject(ctx, s.project.ID); err != nil {
			return err
		}
		payload := map[string]any{"key": s.project.Key, "tasks": len(s.tasks)}
		if s.target != nil && len(s.tasks) > 0 {
			payload["moved_to"] = s.target.Key
		}
		if in.DeleteTasks && len(s.tasks) > 0 {
			payload["deleted_tasks"] = len(s.tasks)
		}
		return m.Record(auditProjectDelete, eventProjectDeleted, "project", s.project.ID, s.project.ID,
			s.project, nil, payload)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveDeletion finds the project, authorises the action on it, walks its
// tasks and resolves the move target, refusing a target the tasks cannot go to.
func (l *Local) resolveDeletion(ctx context.Context, tx store.Tx, ref string, in core.DeleteProjectInput,
	action authz.Action) (*deletionScope, error) {
	p, err := lookupProject(ctx, tx, ref)
	if err != nil {
		return nil, err
	}
	if _, err := l.authorize(ctx, action, authz.Resource{ProjectID: p.ID}); err != nil {
		return nil, err
	}
	s := &deletionScope{project: p}
	if err := eachTask(ctx, tx, p.ID, true, func(t core.Task) error {
		if len(s.tasks) >= maxProjectTasks {
			return core.Precondition("project %q holds more than %d tasks; move or delete some of them first",
				p.Key, maxProjectTasks)
		}
		s.tasks = append(s.tasks, t)
		return nil
	}); err != nil {
		return nil, err
	}
	if in.MoveTasksTo == "" {
		return s, nil
	}
	target, err := lookupProject(ctx, tx, in.MoveTasksTo)
	if err != nil {
		return nil, err
	}
	if target.ID == p.ID {
		return nil, core.Invalid("tasks cannot move to the project being deleted").
			WithDetail(core.DetailField, "move_tasks_to")
	}
	if target.Archived() {
		return nil, core.Precondition("project %q is archived; moved tasks would vanish from every list",
			target.Key).WithDetail(core.DetailField, "move_tasks_to")
	}
	if _, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{ProjectID: target.ID}); err != nil {
		return nil, err
	}
	from, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
	if err != nil {
		return nil, err
	}
	to, err := tx.GetWorkflowByID(ctx, target.WorkflowID)
	if err != nil {
		return nil, err
	}
	s.target, s.from, s.to = target, from.Definition, to.Definition
	return s, nil
}

// summary states the deletion's reach in the shape every surface reports.
func (s *deletionScope) summary(in core.DeleteProjectInput) *core.ProjectDeletion {
	out := &core.ProjectDeletion{Project: s.project.Key, DeleteTasks: in.DeleteTasks}
	at := map[string]int{}
	for _, t := range s.tasks {
		if t.DeletedAt != nil {
			out.Deleted++
		} else {
			out.Tasks++
		}
		if s.target == nil {
			continue
		}
		i, seen := at[t.Status]
		if !seen {
			i = len(out.Statuses)
			at[t.Status] = i
			out.Statuses = append(out.Statuses, core.StatusMove{From: t.Status, To: mapStatus(s.from, s.to, t.Status)})
		}
		out.Statuses[i].Tasks++
	}
	if s.target != nil {
		out.MoveTo = s.target.Key
		order := map[string]int{}
		for i, st := range s.from.States {
			order[st.Key] = i
		}
		slices.SortStableFunc(out.Statuses, func(a, b core.StatusMove) int {
			return cmp.Compare(order[a.From], order[b.From])
		})
	}
	return out
}

// mapStatus places a status of one workflow in another: the same state when
// the target defines it, else the target's first state of the same category,
// else the target's initial state.
func mapStatus(from, to core.WorkflowDefinition, status string) string {
	if to.HasState(status) {
		return status
	}
	if category := from.CategoryOf(status); category != "" {
		for _, st := range to.States {
			if to.CategoryOf(st.Key) == category {
				return st.Key
			}
		}
	}
	return to.Initial
}

// moveProjectTasks renumbers every task into the target in sequence order,
// maps its status and keeps project-scoped tags it carries alive in the target.
func (l *Local) moveProjectTasks(ctx context.Context, m *mutation, s *deletionScope) error {
	scoped, err := projectTags(ctx, m.tx, s.project.ID)
	if err != nil {
		return err
	}
	rehomed := map[string]*core.Tag{}
	for i := range s.tasks {
		t := &s.tasks[i]
		before := *t
		seq, err := m.tx.NextSeq(ctx, s.target.ID)
		if err != nil {
			return err
		}
		status := mapStatus(s.from, s.to, t.Status)
		if err := m.tx.MoveTask(ctx, t.ID, s.target.ID, seq, status); err != nil {
			return err
		}
		for _, name := range t.Tags {
			old, ok := scoped[name]
			if !ok {
				continue
			}
			tag, ok := rehomed[name]
			if !ok {
				tag = &core.Tag{ProjectID: s.target.ID, Name: old.Name, Color: old.Color}
				if err := m.tx.PutTag(ctx, tag); err != nil {
					return err
				}
				rehomed[name] = tag
			}
			if err := m.tx.AttachTag(ctx, t.ID, tag.ID); err != nil {
				return err
			}
		}
		after, err := m.tx.GetTask(ctx, core.TaskRef{ID: t.ID})
		if err != nil {
			return err
		}
		if err := m.Record("task.move", core.EventTaskUpdated, "task", t.ID, s.target.ID, before, after,
			map[string]any{
				"ref": after.Ref, "from_ref": before.Ref,
				"project": s.target.Key, "from_project": s.project.Key,
				"status": status, "from_status": before.Status,
			}); err != nil {
			return err
		}
	}
	return nil
}

// projectTags returns the tags scoped to one project, by name.
func projectTags(ctx context.Context, tx store.Tx, projectID string) (map[string]core.Tag, error) {
	tags, err := tx.ListTags(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]core.Tag{}
	for _, t := range tags {
		if t.ProjectID == projectID {
			out[t.Name] = t
		}
	}
	return out, nil
}

// recordProjectTaskDeletions writes the audit entry and event for each task
// the project's removal takes with it, which the store's cascade then deletes.
func recordProjectTaskDeletions(m *mutation, s *deletionScope) error {
	for i := range s.tasks {
		t := &s.tasks[i]
		if err := m.Record("task.delete", core.EventTaskDeleted, "task", t.ID, t.ProjectID, t, nil,
			map[string]any{"ref": t.Ref, "hard": true, "project_deleted": s.project.Key}); err != nil {
			return err
		}
	}
	return nil
}
