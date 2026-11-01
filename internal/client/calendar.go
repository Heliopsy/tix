// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// GetCalendarFeed returns the caller's feed, without its secret.
func (c *Client) GetCalendarFeed(ctx context.Context) (*core.CalendarFeed, error) {
	return call[core.CalendarFeed](ctx, c, http.MethodGet, wire.RouteCalendar, nil, nil)
}

// CreateCalendarFeed issues the caller a feed.
func (c *Client) CreateCalendarFeed(ctx context.Context, in core.CalendarFeedInput) (*core.IssuedCalendarFeed, error) {
	return call[core.IssuedCalendarFeed](ctx, c, http.MethodPost, wire.RouteCalendar, nil, in)
}

// RotateCalendarFeed replaces the caller's feed secret.
func (c *Client) RotateCalendarFeed(ctx context.Context) (*core.IssuedCalendarFeed, error) {
	return call[core.IssuedCalendarFeed](ctx, c, http.MethodPost, wire.RouteCalendarRotate, nil, nil)
}

// SetCalendarFeedFilter changes the caller's feed filter.
func (c *Client) SetCalendarFeedFilter(ctx context.Context, in core.CalendarFeedInput) (*core.CalendarFeed, error) {
	return call[core.CalendarFeed](ctx, c, http.MethodPatch, wire.RouteCalendar, nil, in)
}

// RevokeCalendarFeed stops the caller's feed.
func (c *Client) RevokeCalendarFeed(ctx context.Context) error {
	return callVoid(ctx, c, http.MethodDelete, wire.RouteCalendar, nil, nil)
}

// ReadCalendarFeed is refused: the server answers a feed as iCalendar to a
// calendar app at its address, not as a record to this client.
func (c *Client) ReadCalendarFeed(context.Context, string) (*core.CalendarEntries, error) {
	return nil, core.Invalid("a calendar feed is read by a calendar app at its address; open the url it was issued with")
}
