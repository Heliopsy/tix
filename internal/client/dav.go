// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// GetDAVInfo reports whether the server answers CalDAV and how to reach it.
func (c *Client) GetDAVInfo(ctx context.Context) (*core.DAVInfo, error) {
	return call[core.DAVInfo](ctx, c, http.MethodGet, wire.RouteDAV, nil, nil)
}

// ListDAVPasswords lists the caller's app passwords.
func (c *Client) ListDAVPasswords(ctx context.Context) ([]core.DAVPassword, error) {
	return listAll[core.DAVPassword](ctx, c, wire.RouteDAVPasswords, nil)
}

// CreateDAVPassword issues the caller an app password.
func (c *Client) CreateDAVPassword(ctx context.Context, in core.CreateDAVPasswordInput) (*core.IssuedDAVPassword, error) {
	return call[core.IssuedDAVPassword](ctx, c, http.MethodPost, wire.RouteDAVPasswords, nil, in)
}

// RevokeDAVPassword revokes one of the caller's app passwords.
func (c *Client) RevokeDAVPassword(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteDAVPassword, "id", id), nil, nil)
}
