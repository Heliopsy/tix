package service

import (
	"context"
	"strings"

	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
)

// lookupProject resolves a project by identifier or by key, accepting a key in
// any case because references are typed by hand.
func lookupProject(ctx context.Context, tx store.Tx, ref string) (*core.Project, error) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return nil, core.Invalid("project reference is required")
	}
	p, err := tx.GetProject(ctx, ref)
	if err == nil {
		return p, nil
	}
	if !core.IsKind(err, core.KindNotFound) {
		return nil, err
	}
	if lower := strings.ToLower(ref); lower != ref {
		return tx.GetProject(ctx, lower)
	}
	return nil, err
}

// projectPage normalizes a project filter's page and fixes its sort field.
func projectPage(f core.ProjectFilter) (core.ProjectFilter, error) {
	if f.Page.Sort == "" {
		f.Page.Sort = core.SortCreatedAt
	}
	page, err := f.Page.Normalize()
	if err != nil {
		return f, err
	}
	if err := mustCursorMatch(page); err != nil {
		return f, err
	}
	f.Page = page
	return f, nil
}

func mustCursorMatch(p core.Page) error {
	if p.Cursor == "" {
		return nil
	}
	c, err := core.DecodeCursor(p.Cursor)
	if err != nil {
		return err
	}
	return c.CheckOrdering(p.Sort, p.Direction)
}

// nextProjectCursor returns the cursor for the page after these projects.
func nextProjectCursor(page core.Page, items []core.Project) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	last := items[len(items)-1]
	var value string
	switch page.Sort {
	case core.SortUpdatedAt:
		value = sqlb.TimeText(last.UpdatedAt)
	case "key":
		value = last.Key
	case "name":
		value = last.Name
	default:
		value = sqlb.TimeText(last.CreatedAt)
	}
	return core.Cursor{
		SortValue: value,
		ID:        last.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}.Encode()
}

// CreateProject creates a project and assigns it a workflow.
func (l *Local) CreateProject(ctx context.Context, in core.CreateProjectInput) (*core.Project, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}
	color, err := core.NormalizeProjectColor(in.Color)
	if err != nil {
		return nil, err
	}
	icon, err := core.NormalizeProjectIcon(in.Icon)
	if err != nil {
		return nil, err
	}
	key := strings.ToLower(strings.TrimSpace(in.Key))
	workflowKey := strings.TrimSpace(in.WorkflowKey)
	if workflowKey == "" {
		workflowKey = BuiltinWorkflowKey
	}

	var out *core.Project
	err = l.write(ctx, actor, func(m *mutation) error {
		switch _, err := m.tx.GetProject(ctx, key); {
		case err == nil:
			return core.Conflict("project key %q is already in use", key)
		case !core.IsKind(err, core.KindNotFound):
			return err
		}
		wf, err := m.tx.GetWorkflow(ctx, workflowKey)
		if err != nil {
			return err
		}
		p := &core.Project{
			Key:         key,
			Name:        in.Name,
			Description: in.Description,
			WorkflowID:  wf.ID,
			Color:       color,
			Icon:        icon,
		}
		if err := m.tx.CreateProject(ctx, p); err != nil {
			return err
		}
		out = p
		return m.Record(auditProjectCreate, core.EventProjectCreated, "project", p.ID, p.ID, nil, p,
			map[string]any{"key": p.Key, "workflow_key": wf.Key})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// GetProject returns a project by identifier or key.
func (l *Local) GetProject(ctx context.Context, ref string) (*core.Project, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.Project
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		p, err := lookupProject(ctx, tx, ref)
		out = p
		return err
	}); err != nil {
		return nil, err
	}
	if _, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{ProjectID: out.ID}); err != nil {
		return nil, err
	}
	return out, nil
}

