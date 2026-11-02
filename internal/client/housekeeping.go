// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// GetHousekeeping returns the housekeeping policy, for one project when set.
func (c *Client) GetHousekeeping(ctx context.Context, project string) (*core.HousekeepingView, error) {
	q := url.Values{}
	if project != "" {
		q.Set("project", project)
	}
	return call[core.HousekeepingView](ctx, c, http.MethodGet, wire.RouteHousekeeping, q, nil)
}

// PutHousekeeping changes the tenant default or one project's override.
func (c *Client) PutHousekeeping(ctx context.Context, in core.HousekeepingInput) (*core.HousekeepingView, error) {
	return call[core.HousekeepingView](ctx, c, http.MethodPut, wire.RouteHousekeeping, nil, in)
}

// RunHousekeeping applies or previews the housekeeping policy once.
func (c *Client) RunHousekeeping(ctx context.Context, in core.HousekeepingRunInput) (*core.HousekeepingResult, error) {
	return call[core.HousekeepingResult](ctx, c, http.MethodPost, wire.RouteHousekeepingRun, nil, in)
}
