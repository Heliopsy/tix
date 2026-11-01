// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// SaveTemplate keeps a task as a template.
func (c *Client) SaveTemplate(ctx context.Context, in core.SaveTemplateInput) (*core.TaskTemplate, error) {
	return call[core.TaskTemplate](ctx, c, http.MethodPost, wire.RouteTemplates, nil, in)
}

// GetTemplate reads one template by name or identifier.
func (c *Client) GetTemplate(ctx context.Context, ref string) (*core.TaskTemplate, error) {
	return call[core.TaskTemplate](ctx, c, http.MethodGet, routePath(wire.RouteTemplate, "ref", ref), nil, nil)
}

// ListTemplates lists the templates the caller can read.
func (c *Client) ListTemplates(ctx context.Context, f core.TemplateFilter) ([]core.TaskTemplate, string, error) {
	q := pageQuery(f.Page)
	if f.ProjectRef != "" {
		q.Set("project", f.ProjectRef)
	}
	return list[core.TaskTemplate](ctx, c, wire.RouteTemplates, q)
}

// UpdateTemplate renames, rescopes or edits a template.
func (c *Client) UpdateTemplate(ctx context.Context, ref string, in core.UpdateTemplateInput) (*core.TaskTemplate, error) {
	return call[core.TaskTemplate](ctx, c, http.MethodPatch, routePath(wire.RouteTemplate, "ref", ref), nil, in)
}

// DeleteTemplate removes a template.
func (c *Client) DeleteTemplate(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteTemplate, "ref", ref), nil, nil)
}

// UseTemplate creates a task from a template.
func (c *Client) UseTemplate(ctx context.Context, ref string, in core.UseTemplateInput) (*core.Task, error) {
	return call[core.Task](ctx, c, http.MethodPost, routePath(wire.RouteTemplateUse, "ref", ref), nil, in)
}
