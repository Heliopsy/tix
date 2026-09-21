package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// registerTenantRoutes binds tenants, their domains and their membership.
func (rt *Router) registerTenantRoutes() {
	rt.mux.HandleFunc("GET "+RouteTenants, rt.handleListTenants)
	rt.mux.HandleFunc("POST "+RouteTenants, rt.handleCreateTenant)
	rt.mux.HandleFunc("GET "+RouteTenant, rt.handleGetTenant)
	rt.mux.HandleFunc("PATCH "+RouteTenant, rt.handleUpdateTenant)
	rt.mux.HandleFunc("DELETE "+RouteTenant, rt.handleDeleteTenant)

	rt.mux.HandleFunc("GET "+RouteMembers, rt.handleListMembers)
	rt.mux.HandleFunc("POST "+RouteMembers, rt.handleAddMember)
	rt.mux.HandleFunc("DELETE "+RouteMember, rt.handleRemoveMember)

	rt.mux.HandleFunc("GET "+RouteDomains, rt.handleListDomains)
	rt.mux.HandleFunc("POST "+RouteDomains, rt.handleAddDomain)
	rt.mux.HandleFunc("GET "+RouteDomain, rt.handleResolveDomain)
	rt.mux.HandleFunc("DELETE "+RouteDomain, rt.handleRemoveDomain)
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

// handleGetTenant returns one tenant.
func (rt *Router) handleGetTenant(w http.ResponseWriter, r *http.Request) {
	tenant, err := rt.cfg.Service.GetTenant(r.Context(), r.PathValue("ref"))
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
	tenant, err := rt.cfg.Service.UpdateTenant(r.Context(), r.PathValue("ref"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tenant)
}

// handleDeleteTenant removes one tenant.
func (rt *Router) handleDeleteTenant(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteTenant(r.Context(), r.PathValue("ref")); err != nil {
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

// handleRemoveDomain unmaps a hostname.
// handleResolveDomain maps a hostname to its tenant.
func (rt *Router) handleResolveDomain(w http.ResponseWriter, r *http.Request) {
	tenant, err := rt.cfg.Service.ResolveDomain(r.Context(), r.PathValue("hostname"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, tenant)
}

func (rt *Router) handleRemoveDomain(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RemoveDomain(r.Context(), r.PathValue("hostname")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
