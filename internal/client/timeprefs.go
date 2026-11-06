// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// GetTimeSettings returns the caller's date format and timezone at each level.
func (c *Client) GetTimeSettings(ctx context.Context) (*core.TimeSettings, error) {
	return call[core.TimeSettings](ctx, c, http.MethodGet, wire.RouteTimeSettings, nil, nil)
}

// SetTimePrefs changes the caller's own date format and timezone.
func (c *Client) SetTimePrefs(ctx context.Context, in core.TimePrefsInput) (*core.TimeSettings, error) {
	return call[core.TimeSettings](ctx, c, http.MethodPatch, wire.RouteTimeSettings, nil, in)
}
