// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"
	"net/url"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// EnrolSSHKey registers a public key against an actor.
func (c *Client) EnrolSSHKey(ctx context.Context, in core.EnrolSSHKeyInput) (*core.SSHKey, error) {
	return call[core.SSHKey](ctx, c, http.MethodPost, wire.RouteSSHKeys, nil, in)
}

// ListSSHKeys returns an actor's enrolled keys, revoked ones included.
func (c *Client) ListSSHKeys(ctx context.Context, actorID string) ([]core.SSHKey, error) {
	q := url.Values{}
	if actorID != "" {
		q.Set("actor_id", actorID)
	}
	return listAll[core.SSHKey](ctx, c, wire.RouteSSHKeys, q)
}

// RevokeSSHKey stops an enrolled key authenticating.
func (c *Client) RevokeSSHKey(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteSSHKey, "id", id), nil, nil)
}
