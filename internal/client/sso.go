// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// ListSSOProviders lists the tenant's sign-in providers.
func (c *Client) ListSSOProviders(ctx context.Context) ([]core.SSOProvider, error) {
	return listAll[core.SSOProvider](ctx, c, wire.RouteSSOProviders, nil)
}

// GetSSOProvider returns one sign-in provider.
func (c *Client) GetSSOProvider(ctx context.Context, ref string) (*core.SSOProvider, error) {
	return call[core.SSOProvider](ctx, c, http.MethodGet, routePath(wire.RouteSSOProvider, "id", ref), nil, nil)
}

// CreateSSOProvider adds a sign-in provider.
func (c *Client) CreateSSOProvider(ctx context.Context, in core.CreateSSOProviderInput) (*core.SSOProvider, error) {
	return call[core.SSOProvider](ctx, c, http.MethodPost, wire.RouteSSOProviders, nil, in)
}

// UpdateSSOProvider changes a sign-in provider.
func (c *Client) UpdateSSOProvider(ctx context.Context, ref string, in core.UpdateSSOProviderInput,
) (*core.SSOProvider, error) {
	return call[core.SSOProvider](ctx, c, http.MethodPatch, routePath(wire.RouteSSOProvider, "id", ref), nil, in)
}

// DeleteSSOProvider removes a sign-in provider.
func (c *Client) DeleteSSOProvider(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteSSOProvider, "id", ref), nil, nil)
}

// SignInOptions reports what the tenant's sign-in screen offers.
func (c *Client) SignInOptions(ctx context.Context) (*core.SignInOptions, error) {
	return call[core.SignInOptions](ctx, c, http.MethodGet, wire.RouteSignInOptions, nil, nil)
}

// BeginSSOLogin starts a sign-in through a provider.
func (c *Client) BeginSSOLogin(ctx context.Context, in core.BeginSSOInput) (*core.SSOBegin, error) {
	return call[core.SSOBegin](ctx, c, http.MethodPost, wire.RouteSSOBegin, nil, in)
}

// CompleteSSOLogin finishes a sign-in through a provider.
func (c *Client) CompleteSSOLogin(ctx context.Context, in core.CompleteSSOInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, wire.RouteSSOComplete, nil, in)
}
