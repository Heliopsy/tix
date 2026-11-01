// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
)

// CopyTask creates a copy of a task, with its subtasks, comments and
// dependencies when asked, in one transaction.
func (l *Local) CopyTask(ctx context.Context, ref core.TaskRef, in core.CopyTaskInput) (*core.Task, error) {
	in.Title = strings.TrimSpace(in.Title)
	in.ProjectRef = strings.TrimSpace(in.ProjectRef)
	if err := in.Validate(); err != nil {
		return nil, err
	}
	actor, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{})
	if err != nil {
		return nil, err
	}

	var out *core.Task
	err = l.write(ctx, actor, func(m *mutation) error {
		source, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, source); err != nil {
			return err
		}
		c, err := l.newCopier(ctx, m, source, in)
		if err != nil {
			return err
		}
		sources := []core.Task{*source}
		if in.WithSubtasks {
			if err := appendDescendants(ctx, m.tx, source.ID, 0, 1, &sources); err != nil {
				return err
			}
		}
		for i := range sources {
			if err := l.authorizeTask(ctx, authz.ActionTaskRead, &sources[i]); err != nil {
				return err
			}
			if err := c.copyOne(ctx, &sources[i], i == 0); err != nil {
				return err
			}
		}
		if in.WithDependencies {
			for i := range sources {
				if err := c.copyDependencies(ctx, &sources[i]); err != nil {
					return err
				}
			}
		}
		if err := c.record(ctx, sources); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: c.copies[source.ID].ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// copier carries what every task of one copy shares: the target project, its
// workflow and fields, and the copies made so far, keyed by source identifier.
type copier struct {
	l        *Local
	m        *mutation
	in       core.CopyTaskInput
	project  *core.Project
	workflow core.WorkflowDefinition
	defs     []core.FieldDef
	sameHome bool
	copies   map[string]*core.Task
	order    []string
	handles  map[string]string
}

// newCopier resolves the target project and checks the authority the options
// need there.
func (l *Local) newCopier(ctx context.Context, m *mutation, source *core.Task, in core.CopyTaskInput) (*copier, error) {
	target := in.ProjectRef
	if target == "" {
		target = source.ProjectID
	}
	project, err := m.tx.GetProject(ctx, target)
	if err != nil {
		return nil, core.OnField(err, "project_ref")
	}
	if _, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{ProjectID: project.ID}); err != nil {
		return nil, err
	}
	if in.WithComments {
		if _, err := l.authorize(ctx, authz.ActionCommentWrite, authz.Resource{ProjectID: project.ID}); err != nil {
			return nil, err
		}
	}
	wf, err := m.tx.GetWorkflowByID(ctx, project.WorkflowID)
	if err != nil {
		return nil, err
	}
	defs, err := m.tx.ListFieldDefs(ctx, project.ID)
	if err != nil {
		return nil, err
	}
	return &copier{
		l: l, m: m, in: in, project: project, workflow: wf.Definition, defs: defs,
		sameHome: project.ID == source.ProjectID,
		copies:   map[string]*core.Task{}, handles: map[string]string{},
	}, nil
}

// copyOne creates the copy of one source task, its tags and, when asked, its
// comments. root is the task the caller named; every other source is a
// subtask whose parent has already been copied.
func (c *copier) copyOne(ctx context.Context, src *core.Task, root bool) error {
	fields, err := c.fields(src.CustomFields)
	if err != nil {
		return fmt.Errorf("copying %s: %w", src.Ref, err)
	}
	seq, err := c.m.tx.NextSeq(ctx, c.project.ID)
	if err != nil {
		return err
	}
	task := &core.Task{
		ProjectID:       c.project.ID,
		Seq:             seq,
		ParentID:        c.parentOf(src, root),
		Title:           src.Title,
		Body:            src.Body,
		Status:          c.status(src.Status),
		Priority:        src.Priority,
		AssigneeActorID: src.AssigneeActorID,
		CreatorActorID:  c.m.actor.ID,
		DueAt:           src.DueAt,
		CustomFields:    fields,
		Recurrence:      src.Recurrence,
	}
	if root && c.in.Title != "" {
		task.Title = c.in.Title
	}
	if c.in.Link {
		task.CopiedFromID = src.ID
	}
	if c.workflow.IsTerminal(task.Status) {
		completed := c.m.now
		task.CompletedAt = &completed
	}
	if err := c.m.tx.CreateTask(ctx, task); err != nil {
		return err
	}
	for _, name := range src.Tags {
		if _, err := attachTaskLabel(ctx, c.m.tx, task.ID, name); err != nil {
			return err
		}
	}
	c.copies[src.ID] = task
	c.order = append(c.order, src.ID)
	if c.in.WithComments {
		return c.copyComments(ctx, src, task)
	}
	return nil
}

