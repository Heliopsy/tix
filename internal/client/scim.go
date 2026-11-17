// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"context"
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// GetSCIM reads the tenant's SCIM provisioning settings.
func (c *Client) GetSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	return call[core.SCIMSettings](ctx, c, http.MethodGet, wire.RouteSCIM, nil, nil)
}

// EnableSCIM turns SCIM provisioning on.
func (c *Client) EnableSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	return call[core.SCIMSettings](ctx, c, http.MethodPost, wire.RouteSCIMEnable, nil, nil)
}

// DisableSCIM turns SCIM provisioning off.
func (c *Client) DisableSCIM(ctx context.Context) (*core.SCIMSettings, error) {
	return call[core.SCIMSettings](ctx, c, http.MethodPost, wire.RouteSCIMDisable, nil, nil)
}

// RotateSCIMToken mints a new provisioning token, revoking the old one.
func (c *Client) RotateSCIMToken(ctx context.Context) (*core.SCIMSettings, error) {
	return call[core.SCIMSettings](ctx, c, http.MethodPost, wire.RouteSCIMToken, nil, nil)
}

// MapSCIMGroup maps a SCIM group to a tenant role.
func (c *Client) MapSCIMGroup(ctx context.Context, in core.SCIMGroupRoleInput) (*core.SCIMGroupRole, error) {
	return call[core.SCIMGroupRole](ctx, c, http.MethodPut, wire.RouteSCIMGroups, nil, in)
}

// UnmapSCIMGroup removes a SCIM group's role mapping.
func (c *Client) UnmapSCIMGroup(ctx context.Context, group string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteSCIMGroup, "group", group), nil, nil)
}
