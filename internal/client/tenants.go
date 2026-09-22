package client

import (
	"context"
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// CreateTenant creates a tenant.
func (c *Client) CreateTenant(ctx context.Context, in core.CreateTenantInput) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodPost, wire.RouteTenants, nil, in)
}

// GetTenant returns one tenant.
func (c *Client) GetTenant(ctx context.Context, ref string) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodGet, routePath(wire.RouteTenant, "ref", tenantRef(ref)), nil, nil)
}

// tenantRef renders a tenant reference for a URL.
//
// The contract lets an empty reference mean the caller's own tenant, which no
// path segment can carry, so it travels as wire.TenantSelf and the server turns
// it back. This is transport spelling, not a rule: the meaning is decided by the
// service on the other side exactly as it is for a direct caller.
func tenantRef(ref string) string {
	if strings.TrimSpace(ref) == "" {
		return wire.TenantSelf
	}
	return ref
}

// ListTenants returns one page of tenants.
func (c *Client) ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, string, error) {
	return list[core.Tenant](ctx, c, wire.RouteTenants, pageQuery(page))
}

// UpdateTenant changes a tenant.
func (c *Client) UpdateTenant(ctx context.Context, ref string, in core.UpdateTenantInput) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodPatch, routePath(wire.RouteTenant, "ref", tenantRef(ref)), nil, in)
}

// DeleteTenant removes a tenant.
func (c *Client) DeleteTenant(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteTenant, "ref", tenantRef(ref)), nil, nil)
}

// AddDomain maps a hostname to the current tenant.
func (c *Client) AddDomain(ctx context.Context, in core.AddDomainInput) (*core.Domain, error) {
	return call[core.Domain](ctx, c, http.MethodPost, wire.RouteDomains, nil, in)
}

// ListDomains returns the tenant's domains.
func (c *Client) ListDomains(ctx context.Context) ([]core.Domain, error) {
	return listAll[core.Domain](ctx, c, wire.RouteDomains, nil)
}

// RemoveDomain unmaps a hostname.
func (c *Client) RemoveDomain(ctx context.Context, hostname string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteDomain, "hostname", hostname), nil, nil)
}

// ResolveDomain returns the tenant a hostname belongs to.
func (c *Client) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodGet, routePath(wire.RouteDomain, "hostname", hostname), nil, nil)
}

// AddMember grants an actor a role in the current tenant.
func (c *Client) AddMember(ctx context.Context, actorID string, role core.Role) (*core.Membership, error) {
	body := struct {
		ActorID string    `json:"actor_id"`
		Role    core.Role `json:"role"`
	}{ActorID: actorID, Role: role}
	return call[core.Membership](ctx, c, http.MethodPost, wire.RouteMembers, nil, body)
}

// ListMembers returns the tenant's memberships.
func (c *Client) ListMembers(ctx context.Context) ([]core.Membership, error) {
	return listAll[core.Membership](ctx, c, wire.RouteMembers, nil)
}

// RemoveMember revokes a membership.
func (c *Client) RemoveMember(ctx context.Context, actorID string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(wire.RouteMember, "actorID", actorID), nil, nil)
}