// parentOf is the parent a copy hangs under: the copy of its source's parent
// for a subtask, the source's own parent for a root copy that stays in its
// project, and none for a root copy that leaves it.
func (c *copier) parentOf(src *core.Task, root bool) string {
	if !root {
		if parent, ok := c.copies[src.ParentID]; ok {
			return parent.ID
		}
		return ""
	}
	if c.sameHome {
		return src.ParentID
	}
	return ""
}

// status is where a copy starts: the target's initial state, or the source's
// own state when the caller asked to keep it and the target has one so named.
func (c *copier) status(from string) string {
	if c.in.KeepStatus && c.workflow.HasState(from) {
		return from
	}
	return c.workflow.Initial
}

// fields keeps the values the target defines and accepts, then applies the
// target's defaults and required fields as a create does.
func (c *copier) fields(values map[string]any) (map[string]any, error) {
	byKey := make(map[string]core.FieldDef, len(c.defs))
	for _, d := range c.defs {
		byKey[d.Key] = d
	}
	kept := map[string]any{}
	for k, v := range values {
		def, ok := byKey[k]
		if !ok || v == nil {
			continue
		}
		if coerced, err := coerceFieldValue(def, v); err == nil {
			kept[k] = coerced
		}
	}
	return validateTaskFields(c.defs, kept, true)
}

// copyComments writes each live comment of src onto its copy as a comment by
// the copier that names who wrote the original and when.
func (c *copier) copyComments(ctx context.Context, src, dst *core.Task) error {
	comments, err := c.m.tx.ListComments(ctx, src.ID)
	if err != nil {
		return err
	}
	for _, orig := range comments {
		body := copiedCommentBody(c.handle(ctx, orig.AuthorActorID), orig.CreatedAt, orig.Body)
		comment := &core.Comment{TaskID: dst.ID, AuthorActorID: c.m.actor.ID, Body: body}
		if err := c.m.tx.CreateComment(ctx, comment); err != nil {
			return err
		}
		if err := c.m.Record("comment.add", core.EventCommentAdded, "comment", comment.ID, dst.ProjectID,
			nil, comment, map[string]any{"ref": dst.Ref, "task_id": dst.ID, "copied_from": orig.ID}); err != nil {
			return err
		}
	}
	return nil
}

// copiedCommentBody prefixes a copied comment with its original author and
// time, trimming the original so the result stays within the comment limit.
func copiedCommentBody(author string, at time.Time, body string) string {
	head := fmt.Sprintf("Copied from a comment by %s on %s:\n\n", author, at.UTC().Format("2006-01-02 15:04 UTC"))
	if room := core.MaxCommentLength - len(head); len(body) > room {
		body = strings.ToValidUTF8(body[:room], "")
	}
	return head + body
}

// handle names an actor the way a comment would, falling back to the
// identifier for an actor this tenant cannot resolve.
func (c *copier) handle(ctx context.Context, actorID string) string {
	if h, ok := c.handles[actorID]; ok {
		return h
	}
	name := actorID
	if a, err := c.m.tx.GetActor(ctx, actorID); err == nil && a.Handle != "" {
		name = "@" + a.Handle
	}
	c.handles[actorID] = name
	return name
}

// copyDependencies gives the copy of src the edges src has, pointing an edge
// whose far end was copied too at that copy. An edge the caller could not
// create through AddDependency is skipped rather than refusing the copy.
func (c *copier) copyDependencies(ctx context.Context, src *core.Task) error {
	edges, err := c.m.tx.ListDependencies(ctx, src.ID)
	if err != nil {
		return err
	}
	dst := c.copies[src.ID]
	for _, edge := range edges {
		far, ok := c.copies[edge.DependsOn]
		if !ok {
			dep, err := c.m.tx.GetTask(ctx, core.TaskRef{ID: edge.DependsOn})
			if err != nil || dep.Deleted() {
				continue
			}
			if c.l.authorizeTask(ctx, authz.ActionTaskUpdate, dep) != nil {
				continue
			}
			far = dep
		}
		if err := linkDependency(ctx, c.m.tx, dst, far); err != nil {
			return err
		}
	}
	return nil
}

// record writes the audit entry and event of every copy, after its tags and
// edges exist, so the snapshot is the task as it landed.
func (c *copier) record(ctx context.Context, sources []core.Task) error {
	refs := make(map[string]string, len(sources))
	for _, s := range sources {
		refs[s.ID] = s.Ref
	}
	for _, id := range c.order {
		loaded, err := c.m.tx.GetTask(ctx, core.TaskRef{ID: c.copies[id].ID})
		if err != nil {
			return err
		}
		if err := c.m.Record("task.copy", core.EventTaskCreated, "task", loaded.ID, loaded.ProjectID,
			nil, loaded, map[string]any{
				"ref": loaded.Ref, "title": loaded.Title, "status": loaded.Status, "copied_from": refs[id],
			}); err != nil {
			return err
		}
	}
	return nil
}
