// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var projectColumns = []string{
	"id", "tenant_id", "key", "name", "description", "workflow_id",
	"color", "icon", "archived_at", "created_at", "updated_at",
}

var projectSortColumns = map[string]string{
	"created_at": "created_at",
	"updated_at": "updated_at",
	"key":        "key",
	"name":       "name",
}

func scanProject(s scanner) (core.Project, error) {
	var (
		p        core.Project
		archived sql.NullTime
		created  sql.NullTime
		updated  sql.NullTime
	)
	if err := s.Scan(&p.ID, &p.TenantID, &p.Key, &p.Name, &p.Description, &p.WorkflowID,
		&p.Color, &p.Icon, &archived, &created, &updated); err != nil {
		return core.Project{}, mapErr(err, "scanning project")
	}
	p.ArchivedAt = scanNullTime(archived)
	p.CreatedAt = scanTime(created)
	p.UpdatedAt = scanTime(updated)
	return p, nil
}

// CreateProject inserts a project.
func (t *tx) CreateProject(ctx context.Context, p *core.Project) error {
	if p.ID == "" {
		p.ID = id.New()
	}
	p.TenantID = t.scope.TenantID
	if p.CreatedAt.IsZero() {
		p.CreatedAt = t.store.clock.Now()
	}
	p.UpdatedAt = t.store.clock.Now()

	ins := t.insert("projects").
		Set("id", p.ID).
		Set("key", p.Key).
		Set("name", p.Name).
		Set("description", p.Description).
		Set("workflow_id", p.WorkflowID).
		Set("color", string(p.Color)).
		Set("icon", p.Icon).
		Set("archived_at", nullTimeArg(p.ArchivedAt)).
		Set("created_at", timeArg(p.CreatedAt)).
		Set("updated_at", timeArg(p.UpdatedAt))
	_, err := t.execInsert(ctx, ins, "creating project %q", p.Key)
	return err
}

// GetProject returns a project by identifier or by key.
func (t *tx) GetProject(ctx context.Context, ref string) (*core.Project, error) {
	b := t.builder("projects").
		Select(projectColumns...).
		Where("(id = ? OR key = ?)", ref, ref).
		Limit(1)
	q, args := b.SelectQuery()
	p, err := scanProject(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("project %q", ref)
		}
		return nil, err
	}
	return &p, nil
}

