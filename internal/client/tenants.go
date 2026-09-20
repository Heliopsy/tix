package client

import (
	"context"
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// CreateTenant creates a tenant.
func (c *Client) CreateTenant(ctx context.Context, in core.CreateTenantInput) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodPost, httpapi.RouteTenants, nil, in)
}

// GetTenant returns one tenant.
func (c *Client) GetTenant(ctx context.Context, ref string) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodGet, routePath(httpapi.RouteTenant, "ref", ref), nil, nil)
}

// ListTenants returns one page of tenants.
func (c *Client) ListTenants(ctx context.Context, page core.Page) ([]core.Tenant, string, error) {
	return list[core.Tenant](ctx, c, httpapi.RouteTenants, pageQuery(page))
}

// UpdateTenant changes a tenant.
func (c *Client) UpdateTenant(ctx context.Context, ref string, in core.UpdateTenantInput) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodPatch, routePath(httpapi.RouteTenant, "ref", ref), nil, in)
}

// DeleteTenant removes a tenant.
func (c *Client) DeleteTenant(ctx context.Context, ref string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteTenant, "ref", ref), nil, nil)
}

// AddDomain maps a hostname to the current tenant.
func (c *Client) AddDomain(ctx context.Context, in core.AddDomainInput) (*core.Domain, error) {
	return call[core.Domain](ctx, c, http.MethodPost, httpapi.RouteDomains, nil, in)
}

// ListDomains returns the tenant's domains.
func (c *Client) ListDomains(ctx context.Context) ([]core.Domain, error) {
	return listAll[core.Domain](ctx, c, httpapi.RouteDomains, nil)
}

// RemoveDomain unmaps a hostname.
func (c *Client) RemoveDomain(ctx context.Context, hostname string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteDomain, "hostname", hostname), nil, nil)
}

// ResolveDomain returns the tenant a hostname belongs to.
func (c *Client) ResolveDomain(ctx context.Context, hostname string) (*core.Tenant, error) {
	return call[core.Tenant](ctx, c, http.MethodGet, routePath(httpapi.RouteDomain, "hostname", hostname), nil, nil)
}

// AddMember grants an actor a role in the current tenant.
func (c *Client) AddMember(ctx context.Context, actorID string, role core.Role) (*core.Membership, error) {
	body := struct {
		ActorID string    `json:"actor_id"`
		Role    core.Role `json:"role"`
	}{ActorID: actorID, Role: role}
	return call[core.Membership](ctx, c, http.MethodPost, httpapi.RouteMembers, nil, body)
}

// ListMembers returns the tenant's memberships.
func (c *Client) ListMembers(ctx context.Context) ([]core.Membership, error) {
	return listAll[core.Membership](ctx, c, httpapi.RouteMembers, nil)
}

// RemoveMember revokes a membership.
func (c *Client) RemoveMember(ctx context.Context, actorID string) error {
	return callVoid(ctx, c, http.MethodDelete, routePath(httpapi.RouteMember, "actorID", actorID), nil, nil)
}
