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

// ListInboxChannels returns the caller's notification channels.
func (c *Client) ListInboxChannels(ctx context.Context) ([]core.InboxChannel, error) {
	return listAll[core.InboxChannel](ctx, c, wire.RouteChannels, nil)
}

// CreateInboxChannel adds a channel and returns its secret, once.
func (c *Client) CreateInboxChannel(ctx context.Context, in core.InboxChannelInput) (*core.IssuedInboxChannel, error) {
	return call[core.IssuedInboxChannel](ctx, c, http.MethodPost, wire.RouteChannels, nil, in)
}

// UpdateInboxChannel changes one of the caller's channels.
func (c *Client) UpdateInboxChannel(ctx context.Context, id string, in core.InboxChannelUpdate) (*core.InboxChannel, error) {
	return call[core.InboxChannel](ctx, c, http.MethodPatch, routePath(wire.RouteChannel, "id", id), nil, in)
}

// DeleteInboxChannel removes one of the caller's channels.
func (c *Client) DeleteInboxChannel(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteChannel, "id", id), nil, nil)
}

// TestInboxChannel sends a test message to one of the caller's channels.
func (c *Client) TestInboxChannel(ctx context.Context, id string) (*core.ChannelDelivery, error) {
	return call[core.ChannelDelivery](ctx, c, http.MethodPost, routePath(wire.RouteChannelTest, "id", id), nil, nil)
}

// ListChannelDeliveries returns a page of one channel's deliveries.
func (c *Client) ListChannelDeliveries(ctx context.Context, f core.ChannelDeliveryFilter) ([]core.ChannelDelivery, string, error) {
	return list[core.ChannelDelivery](ctx, c, routePath(wire.RouteChannelDeliveries, "id", f.ChannelID), pageQuery(f.Page))
}

// RedeliverChannelDelivery queues another attempt of a channel delivery.
func (c *Client) RedeliverChannelDelivery(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodPost, routePath(wire.RouteChannelRedeliver, "id", id), nil, nil)
}
