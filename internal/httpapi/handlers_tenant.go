// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerTenantRoutes binds tenants, their domains and their membership.
func (rt *Router) registerTenantRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteTenants, rt.handleListTenants)
	rt.mux.HandleFunc("POST "+wire.RouteTenants, rt.handleCreateTenant)
	rt.mux.HandleFunc("GET "+wire.RouteTenant, rt.handleGetTenant)
	rt.mux.HandleFunc("PATCH "+wire.RouteTenant, rt.handleUpdateTenant)
	rt.mux.HandleFunc("DELETE "+wire.RouteTenant, rt.handleDeleteTenant)

	rt.mux.HandleFunc("GET "+wire.RouteMembers, rt.handleListMembers)
	rt.mux.HandleFunc("POST "+wire.RouteMembers, rt.handleAddMember)
	rt.mux.HandleFunc("DELETE "+wire.RouteMember, rt.handleRemoveMember)

	rt.mux.HandleFunc("GET "+wire.RouteDomains, rt.handleListDomains)
	rt.mux.HandleFunc("POST "+wire.RouteDomains, rt.handleAddDomain)
	rt.mux.HandleFunc("GET "+wire.RouteDomain, rt.handleResolveDomain)
	rt.mux.HandleFunc("DELETE "+wire.RouteDomain, rt.handleRemoveDomain)
}

// handleListTenants returns a page of tenants.
func (rt *Router) handleListTenants(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	tenants, next, err := rt.cfg.Service.ListTenants(r.Context(), page)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, tenants, next)
}

// handleCreateTenant creates a tenant.
func (rt *Router) handleCreateTenant(w http.ResponseWriter, r *http.Request) {
	var in core.CreateTenantInput
	if !readJSON(w, r, &in) {
		return
	}
	tenant, err := rt.cfg.Service.CreateTenant(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, tenant)
}

// tenantRef reads the tenant reference from the path.
//
// wire.TenantSelf becomes the empty reference the service contract already
// understands as "the caller's own tenant". A URL cannot carry an empty path
// segment, so the client sends the stand-in and it is undone here, before the
// service sees it: the convention stays the service's, not the transport's.
func tenantRef(r *http.Request) string {
	ref := r.PathValue("ref")
	if ref == wire.TenantSelf {
		return ""
	}
	return ref
}

// handleGetTenant returns one tenant.
func (rt *Router) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	tenant, err := rt.cfg.Service.GetTenant(r.Context(), tenantRef(r))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tenant)
}

// handleUpdateTenant changes one tenant.
func (rt *Router) handleUpdateTenant(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateTenantInput
	if !readJSON(w, r, &in) {
		return
	}
	tenant, err := rt.cfg.Service.UpdateTenant(r.Context(), tenantRef(r), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tenant)
}

// handleDeleteTenant removes one tenant.
func (rt *Router) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteTenant(r.Context(), tenantRef(r)); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListMembers returns the membership of the resolved tenant.
func (rt *Router) handleListMembers(w http.ResponseWriter, r *http.Request) {
	members, err := rt.cfg.Service.ListMembers(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, members, "")
}

// handleAddMember grants an actor a role.
func (rt *Router) handleAddMember(w http.ResponseWriter, r *http.Request) {
	var in AddMemberRequest
	if !readJSON(w, r, &in) {
		return
	}
	member, err := rt.cfg.Service.AddMember(r.Context(), in.ActorID, in.Role)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, member)
}

// handleRemoveMember revokes an actor's membership.
func (rt *Router) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RemoveMember(r.Context(), r.PathValue("actorID")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListDomains returns the hostnames mapped to the resolved tenant.
func (rt *Router) handleListDomains(w http.ResponseWriter, r *http.Request) {
	domains, err := rt.cfg.Service.ListDomains(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, domains, "")
}

// handleAddDomain maps a hostname to the resolved tenant.
func (rt *Router) handleAddDomain(w http.ResponseWriter, r *http.Request) {
	var in core.AddDomainInput
	if !readJSON(w, r, &in) {
		return
	}
	domain, err := rt.cfg.Service.AddDomain(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, domain)
}

// handleResolveDomain maps a hostname to the tenant this request is already
// scoped to.
//
// The service call underneath is deliberately unscoped, because the server
// resolves the Host header before any credential exists. That path is an
// in-process call from the middleware. Reached as a route it is merely
// authenticated, so answering it unscoped would let an actor holding nothing
// but task:read in one tenant read another tenant's record. A hostname of
// another tenant is therefore reported missing, exactly as an unmapped one is.
func (rt *Router) handleResolveDomain(w http.ResponseWriter, r *http.Request) {
	hostname := r.PathValue("hostname")
	tenant, err := rt.cfg.Service.ResolveDomain(r.Context(), hostname)
	if err != nil {
		WriteError(w, err)
		return
	}
	scope, ok := core.TenantFrom(r.Context())
	if !ok || tenant == nil || tenant.ID != scope.TenantID {
		WriteError(w, core.NotFound("domain %q", hostname))
		return
	}
	WriteJSON(w, http.StatusOK, tenant)
}

// handleRemoveDomain unmaps a hostname.
func (rt *Router) handleRemoveDomain(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RemoveDomain(r.Context(), r.PathValue("hostname")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
