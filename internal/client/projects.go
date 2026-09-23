// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// CreateProject creates a project.
func (c *Client) CreateProject(ctx context.Context, in core.CreateProjectInput) (*core.Project, error) {
	return call[core.Project](ctx, c, http.MethodPost, wire.RouteProjects, nil, in)
}

// GetProject returns one project.
func (c *Client) GetProject(ctx context.Context, ref string) (*core.Project, error) {
	return call[core.Project](ctx, c, http.MethodGet, routePath(wire.RouteProject, "ref", ref), nil, nil)
}

// ListProjects returns one page of projects.
func (c *Client) ListProjects(ctx context.Context, f core.ProjectFilter) ([]core.Project, string, error) {
	q := pageQuery(f.Page)
	setStrings(q, "key", f.Keys)
	setBool(q, "include_archived", f.IncludeArchived)
	return list[core.Project](ctx, c, wire.RouteProjects, q)
}

// UpdateProject changes a project.
func (c *Client) UpdateProject(ctx context.Context, ref string, in core.UpdateProjectInput) (*core.Project, error) {
	return call[core.Project](ctx, c, http.MethodPatch, routePath(wire.RouteProject, "ref", ref), nil, in)
}

// ArchiveProject archives a project.
func (c *Client) ArchiveProject(ctx context.Context, ref string) error {
	q := url.Values{"archive": []string{"true"}}
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteProject, "ref", ref), q, nil)
}

// DeleteProject removes a project.
func (c *Client) DeleteProject(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteProject, "ref", ref), nil, nil)
}

// PutFieldDef defines or redefines a custom field.
func (c *Client) PutFieldDef(ctx context.Context, projectRef string, in core.FieldDefInput) (*core.FieldDef, error) {
	path := routePath(wire.RouteProjectFields, "ref", projectRef)
	return call[core.FieldDef](ctx, c, http.MethodPut, path, nil, in)
}

// ListFieldDefs returns a project's custom fields.
func (c *Client) ListFieldDefs(ctx context.Context, projectRef string) ([]core.FieldDef, error) {
	return listAll[core.FieldDef](ctx, c, routePath(wire.RouteProjectFields, "ref", projectRef), nil)
}

// DeleteFieldDef removes a custom field.
func (c *Client) DeleteFieldDef(ctx context.Context, projectRef, key string) error {
	path := routePath(wire.RouteProjectField, "ref", projectRef, "key", key)
	return callVoid(ctx, c, http.MethodDelete, path, nil, nil)
}
