// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
)

// moveTask puts one task into another project under the target's next number,
// keeping its identifier and everything keyed by it, and keeps its old number
// as an alias so the old reference still resolves.
func (l *Local) moveTask(ctx context.Context, ref core.TaskRef, in core.MoveTaskInput) (*core.TaskMove, error) {
	in.ProjectRef = strings.TrimSpace(in.ProjectRef)
	actor, err := l.authorize(ctx, authz.ActionTaskUpdate, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	var out *core.TaskMove
	err = l.write(ctx, actor, func(m *mutation) error {
		source, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskUpdate, source); err != nil {
			return err
		}
		from, err := m.tx.GetProject(ctx, source.ProjectID)
		if err != nil {
			return err
		}
		target, err := lookupProject(ctx, m.tx, in.ProjectRef)
		if err != nil {
			return core.OnField(err, "project_ref")
		}
		if err := checkMoveTarget(source, target); err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{ProjectID: target.ID}); err != nil {
			return err
		}
		if source.ClaimedAtTime(m.now) && source.ClaimedByActorID != m.actor.ID {
			return core.Conflict("task %q is claimed by somebody else; it moves once the claim is released",
				source.Ref).WithDetail("claimed_by", source.ClaimedByActorID)
		}
		out, err = l.applyMove(ctx, m, source, from, target)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// checkMoveTarget refuses a move to where the task already is or to a project
// whose tasks no list shows.
func checkMoveTarget(source *core.Task, target *core.Project) error {
	if target.ID == source.ProjectID {
		return core.Invalid("task %q is already in project %q", source.Ref, target.Key).
			WithDetail(core.DetailField, "project_ref")
	}
	if target.Archived() {
		return core.Precondition("project %q is archived; a task moved there would vanish from every list",
			target.Key).WithDetail(core.DetailField, "project_ref")
	}
	return nil
}

// applyMove renumbers the task into the target, maps its status and fields,
// rehomes its project-scoped tags, keeps the old number as an alias and
// records the move in both projects.
func (l *Local) applyMove(ctx context.Context, m *mutation, source *core.Task, from, target *core.Project) (*core.TaskMove, error) {
	wf, err := m.tx.GetWorkflowByID(ctx, target.WorkflowID)
	if err != nil {
		return nil, err
	}
	defs, err := m.tx.ListFieldDefs(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	fields, dropped, err := moveFields(defs, source.CustomFields)
	if err != nil {
		return nil, err
	}
	status, kept := source.Status, wf.Definition.HasState(source.Status)
	if !kept {
		status = wf.Definition.Initial
	}
	seq, err := m.tx.NextSeq(ctx, target.ID)
	if err != nil {
		return nil, err
	}
	if err := m.tx.MoveTask(ctx, source.ID, target.ID, seq, status); err != nil {
		return nil, err
	}
	if err := m.tx.AddTaskAlias(ctx, source.ProjectID, source.Seq, source.ID); err != nil {
		return nil, err
	}
	scoped, err := projectTags(ctx, m.tx, source.ProjectID)
	if err != nil {
		return nil, err
	}
	if err := rehomeTags(ctx, m.tx, source, scoped, target.ID, map[string]*core.Tag{}); err != nil {
		return nil, err
	}
	moved, err := m.tx.GetTask(ctx, core.TaskRef{ID: source.ID})
	if err != nil {
		return nil, err
	}
	moved.CustomFields = fields
	moved.CompletedAt = nil
	if wf.Definition.IsTerminal(status) {
		moved.CompletedAt = source.CompletedAt
		if moved.CompletedAt == nil {
			done := m.now
			moved.CompletedAt = &done
		}
	}
	if err := m.tx.UpdateTask(ctx, moved); err != nil {
		return nil, err
	}
	after, err := m.tx.GetTask(ctx, core.TaskRef{ID: source.ID})
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"ref": after.Ref, "from_ref": source.Ref,
		"project": target.Key, "from_project": from.Key,
		"status": status, "from_status": source.Status, "status_kept": kept,
	}
	if len(dropped) > 0 {
		payload["dropped_fields"] = dropped
	}
	if err := m.Record("task.move", core.EventTaskUpdated, "task", after.ID, target.ID, source, after, payload); err != nil {
		return nil, err
	}
	m.Event(core.EventTaskUpdated, "task", after.ID, from.ID, payload)
	return &core.TaskMove{
		Task: *after, FromRef: source.Ref, FromProject: from.Key, Project: target.Key,
		FromStatus: source.Status, StatusKept: kept, DroppedFields: dropped,
	}, nil
}

// moveFields keeps the custom field values the target defines and accepts,
// names the ones it leaves behind, then applies the target's defaults and
// required fields as a create does.
func moveFields(defs []core.FieldDef, values map[string]any) (map[string]any, []string, error) {
	byKey := make(map[string]core.FieldDef, len(defs))
	for _, d := range defs {
		byKey[d.Key] = d
	}
	kept := map[string]any{}
	var dropped []string
	for k, v := range values {
		if v == nil {
			continue
		}
		def, ok := byKey[k]
		if !ok {
			dropped = append(dropped, k)
			continue
		}
		coerced, err := coerceFieldValue(def, v)
		if err != nil {
			dropped = append(dropped, k)
			continue
		}
		kept[k] = coerced
	}
	slices.Sort(dropped)
	fields, err := validateTaskFields(defs, kept, true)
	if err != nil {
		return nil, nil, err
	}
	return fields, dropped, nil
}
