// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// CreateInvite invites an address and returns its link once.
func (c *Client) CreateInvite(ctx context.Context, in core.CreateInviteInput) (*core.IssuedInvite, error) {
	return call[core.IssuedInvite](ctx, c, http.MethodPost, wire.RouteInvites, nil, in)
}

// ListInvites returns the pending invites.
func (c *Client) ListInvites(ctx context.Context) ([]core.Invite, error) {
	return listAll[core.Invite](ctx, c, wire.RouteInvites, nil)
}

// RevokeInvite stops a pending invite's link.
func (c *Client) RevokeInvite(ctx context.Context, id string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteInvite, "id", id), nil, nil)
}

// ReissueInvite replaces an invite's link and returns the new one once.
func (c *Client) ReissueInvite(ctx context.Context, id string, in core.ReissueInviteInput) (*core.IssuedInvite, error) {
	return call[core.IssuedInvite](ctx, c, http.MethodPost, routePath(wire.RouteInviteReissue, "id", id), nil, in)
}

// PreviewInvite describes the invite a token accepts.
func (c *Client) PreviewInvite(ctx context.Context, token string) (*core.InvitePreview, error) {
	return call[core.InvitePreview](ctx, c, http.MethodGet, routePath(wire.RouteInviteToken, "token", token), nil, nil)
}

// AcceptInvite accepts an invite and signs in.
func (c *Client) AcceptInvite(ctx context.Context, token string, in core.AcceptInviteInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, routePath(wire.RouteInviteToken, "token", token), nil, in)
}
