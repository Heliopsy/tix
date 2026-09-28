// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Status reads what the installation holds and what is running in it.
func (c *Client) Status(ctx context.Context) (*core.StatusReport, error) {
	return call[core.StatusReport](ctx, c, http.MethodGet, wire.RouteStatus, nil, nil)
}
