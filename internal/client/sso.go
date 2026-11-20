// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"encoding/xml"
	"io"
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

// RefreshSAMLMetadata reads a SAML provider's IdP metadata again.
func (c *Client) RefreshSAMLMetadata(ctx context.Context, ref string) (*core.SSOProvider, error) {
	return call[core.SSOProvider](ctx, c, http.MethodPost, routePath(wire.RouteSSOProviderRefresh, "id", ref), nil, nil)
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

// ExchangeSSOCode trades an app sign-in's one-time code for a session.
func (c *Client) ExchangeSSOCode(ctx context.Context, in core.SSOExchangeInput) (*core.LoginResult, error) {
	return call[core.LoginResult](ctx, c, http.MethodPost, wire.RouteSSOExchange, nil, in)
}

// samlMetadataLimit bounds the metadata document read back.
const samlMetadataLimit = 1 << 20

// SAMLMetadata reads a SAML provider's service provider metadata. The
// server builds it on the address the client reached it at, so BaseURL is
// not sent.
func (c *Client) SAMLMetadata(ctx context.Context, in core.SAMLMetadataInput) (*core.SAMLMetadata, error) {
	ctx, cancel := c.withTimeout(ctx)
	defer cancel()
	req, err := c.newRequest(ctx, http.MethodGet, routePath(wire.RouteSAMLMetadata, "id", in.Provider), nil, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set(wire.HeaderAccept, wire.ContentSAMLMetadata+", "+wire.ContentJSON)
	resp, err := c.send(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(io.LimitReader(resp.Body, samlMetadataLimit))
	if err != nil {
		return nil, core.Internal("reading the metadata of provider %q", in.Provider).Wrap(err)
	}
	var doc struct {
		EntityID string `xml:"entityID,attr"`
		SP       struct {
			ACS []struct {
				Location string `xml:"Location,attr"`
			} `xml:"AssertionConsumerService"`
		} `xml:"SPSSODescriptor"`
	}
	out := &core.SAMLMetadata{XML: string(data)}
	if xml.Unmarshal(data, &doc) == nil {
		out.EntityID = doc.EntityID
		if len(doc.SP.ACS) > 0 {
			out.ACSURL = doc.SP.ACS[0].Location
		}
	}
	return out, nil
}