// ListProjects returns projects matching the filter, keyset paginated.
func (t *tx) ListProjects(ctx context.Context, f core.ProjectFilter) ([]core.Project, error) {
	spec, err := resolvePage(f.Page, "created_at", projectSortColumns)
	if err != nil {
		return nil, err
	}
	b := t.builder("projects").Select(projectColumns...)
	if len(f.Keys) > 0 {
		b.WhereIn("key", f.Keys)
	}
	if !f.IncludeArchived {
		b.Where("archived_at IS NULL")
	}
	rows, err := t.query(ctx, spec.apply(b, "id"), "listing projects")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Project{}
	for rows.Next() {
		v, err := scanProject(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing projects")
}

// UpdateProject writes a project's mutable fields.
func (t *tx) UpdateProject(ctx context.Context, p *core.Project) error {
	p.UpdatedAt = t.store.clock.Now()
	b := t.builder("projects").
		Where("id = ?", p.ID).
		Set("key", p.Key).
		Set("name", p.Name).
		Set("description", p.Description).
		Set("workflow_id", p.WorkflowID).
		Set("color", string(p.Color)).
		Set("icon", p.Icon).
		Set("archived_at", nullTimeArg(p.ArchivedAt)).
		Set("updated_at", timeArg(p.UpdatedAt))
	n, err := t.execUpdate(ctx, b, "updating project %q", p.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("project %q", p.ID)
	}
	return nil
}

// DeleteProject removes a project and everything cascading from it.
func (t *tx) DeleteProject(ctx context.Context, projectID string) error {
	b := t.builder("projects").Where("id = ?", projectID)
	n, err := t.execDelete(ctx, b, "deleting project %q", projectID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("project %q", projectID)
	}
	return nil
}

var fieldDefColumns = []string{
	"id", "tenant_id", "project_id", "key", "tag", "type", "required",
	"enum_options", "default_value", "indexed", "position",
}

func scanFieldDef(s scanner) (core.FieldDef, error) {
	var (
		d       core.FieldDef
		options sql.NullString
		def     sql.NullString
	)
	if err := s.Scan(&d.ID, &d.TenantID, &d.ProjectID, &d.Key, &d.Label, &d.Type,
		&d.Required, &options, &def, &d.Indexed, &d.Position); err != nil {
		return core.FieldDef{}, mapErr(err, "scanning field definition")
	}
	if err := sqlb.ParseJSON(text(options), &d.EnumOptions); err != nil {
		return core.FieldDef{}, core.Internal("decoding enum options of field %q", d.Key).Wrap(err)
	}
	if def.Valid && def.String != "" {
		if err := sqlb.ParseJSON(def.String, &d.Default); err != nil {
			return core.FieldDef{}, core.Internal("decoding default of field %q", d.Key).Wrap(err)
		}
	}
	return d, nil
}

// PutFieldDef creates or replaces a custom field definition.
func (t *tx) PutFieldDef(ctx context.Context, d *core.FieldDef) error {
	d.TenantID = t.scope.TenantID
	options, err := sqlb.JSONText(d.EnumOptions, "")
	if err != nil {
		return core.Internal("encoding enum options of field %q", d.Key).Wrap(err)
	}
	def, err := sqlb.JSONText(d.Default, "")
	if err != nil {
		return core.Internal("encoding default of field %q", d.Key).Wrap(err)
	}

	upd := t.builder("field_defs").
		Where("project_id = ?", d.ProjectID).
		Where("key = ?", d.Key).
		Set("tag", d.Label).
		Set("type", string(d.Type)).
		Set("required", d.Required).
		Set("enum_options", nullText(options)).
		Set("default_value", nullText(def)).
		Set("indexed", d.Indexed).
		Set("position", d.Position)
	n, err := t.execUpdate(ctx, upd, "updating field %q", d.Key)
	if err != nil {
		return err
	}
	if n > 0 {
		if d.ID == "" {
			existing, err := t.getFieldDef(ctx, d.ProjectID, d.Key)
			if err != nil {
				return err
			}
			d.ID = existing.ID
		}
		return nil
	}

	if d.ID == "" {
		d.ID = id.New()
	}
	ins := t.insert("field_defs").
		Set("id", d.ID).
		Set("project_id", d.ProjectID).
		Set("key", d.Key).
		Set("tag", d.Label).
		Set("type", string(d.Type)).
		Set("required", d.Required).
		Set("enum_options", nullText(options)).
		Set("default_value", nullText(def)).
		Set("indexed", d.Indexed).
		Set("position", d.Position)
	_, err = t.execInsert(ctx, ins, "creating field %q", d.Key)
	return err
}

func (t *tx) getFieldDef(ctx context.Context, projectID, key string) (*core.FieldDef, error) {
	b := t.builder("field_defs").
		Select(fieldDefColumns...).
		Where("project_id = ?", projectID).
		Where("key = ?", key).
		Limit(1)
	q, args := b.SelectQuery()
	d, err := scanFieldDef(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("field %q on project %q", key, projectID)
		}
		return nil, err
	}
	return &d, nil
}

// ListFieldDefs returns a project's custom field definitions in display order.
func (t *tx) ListFieldDefs(ctx context.Context, projectID string) ([]core.FieldDef, error) {
	b := t.builder("field_defs").
		Select(fieldDefColumns...).
		Where("project_id = ?", projectID).
		OrderBy("position", core.Ascending).
		OrderBy("key", core.Ascending)
	rows, err := t.query(ctx, b, "listing field definitions")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.FieldDef{}
	for rows.Next() {
		v, err := scanFieldDef(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing field definitions")
}

// DeleteFieldDef removes a custom field definition.
func (t *tx) DeleteFieldDef(ctx context.Context, projectID, key string) error {
	b := t.builder("field_defs").
		Where("project_id = ?", projectID).
		Where("key = ?", key)
	n, err := t.execDelete(ctx, b, "deleting field %q", key)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("field %q on project %q", key, projectID)
	}
	return nil
}
