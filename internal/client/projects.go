// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

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

// GetProjectIcon fetches a project's uploaded icon image.
func (c *Client) GetProjectIcon(ctx context.Context, ref string) (*core.ProjectIconImage, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodGet, routePath(wire.RouteProjectIcon, "ref", ref), nil, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(wire.HeaderAccept, "image/*, "+wire.ContentJSON)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, core.MaxProjectIconBytes+1))
	if err != nil {
		return nil, core.Internal("reading the icon of project %q: %v", ref, err).Wrap(err)
	}
	return &core.ProjectIconImage{
		ContentType: resp.Header.Get(wire.HeaderContentType),
		Digest:      strings.Trim(resp.Header.Get("ETag"), `"`),
		Data:        data,
	}, nil
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

// DeleteProject removes a project, moving or deleting its tasks.
func (c *Client) DeleteProject(ctx context.Context, ref string, in core.DeleteProjectInput) (*core.ProjectDeletion, error) {
	return call[core.ProjectDeletion](ctx, c, http.MethodDelete, routePath(wire.RouteProject, "ref", ref),
		deletionQuery(in), nil)
}

// PlanProjectDeletion reports what deleting a project would do.
func (c *Client) PlanProjectDeletion(ctx context.Context, ref string, in core.DeleteProjectInput) (*core.ProjectDeletion, error) {
	return call[core.ProjectDeletion](ctx, c, http.MethodGet, routePath(wire.RouteProjectDeletion, "ref", ref),
		deletionQuery(in), nil)
}

func deletionQuery(in core.DeleteProjectInput) url.Values {
	q := url.Values{}
	if in.MoveTasksTo != "" {
		q.Set("move_tasks_to", in.MoveTasksTo)
	}
	setBool(q, "delete_tasks", in.DeleteTasks)
	return q
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
