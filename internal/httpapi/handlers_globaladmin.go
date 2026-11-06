// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerGlobalAdminRoutes binds the installation's list of global admins.
func (rt *Router) registerGlobalAdminRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteGlobalAdmins, rt.handleListGlobalAdmins)
	rt.mux.HandleFunc("POST "+wire.RouteGlobalAdmins, rt.handleGrantGlobalAdmin)
	rt.mux.HandleFunc("DELETE "+wire.RouteGlobalAdmin, rt.handleRevokeGlobalAdmin)

	rt.mux.HandleFunc("GET "+wire.RouteTenantUsers, rt.handleListTenantUsers)
	rt.mux.HandleFunc("POST "+wire.RouteTenantUsers, rt.handleCreateTenantUser)
	rt.mux.HandleFunc("PATCH "+wire.RouteTenantUser, rt.handleUpdateTenantUser)
	rt.mux.HandleFunc("DELETE "+wire.RouteTenantUser, rt.handleDeleteTenantUser)
}

// handleListTenantUsers returns a page of a tenant's users.
func (rt *Router) handleListTenantUsers(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	users, next, err := rt.cfg.Service.ListTenantUsers(r.Context(), tenantRef(r), page)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, users, next)
}

// handleCreateTenantUser creates a user in a tenant.
func (rt *Router) handleCreateTenantUser(w http.ResponseWriter, r *http.Request) {
	var in core.CreateUserInput
	if !readJSON(w, r, &in) {
		return
	}
	user, err := rt.cfg.Service.CreateTenantUser(r.Context(), tenantRef(r), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, user)
}

// handleUpdateTenantUser changes a user of a tenant.
func (rt *Router) handleUpdateTenantUser(w http.ResponseWriter, r *http.Request) {
	var in core.UpdateUserInput
	if !readJSON(w, r, &in) {
		return
	}
	user, err := rt.cfg.Service.UpdateTenantUser(r.Context(), tenantRef(r), r.PathValue("id"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, user)
}

// handleDeleteTenantUser deletes a user of a tenant.
func (rt *Router) handleDeleteTenantUser(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteTenantUser(r.Context(), tenantRef(r), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListGlobalAdmins returns every global admin.
func (rt *Router) handleListGlobalAdmins(w http.ResponseWriter, r *http.Request) {
	users, err := rt.cfg.Service.ListGlobalAdmins(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, users, "")
}

// handleGrantGlobalAdmin makes a user a global admin.
func (rt *Router) handleGrantGlobalAdmin(w http.ResponseWriter, r *http.Request) {
	var in core.GlobalAdminInput
	if !readJSON(w, r, &in) {
		return
	}
	user, err := rt.cfg.Service.GrantGlobalAdmin(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, user)
}

// handleRevokeGlobalAdmin takes global admin away from a user.
func (rt *Router) handleRevokeGlobalAdmin(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeGlobalAdmin(r.Context(), r.PathValue("email")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
