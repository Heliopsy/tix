// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListGroupPrefs returns the caller's own preferences on one view.
func (c *Client) ListGroupPrefs(ctx context.Context, view string) ([]core.GroupPref, error) {
	return listAll[core.GroupPref](ctx, c, routePath(wire.RouteViewGroups, "view", view), nil)
}

// SetGroupPref hides, collapses or stars one group for the caller.
func (c *Client) SetGroupPref(ctx context.Context, view, group string, in core.GroupPrefInput) (*core.GroupPref, error) {
	path := routePath(wire.RouteViewGroup, "view", view, "group", group)
	return call[core.GroupPref](ctx, c, http.MethodPatch, path, nil, in)
}

// OrderGroups puts the named groups first, in the order given.
func (c *Client) OrderGroups(ctx context.Context, view string, in core.GroupOrderInput) ([]core.GroupPref, error) {
	page, err := call[wire.Page[core.GroupPref]](ctx, c, http.MethodPut, routePath(wire.RouteViewOrder, "view", view), nil, in)
	if err != nil {
		return nil, err
	}
	return page.Items, nil
}

// ResetGroupPrefs forgets the caller's preference on one group, or on every
// group of the view when group is empty.
func (c *Client) ResetGroupPrefs(ctx context.Context, view, group string) error {
	var q url.Values
	if group != "" {
		q = url.Values{"group": {group}}
	}
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteViewGroups, "view", view), q, nil)
}
