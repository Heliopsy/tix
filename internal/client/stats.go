// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"
	"net/url"
	"strconv"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// Stats reads throughput, ageing and the leaderboard for a window.
func (c *Client) Stats(ctx context.Context, in core.StatsInput) (*core.Stats, error) {
	q := url.Values{}
	if in.ProjectRef != "" {
		q.Set("project", in.ProjectRef)
	}
	if in.Window != 0 {
		q.Set("window", in.Window.String())
	}
	if !in.Since.IsZero() {
		setTime(q, "since", &in.Since)
	}
	if in.TopActors != 0 {
		q.Set("top", strconv.Itoa(in.TopActors))
	}
	if in.Oldest != 0 {
		q.Set("oldest", strconv.Itoa(in.Oldest))
	}
	return call[core.Stats](ctx, c, http.MethodGet, wire.RouteStats, q, nil)
}
