// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Audit actions of task templates.
const (
	auditTemplateSave   = "template.save"
	auditTemplateUpdate = "template.update"
	auditTemplateDelete = "template.delete"
	auditTaskTemplate   = "task.template"
)

// SaveTemplate keeps a live task, and its live subtasks when asked, as a
// named template in the task's project or, asked to, for the whole tenant.
func (l *Local) SaveTemplate(ctx context.Context, in core.SaveTemplateInput) (*core.TaskTemplate, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.TaskRef = strings.TrimSpace(in.TaskRef)
	if err := in.Validate(); err != nil {
		return nil, err
	}
	ref, err := core.ParseTaskRef(in.TaskRef)
	if err != nil {
		return nil, core.OnField(err, "task_ref")
	}
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.TaskTemplate
	err = l.write(ctx, actor, func(m *mutation) error {
		source, err := liveTask(ctx, m.tx, ref)
		if err != nil {
			return core.OnField(err, "task_ref")
		}
		if err := l.authorizeTask(ctx, authz.ActionTaskRead, source); err != nil {
			return err
		}
		tpl := &core.TaskTemplate{Name: in.Name, DueIn: in.DueIn, CreatorActorID: actor.ID}
		if !in.TenantWide {
			tpl.ProjectID = source.ProjectID
		}
		if err := l.authorizeTemplateWrite(ctx, tpl.ProjectID); err != nil {
			return err
		}
		if err := nameFree(ctx, m.tx, tpl.Name, ""); err != nil {
			return err
		}
		if tpl.Task, err = snapshotTask(ctx, m.tx, source, in.WithSubtasks); err != nil {
			return err
		}
		if err := m.tx.CreateTemplate(ctx, tpl); err != nil {
			return err
		}
		if out, err = m.tx.GetTemplate(ctx, tpl.ID); err != nil {
			return err
		}
		return recordTemplate(m, auditTemplateSave, nil, out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetTemplate reads one template by name or identifier.
func (l *Local) GetTemplate(ctx context.Context, ref string) (*core.TaskTemplate, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.TaskTemplate
	err = l.read(ctx, actor, func(tx store.Tx) error {
		out, err = l.readableTemplate(ctx, tx, ref)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListTemplates lists the templates the caller can read, by name. A project
// filter keeps that project's templates and the tenant-wide ones; a token
// pinned to a project always has that filter.
func (l *Local) ListTemplates(ctx context.Context, f core.TemplateFilter) ([]core.TaskTemplate, string, error) {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	if f.Page.Sort == "" {
		f.Page.Sort = "name"
	}
	page, err := f.Page.Normalize()
	if err != nil {
		return nil, "", err
	}
	if err := mustCursorMatch(page); err != nil {
		return nil, "", err
	}
	var out []core.TaskTemplate
	err = l.read(ctx, actor, func(tx store.Tx) error {
		q := store.TemplateQuery{Page: page, ProjectID: actor.ProjectID}
		if ref := strings.TrimSpace(f.ProjectRef); ref != "" {
			p, err := lookupProject(ctx, tx, ref)
			if err != nil {
				return core.OnField(err, "project")
			}
			if _, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{ProjectID: p.ID}); err != nil {
				return err
			}
			q.ProjectID = p.ID
		}
		out, err = tx.ListTemplates(ctx, q)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	return out, nextTemplateCursor(page, out), nil
}

// UpdateTemplate renames, rescopes or edits a template. A change that leaves
// it as it was writes nothing.
func (l *Local) UpdateTemplate(ctx context.Context, ref string, in core.UpdateTemplateInput) (*core.TaskTemplate, error) {
	if err := in.Validate(); err != nil {
		return nil, err
	}
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.TaskTemplate
	err = l.write(ctx, actor, func(m *mutation) error {
		before, err := l.readableTemplate(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTemplateWrite(ctx, before.ProjectID); err != nil {
			return err
		}
		after, err := applyTemplateUpdate(ctx, m.tx, *before, in)
		if err != nil {
			return err
		}
		if after.ProjectID != before.ProjectID {
			if err := l.authorizeTemplateWrite(ctx, after.ProjectID); err != nil {
				return err
			}
		}
		if !strings.EqualFold(after.Name, before.Name) {
			if err := nameFree(ctx, m.tx, after.Name, before.ID); err != nil {
				return err
			}
		}
		if sameTemplate(*before, after) {
			out = before
			return nil
		}
		if err := m.tx.UpdateTemplate(ctx, &after); err != nil {
			return err
		}
		if out, err = m.tx.GetTemplate(ctx, before.ID); err != nil {
			return err
		}
		return recordTemplate(m, auditTemplateUpdate, before, out)
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteTemplate removes a template. Tasks made from it are not touched.
func (l *Local) DeleteTemplate(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		tpl, err := l.readableTemplate(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if err := l.authorizeTemplateWrite(ctx, tpl.ProjectID); err != nil {
			return err
		}
		if err := m.tx.DeleteTemplate(ctx, tpl.ID); err != nil {
			return err
		}
		return recordTemplate(m, auditTemplateDelete, tpl, nil)
	})
}

// UseTemplate creates a task, and a subtask for each the template holds, in
// one transaction, through the copier CopyTask uses, so a task made from a
// template gets the same status, fields and tags rules as a copy.
func (l *Local) UseTemplate(ctx context.Context, ref string, in core.UseTemplateInput) (*core.Task, error) {
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
		tpl, err := l.readableTemplate(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		target := in.ProjectRef
		if target == "" {
			target = tpl.ProjectID
		}
		project, err := projectForTask(ctx, m.tx, target)
		if err != nil {
			return core.OnField(err, "project_ref")
		}
		c, err := l.copierInto(ctx, m, project, core.CopyTaskInput{}, false)
		if err != nil {
			return err
		}
		c.template = tpl.Name
		tpl.Uses++
		values := core.TemplateValues{Date: m.now, N: tpl.Uses, Project: project.Key}
		sources := templateSources(tpl.Task, values)
		if in.Title != "" {
			sources[0].Title = core.FillPlaceholders(in.Title, values)
		}
		due := tpl.DueIn
		if in.DueIn != nil {
			due = *in.DueIn
		}
		sources[0].DueAt = templateDue(m.now, due)
		for i := range sources {
			if err := c.copyOne(ctx, &sources[i], i == 0); err != nil {
				return err
			}
		}
		if err := c.record(ctx, sources); err != nil {
			return err
		}
		if err := m.tx.UpdateTemplate(ctx, tpl); err != nil {
			return err
		}
		out, err = m.tx.GetTask(ctx, core.TaskRef{ID: c.copies[sources[0].ID].ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// readableTemplate loads a template and checks the caller may read it. A
// project template outside a pinned token's project reads as absent.
func (l *Local) readableTemplate(ctx context.Context, tx store.Tx, ref string) (*core.TaskTemplate, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("a template name is required").WithDetail(core.DetailField, "template")
	}
	tpl, err := tx.GetTemplate(ctx, ref)
	if err != nil {
		return nil, err
	}
	if tpl.ProjectID == "" {
		return tpl, nil
	}
	if _, err := l.authorize(ctx, authz.ActionTaskRead, authz.Resource{ProjectID: tpl.ProjectID}); err != nil {
		if core.IsKind(err, core.KindForbidden) {
			return nil, core.NotFound("template %q", ref)
		}
		return nil, err
	}
	return tpl, nil
}

// authorizeTemplateWrite checks the authority to change a template in a
// project, or a tenant-wide one when projectID is empty.
func (l *Local) authorizeTemplateWrite(ctx context.Context, projectID string) error {
	if projectID == "" {
		_, err := l.authorize(ctx, authz.ActionTemplateTenantWrite, authz.Resource{})
		return err
	}
	_, err := l.authorize(ctx, authz.ActionTaskCreate, authz.Resource{ProjectID: projectID})
	return err
}

// nameFree refuses a name another template already holds, in any case.
func nameFree(ctx context.Context, tx store.Tx, name, selfID string) error {
	other, err := tx.GetTemplate(ctx, name)
	switch {
	case err == nil && other.ID != selfID:
		return core.Conflict("a template named %q already exists", other.Name).WithDetail(core.DetailField, "name")
	case err != nil && !core.IsKind(err, core.KindNotFound):
		return err
	}
	return nil
}

// snapshotTask is the template shape of a task and, when asked, its live
// subtree, each subtask under its parent in the order the tree walk gives.
func snapshotTask(ctx context.Context, tx store.Tx, root *core.Task, withSubtasks bool) (core.TemplateTask, error) {
	tasks := []core.Task{*root}
	if withSubtasks {
		if err := appendDescendants(ctx, tx, root.ID, 0, 1, &tasks); err != nil {
			return core.TemplateTask{}, err
		}
	}
	if len(tasks) > core.MaxTemplateTasks {
		return core.TemplateTask{}, core.Invalid("a template holds at most %d tasks and this one has %d",
			core.MaxTemplateTasks, len(tasks)).WithDetail(core.DetailField, "with_subtasks")
	}
	children := map[string][]core.Task{}
	for _, t := range tasks[1:] {
		children[t.ParentID] = append(children[t.ParentID], t)
	}
	var build func(t core.Task) core.TemplateTask
	build = func(t core.Task) core.TemplateTask {
		node := core.TemplateTask{
			Title: t.Title, Body: t.Body, Priority: t.Priority, Tags: t.Tags,
			CustomFields: t.CustomFields, Recurrence: t.Recurrence,
		}
		for _, child := range children[t.ID] {
			node.Subtasks = append(node.Subtasks, build(child))
		}
		return node
	}
	return build(*root), nil
}

// templateSources flattens a template into the in-memory tasks the copier
// takes, in pre-order, with synthetic identifiers linking each subtask to
// its parent and every title's placeholders filled.
func templateSources(root core.TemplateTask, v core.TemplateValues) []core.Task {
	var out []core.Task
	var walk func(t core.TemplateTask, parent string)
	walk = func(t core.TemplateTask, parent string) {
		id := "template:" + strconv.Itoa(len(out))
		priority := t.Priority
		if !priority.Valid() {
			priority = core.PriorityNormal
		}
		out = append(out, core.Task{
			ID: id, ParentID: parent, Title: core.FillPlaceholders(t.Title, v), Body: t.Body,
			Priority: priority, Tags: t.Tags, CustomFields: t.CustomFields, Recurrence: t.Recurrence,
		})
		for _, s := range t.Subtasks {
			walk(s, id)
		}
	}
	walk(root, "")
	return out
}

// applyTemplateUpdate returns the template with the input's fields set.
func applyTemplateUpdate(ctx context.Context, tx store.Tx, tpl core.TaskTemplate, in core.UpdateTemplateInput) (core.TaskTemplate, error) {
	if in.Name != nil {
		tpl.Name = strings.TrimSpace(*in.Name)
	}
	if in.ProjectRef != nil {
		tpl.ProjectID, tpl.ProjectKey = "", ""
		if ref := strings.TrimSpace(*in.ProjectRef); ref != "" {
			p, err := lookupProject(ctx, tx, ref)
			if err != nil {
				return tpl, core.OnField(err, "project_ref")
			}
			tpl.ProjectID, tpl.ProjectKey = p.ID, p.Key
		}
	}
	if in.DueIn != nil {
		tpl.DueIn = *in.DueIn
	}
	if in.Title != nil {
		tpl.Task.Title = strings.TrimSpace(*in.Title)
	}
	if in.Body != nil {
		tpl.Task.Body = *in.Body
	}
	if in.Priority != nil {
		tpl.Task.Priority = *in.Priority
	}
	if in.Tags != nil {
		tpl.Task.Tags = cleanTags(*in.Tags)
	}
	if in.Recurrence != nil {
		rule, err := core.NormalizeRecurrence(*in.Recurrence)
		if err != nil {
			return tpl, core.OnField(err, "repeat")
		}
		tpl.Task.Recurrence = rule
	}
	if in.CustomFields != nil {
		fields := make(map[string]any, len(tpl.Task.CustomFields)+len(in.CustomFields))
		for k, v := range tpl.Task.CustomFields {
			fields[k] = v
		}
		for k, v := range in.CustomFields {
			if v == nil {
				delete(fields, k)
				continue
			}
			fields[k] = v
		}
		if len(fields) == 0 {
			fields = nil
		}
		tpl.Task.CustomFields = fields
	}
	return tpl, nil
}

// cleanTags trims tag names and drops blanks and repeats, keeping order.
func cleanTags(tags []string) []string {
	var out []string
	seen := map[string]bool{}
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if t == "" || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		out = append(out, t)
	}
	return out
}

// sameTemplate reports whether an update changed nothing a reader can see.
func sameTemplate(a, b core.TaskTemplate) bool {
	ja, errA := json.Marshal(a)
	jb, errB := json.Marshal(b)
	return errA == nil && errB == nil && string(ja) == string(jb)
}

func recordTemplate(m *mutation, action string, before, after *core.TaskTemplate) error {
	subject := after
	if subject == nil {
		subject = before
	}
	var beforeSnap, afterSnap any
	if before != nil {
		beforeSnap = before
	}
	if after != nil {
		afterSnap = after
	}
	return m.Record(action, core.EventTemplateUpdated, "template", subject.ID, subject.ProjectID,
		beforeSnap, afterSnap, map[string]any{"action": action, "name": subject.Name, "template_id": subject.ID})
}

func nextTemplateCursor(page core.Page, items []core.TaskTemplate) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	last := items[len(items)-1]
	return core.Cursor{
		SortValue: strings.ToLower(last.Name),
		ID:        last.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}.Encode()
}

// templateDue is when a use at now falls due under an offset, nil for none.
func templateDue(now time.Time, d core.Duration) *time.Time {
	if d <= 0 {
		return nil
	}
	at := now.Add(d.D())
	return &at
}
