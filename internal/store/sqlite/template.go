// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var templateColumns = []string{
	"task_templates.id", "task_templates.tenant_id", "task_templates.name", "task_templates.project_id",
	"task_templates.body", "task_templates.due_in", "task_templates.uses", "task_templates.creator_actor_id",
	"task_templates.created_at", "task_templates.updated_at", "projects.key",
}

const templateProjectJoin = "LEFT JOIN projects ON projects.id = task_templates.project_id"

var templateSortColumns = map[string]string{"name": "task_templates.name_key"}

func scanTemplate(s scanner) (core.TaskTemplate, error) {
	var (
		tpl              core.TaskTemplate
		project, key     sql.NullString
		body             string
		dueIn            int64
		created, updated sql.NullString
	)
	if err := s.Scan(&tpl.ID, &tpl.TenantID, &tpl.Name, &project, &body, &dueIn, &tpl.Uses,
		&tpl.CreatorActorID, &created, &updated, &key); err != nil {
		return core.TaskTemplate{}, mapErr(err, "scanning task template")
	}
	tpl.ProjectID, tpl.ProjectKey = sqlb.Text(project), sqlb.Text(key)
	tpl.DueIn = core.Duration(time.Duration(dueIn) * time.Second)
	if err := sqlb.ParseJSON(body, &tpl.Task); err != nil {
		return core.TaskTemplate{}, core.Internal("decoding task template %q", tpl.ID).Wrap(err)
	}
	var err error
	if tpl.CreatedAt, err = sqlb.ScanTime(created); err != nil {
		return core.TaskTemplate{}, err
	}
	if tpl.UpdatedAt, err = sqlb.ScanTime(updated); err != nil {
		return core.TaskTemplate{}, err
	}
	return tpl, nil
}

// CreateTemplate inserts a task template.
func (t *tx) CreateTemplate(ctx context.Context, tpl *core.TaskTemplate) error {
	if tpl.ID == "" {
		tpl.ID = id.New()
	}
	tpl.TenantID = t.scope.TenantID
	now := t.store.clock.Now()
	tpl.CreatedAt, tpl.UpdatedAt = now, now
	body, err := sqlb.JSONText(tpl.Task, "{}")
	if err != nil {
		return core.Internal("encoding task template %q", tpl.Name).Wrap(err)
	}
	ins := t.insert("task_templates").
		Set("id", tpl.ID).
		Set("name", tpl.Name).
		Set("name_key", strings.ToLower(tpl.Name)).
		Set("project_id", sqlb.NullText(tpl.ProjectID)).
		Set("body", body).
		Set("due_in", int64(tpl.DueIn.D()/time.Second)).
		Set("uses", tpl.Uses).
		Set("creator_actor_id", tpl.CreatorActorID).
		Set("created_at", sqlb.TimeText(now)).
		Set("updated_at", sqlb.TimeText(now))
	_, err = t.execInsert(ctx, ins, "creating task template %q", tpl.Name)
	return err
}

// GetTemplate reads a task template by identifier or, ignoring case, name.
func (t *tx) GetTemplate(ctx context.Context, ref string) (*core.TaskTemplate, error) {
	b := t.builder("task_templates").Select(templateColumns...).Join(templateProjectJoin).
		Where("(task_templates.id = ? OR task_templates.name_key = ?)", ref, strings.ToLower(ref)).
		Limit(1)
	q, args := b.SelectQuery()
	tpl, err := scanTemplate(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("template %q", ref)
		}
		return nil, err
	}
	return &tpl, nil
}

// ListTemplates lists task templates by name, keyset-paginated.
func (t *tx) ListTemplates(ctx context.Context, f store.TemplateQuery) ([]core.TaskTemplate, error) {
	spec, err := resolvePage(f.Page, "name", templateSortColumns)
	if err != nil {
		return nil, err
	}
	b := t.builder("task_templates").Select(templateColumns...).Join(templateProjectJoin)
	if f.ProjectID != "" {
		b.Where("(task_templates.project_id = ? OR task_templates.project_id IS NULL)", f.ProjectID)
	}
	rows, err := t.query(ctx, spec.apply(b, "task_templates.id"), "listing task templates")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.TaskTemplate{}
	for rows.Next() {
		tpl, err := scanTemplate(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, tpl)
	}
	return out, mapRowsErr(rows, "listing task templates")
}

// UpdateTemplate writes a task template's mutable fields.
func (t *tx) UpdateTemplate(ctx context.Context, tpl *core.TaskTemplate) error {
	tpl.UpdatedAt = t.store.clock.Now()
	body, err := sqlb.JSONText(tpl.Task, "{}")
	if err != nil {
		return core.Internal("encoding task template %q", tpl.Name).Wrap(err)
	}
	b := t.builder("task_templates").
		Where("id = ?", tpl.ID).
		Set("name", tpl.Name).
		Set("name_key", strings.ToLower(tpl.Name)).
		Set("project_id", sqlb.NullText(tpl.ProjectID)).
		Set("body", body).
		Set("due_in", int64(tpl.DueIn.D()/time.Second)).
		Set("uses", tpl.Uses).
		Set("updated_at", sqlb.TimeText(tpl.UpdatedAt))
	n, err := t.execUpdate(ctx, b, "updating task template %q", tpl.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("template %q", tpl.ID)
	}
	return nil
}

// DeleteTemplate removes a task template.
func (t *tx) DeleteTemplate(ctx context.Context, templateID string) error {
	n, err := t.execDelete(ctx, t.builder("task_templates").Where("id = ?", templateID),
		"deleting task template %q", templateID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("template %q", templateID)
	}
	return nil
}
