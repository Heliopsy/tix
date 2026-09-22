package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// PutWebhook registers or updates a delivery endpoint.
func (c *Client) PutWebhook(ctx context.Context, in core.WebhookInput) (*core.WebhookEndpoint, error) {
	return call[core.WebhookEndpoint](ctx, c, http.MethodPut, wire.RouteWebhooks, nil, in)
}

// ListWebhooks returns the tenant's endpoints.
func (c *Client) ListWebhooks(ctx context.Context) ([]core.WebhookEndpoint, error) {
	return listAll[core.WebhookEndpoint](ctx, c, wire.RouteWebhooks, nil)
}

// DeleteWebhook removes an endpoint.
func (c *Client) DeleteWebhook(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteWebhook, "id", id), nil, nil)
}

// ListDeliveries returns one page of delivery records.
func (c *Client) ListDeliveries(ctx context.Context, f core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	q := pageQuery(f.Page)
	if f.EndpointID != "" {
		q.Set("endpoint_id", f.EndpointID)
	}
	for _, s := range f.Statuses {
		q.Add("status", string(s))
	}
	return list[core.WebhookDelivery](ctx, c, wire.RouteDeliveries, q)
}

// RedeliverWebhook queues a delivery for another attempt.
func (c *Client) RedeliverWebhook(ctx context.Context, deliveryID string) error {
	path := routePath(wire.RouteDeliveryRedeliver, "id", deliveryID)
	return callVoid(ctx, c, http.MethodPost, path, nil, nil)
}
