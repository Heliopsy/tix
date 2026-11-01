// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListInbox returns one page of the caller's items, newest first.
func (c *Client) ListInbox(ctx context.Context, f core.InboxFilter) ([]core.InboxItem, string, error) {
	q := pageQuery(f.Page)
	if f.Unread {
		q.Set("unread", "true")
	}
	return list[core.InboxItem](ctx, c, wire.RouteInbox, q)
}

// CountInbox counts the caller's unread items.
func (c *Client) CountInbox(ctx context.Context) (*core.InboxCount, error) {
	return call[core.InboxCount](ctx, c, http.MethodGet, wire.RouteInboxCount, nil, nil)
}

// ReadInboxItem marks one of the caller's items read.
func (c *Client) ReadInboxItem(ctx context.Context, id string) (*core.InboxItem, error) {
	return call[core.InboxItem](ctx, c, http.MethodPut, routePath(wire.RouteInboxRead, "id", id), nil, nil)
}

// UnreadInboxItem marks one of the caller's items unread.
func (c *Client) UnreadInboxItem(ctx context.Context, id string) (*core.InboxItem, error) {
	return call[core.InboxItem](ctx, c, http.MethodDelete, routePath(wire.RouteInboxRead, "id", id), nil, nil)
}

// ReadAllInbox marks every one of the caller's items read.
func (c *Client) ReadAllInbox(ctx context.Context) (*core.InboxReadAll, error) {
	return call[core.InboxReadAll](ctx, c, http.MethodPost, wire.RouteInboxReadAll, nil, nil)
}

// GetInboxPrefs returns the kinds the caller muted.
func (c *Client) GetInboxPrefs(ctx context.Context) (*core.InboxPrefs, error) {
	return call[core.InboxPrefs](ctx, c, http.MethodGet, wire.RouteInboxPrefs, nil, nil)
}

// PutInboxPrefs replaces the kinds the caller muted.
func (c *Client) PutInboxPrefs(ctx context.Context, in core.InboxPrefsInput) (*core.InboxPrefs, error) {
	return call[core.InboxPrefs](ctx, c, http.MethodPut, wire.RouteInboxPrefs, nil, in)
}
