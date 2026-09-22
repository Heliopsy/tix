package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// EnrolSSHKey registers a public key against an actor.
func (c *Client) EnrolSSHKey(ctx context.Context, in core.EnrolSSHKeyInput) (*core.SSHKey, error) {
	return call[core.SSHKey](ctx, c, http.MethodPost, httpapi.RouteSSHKeys, nil, in)
}

// ListSSHKeys returns an actor's enrolled keys, revoked ones included.
func (c *Client) ListSSHKeys(ctx context.Context, actorID string) ([]core.SSHKey, error) {
	q := url.Values{}
	if actorID != "" {
		q.Set("actor_id", actorID)
	}
	return listAll[core.SSHKey](ctx, c, httpapi.RouteSSHKeys, q)
}

// RevokeSSHKey stops an enrolled key authenticating.
func (c *Client) RevokeSSHKey(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteSSHKey, "id", id), nil, nil)
}