// ListProjects returns projects matching the filter, keyset paginated.
func (l *Local) ListProjects(ctx context.Context, f core.ProjectFilter) ([]core.Project, string, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectRead, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	f, err = projectPage(f)
	if err != nil {
		return nil, "", err
	}

	out := []core.Project{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListProjects(ctx, f)
		out = found
		return err
	}); err != nil {
		return nil, "", err
	}
	if actor.ScopedToProject() {
		kept := out[:0]
		for _, p := range out {
			if p.ID == actor.ProjectID {
				kept = append(kept, p)
			}
		}
		return kept, "", nil
	}
	return out, nextProjectCursor(f.Page, out), nil
}

// UpdateProject changes a project's mutable fields.
func (l *Local) UpdateProject(ctx context.Context, ref string, in core.UpdateProjectInput) (*core.Project, error) {
	actor, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) == "" {
		return nil, core.Invalid("project name is required")
	}
	var color core.ProjectColor
	if in.Color != nil {
		if color, err = core.NormalizeProjectColor(*in.Color); err != nil {
			return nil, err
		}
	}
	var icon string
	if in.Icon != nil {
		if icon, err = core.NormalizeProjectIcon(*in.Icon); err != nil {
			return nil, err
		}
	}

	var out *core.Project
	err = l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		before := *p
		if in.Name != nil {
			p.Name = *in.Name
		}
		if in.Description != nil {
			p.Description = *in.Description
		}
		if in.Color != nil {
			p.Color = color
		}
		if in.Icon != nil {
			p.Icon = icon
		}
		if in.WorkflowKey != nil {
			wf, err := m.tx.GetWorkflow(ctx, strings.TrimSpace(*in.WorkflowKey))
			if err != nil {
				return err
			}
			if err := ensureStatusesSurvive(ctx, m.tx, p, wf); err != nil {
				return err
			}
			p.WorkflowID = wf.ID
		}
		if err := m.tx.UpdateProject(ctx, p); err != nil {
			return err
		}
		out = p
		return m.Record(auditProjectUpdate, core.EventProjectUpdated, "project", p.ID, p.ID, before, p,
			map[string]any{"key": p.Key})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ensureStatusesSurvive refuses a workflow reassignment that would leave a task
// of the project in a status the new workflow does not define.
func ensureStatusesSurvive(ctx context.Context, tx store.Tx, p *core.Project, next *core.Workflow) error {
	if p.WorkflowID == next.ID {
		return nil
	}
	current, err := tx.GetWorkflowByID(ctx, p.WorkflowID)
	if err != nil {
		return err
	}
	for _, s := range current.Definition.States {
		if next.Definition.HasState(s.Key) {
			continue
		}
		tasks, err := tx.ListTasks(ctx, core.TaskFilter{
			ProjectIDs: []string{p.ID},
			Statuses:   []string{s.Key},
			Page:       core.Page{Limit: 1},
		})
		if err != nil {
			return err
		}
		if len(tasks) > 0 {
			return core.Conflict("workflow %q does not define state %q, which tasks of project %q are in",
				next.Key, s.Key, p.Key).WithDetail("state", s.Key)
		}
	}
	return nil
}

// ArchiveProject hides a project without destroying its history.
func (l *Local) ArchiveProject(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		if p.Archived() {
			return core.Conflict("project %q is already archived", p.Key)
		}
		before := *p
		now := m.now
		p.ArchivedAt = &now
		if err := m.tx.UpdateProject(ctx, p); err != nil {
			return err
		}
		return m.Record(auditProjectUpdate, core.EventProjectUpdated, "project", p.ID, p.ID, before, p,
			map[string]any{"key": p.Key, "archived": true})
	})
}

// DeleteProject removes a project and everything cascading from it.
func (l *Local) DeleteProject(ctx context.Context, ref string) error {
	actor, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		p, err := lookupProject(ctx, m.tx, ref)
		if err != nil {
			return err
		}
		if _, err := l.authorize(ctx, authz.ActionProjectWrite, authz.Resource{ProjectID: p.ID}); err != nil {
			return err
		}
		if err := m.tx.DeleteProject(ctx, p.ID); err != nil {
			return err
		}
		return m.Record(auditProjectDelete, eventProjectDeleted, "project", p.ID, p.ID, p, nil,
			map[string]any{"key": p.Key})
	})
}
