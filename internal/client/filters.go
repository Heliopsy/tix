// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListSavedFilters returns the caller's own filters, then the shared ones.
func (c *Client) ListSavedFilters(ctx context.Context) ([]core.SavedFilter, error) {
	return listAll[core.SavedFilter](ctx, c, wire.RouteFilters, nil)
}

// GetSavedFilter returns one filter the caller can see, by id or name.
func (c *Client) GetSavedFilter(ctx context.Context, ref string) (*core.SavedFilter, error) {
	return call[core.SavedFilter](ctx, c, http.MethodGet, routePath(wire.RouteFilter, "filter", ref), nil, nil)
}

// CreateSavedFilter saves a new filter for the caller.
func (c *Client) CreateSavedFilter(ctx context.Context, in core.SavedFilterInput) (*core.SavedFilter, error) {
	return call[core.SavedFilter](ctx, c, http.MethodPost, wire.RouteFilters, nil, in)
}

// UpdateSavedFilter renames, edits or shares a filter.
func (c *Client) UpdateSavedFilter(ctx context.Context, ref string, in core.SavedFilterUpdate) (*core.SavedFilter, error) {
	return call[core.SavedFilter](ctx, c, http.MethodPatch, routePath(wire.RouteFilter, "filter", ref), nil, in)
}

// OrderSavedFilters puts the caller's named filters first, in that order.
func (c *Client) OrderSavedFilters(ctx context.Context, in core.SavedFilterOrderInput) ([]core.SavedFilter, error) {
	page, err := call[wire.Page[core.SavedFilter]](ctx, c, http.MethodPut, wire.RouteFilterOrder, nil, in)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// DeleteSavedFilter removes a filter.
func (c *Client) DeleteSavedFilter(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteFilter, "filter", ref), nil, nil)
}
