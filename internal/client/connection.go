package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListConnections returns the live connections the server holds for this tenant.
func (c *Client) ListConnections(ctx context.Context) (*core.ConnectionList, error) {
	return call[core.ConnectionList](ctx, c, http.MethodGet, wire.RouteConnections, nil, nil)
}

// EndConnection closes one live connection by identifier.
func (c *Client) EndConnection(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteConnection, "id", id), nil, nil)
}
