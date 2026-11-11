// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// auditTaskMove is the audit action of a task that changed project.
const auditTaskMove = "task.move"

// taskMover moves tasks from one project into another inside one mutation:
// the target's next number, the status mapped across the two workflows, and
// the source's scoped tags re-created under the target once each, or the
// target's own tag of that name used as it is.
type taskMover struct {
	m        *mutation
	source   *core.Project
	target   *core.Project
	from, to core.WorkflowDefinition
	scoped   map[string]core.Tag
	rehomed  map[string]*core.Tag
}

// newTaskMover prepares a move between two resolved projects.
func newTaskMover(ctx context.Context, m *mutation, source, target *core.Project,
	from, to core.WorkflowDefinition) (*taskMover, error) {
	scoped, err := projectTags(ctx, m.tx, source.ID)
	if err != nil {
		return nil, err
	}
	existing, err := projectTags(ctx, m.tx, target.ID)
	if err != nil {
		return nil, err
	}
	rehomed := make(map[string]*core.Tag, len(existing))
	for name, tag := range existing {
		rehomed[name] = &tag
	}
	return &taskMover{
		m: m, source: source, target: target, from: from, to: to,
		scoped: scoped, rehomed: rehomed,
	}, nil
}

// move puts one task into the target, detaching it from its parent first
// when asked, and records it. It returns the task as it landed.
func (mv *taskMover) move(ctx context.Context, t *core.Task, detach bool) (*core.Task, error) {
	before := *t
	var fromParent string
	if detach && t.ParentID != "" {
		if parent, err := mv.m.tx.GetTask(ctx, core.TaskRef{ID: t.ParentID}); err == nil {
			fromParent = parent.Ref
		} else {
			fromParent = t.ParentID
		}
		detached := *t
		detached.ParentID = ""
		if err := mv.m.tx.UpdateTask(ctx, &detached); err != nil {
			return nil, err
		}
	}
	seq, err := mv.m.tx.NextSeq(ctx, mv.target.ID)
	if err != nil {
		return nil, err
	}
	status := mapStatus(mv.from, mv.to, t.Status)
	if err := mv.m.tx.MoveTask(ctx, t.ID, mv.target.ID, seq, status); err != nil {
		return nil, err
	}
	if err := mv.rehomeTags(ctx, t); err != nil {
		return nil, err
	}
	after, err := mv.m.tx.GetTask(ctx, core.TaskRef{ID: t.ID})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"ref": after.Ref, "from_ref": before.Ref,
		"project": mv.target.Key, "from_project": mv.source.Key,
		"status": status, "from_status": before.Status,
	}
	if fromParent != "" {
		payload["from_parent"] = fromParent
	}
	if err := mv.m.Record(auditTaskMove, core.EventTaskUpdated, "task", t.ID, mv.target.ID, before, after,
		payload); err != nil {
		return nil, err
	}
	return after, nil
}

// rehomeTags puts each source-scoped tag a moved task carries back on it as
// the target's tag of the same name and colour.
func (mv *taskMover) rehomeTags(ctx context.Context, t *core.Task) error {
	for _, name := range t.Tags {
		old, ok := mv.scoped[name]
		if !ok {
			continue
		}
		tag, ok := mv.rehomed[name]
		if !ok {
			tag = &core.Tag{ProjectID: mv.target.ID, Name: old.Name, Color: old.Color}
			if err := mv.m.tx.PutTag(ctx, tag); err != nil {
				return err
			}
			mv.rehomed[name] = tag
		}
		if err := mv.m.tx.AttachTag(ctx, t.ID, tag.ID); err != nil {
			return err
		}
	}
	return nil
}

// moveTask moves a task and its live subtree in the same project to another
// project, detaching the task from its parent, in one transaction.
func (l *Local) moveTask(ctx context.Context, ref core.TaskRef, in core.MoveTaskInput) (*core.Task, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return nil, err
	}
	in.ProjectRef = strings.TrimSpace(in.ProjectRef)
	if err := in.Validate(); err != nil {
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
		if err := checkVersion(task, in.Version); err != nil {
			return err
		}
		mv, err := l.resolveTaskMove(ctx, m, task, in.ProjectRef)
		if err != nil {
			return err
		}
		tasks := []core.Task{*task}
		if err := appendDescendants(ctx, m.tx, task.ID, 0, 1, &tasks); err != nil {
			return err
		}
		moving := tasks[:0]
		for _, t := range tasks {
			if t.ProjectID != task.ProjectID {
				continue
			}
			if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, &t); err != nil {
				return err
			}
			moving = append(moving, t)
		}
		for i := range moving {
			moved, err := mv.move(ctx, &moving[i], i == 0)
			if err != nil {
				return err
			}
			if i == 0 {
				out = moved
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// resolveTaskMove finds the project a task moves to, refusing its own, an
// archived one or one the caller may not update tasks in.
func (l *Local) resolveTaskMove(ctx context.Context, m *mutation, task *core.Task, ref string) (*taskMover, error) {
	target, err := lookupProject(ctx, m.tx, ref)
	if err != nil {
		return nil, core.OnField(err, "project_ref")
	}
	if target.ID == task.ProjectID {
		return nil, core.Invalid("task %q is already in project %q", task.Ref, target.Key).
			WithDetail(core.DetailField, "project_ref")
	}
	if target.Archived() {
		return nil, core.Precondition("project %q is archived; a moved task would vanish from every list",
			target.Key).WithDetail(core.DetailField, "project_ref")
	}
	if _, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{ProjectID: target.ID}); err != nil {
		return nil, err
	}
	source, err := lookupProject(ctx, m.tx, task.ProjectID)
	if err != nil {
		return nil, err
	}
	from, err := workflowOf(ctx, m.tx, source)
	if err != nil {
		return nil, err
	}
	to, err := workflowOf(ctx, m.tx, target)
	if err != nil {
		return nil, err
	}
	return newTaskMover(ctx, m, source, target, from, to)
}

// workflowOf reads the definition of a project's workflow.
func workflowOf(ctx context.Context, tx store.Tx, p *core.Project) (core.WorkflowDefinition, error) {
	wf, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
	if err != nil {
		return core.WorkflowDefinition{}, err
	}
	return wf.Definition, nil
}
