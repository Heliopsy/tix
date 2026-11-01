// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerFilterRoutes binds the caller's saved filters.
func (rt *Router) registerFilterRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteFilters, rt.handleListSavedFilters)
	rt.mux.HandleFunc("POST "+wire.RouteFilters, rt.handleCreateSavedFilter)
	rt.mux.HandleFunc("PUT "+wire.RouteFilterOrder, rt.handleOrderSavedFilters)
	rt.mux.HandleFunc("GET "+wire.RouteFilter, rt.handleGetSavedFilter)
	rt.mux.HandleFunc("PATCH "+wire.RouteFilter, rt.handleUpdateSavedFilter)
	rt.mux.HandleFunc("DELETE "+wire.RouteFilter, rt.handleDeleteSavedFilter)
}

// handleListSavedFilters returns the caller's filters and the shared ones.
func (rt *Router) handleListSavedFilters(w http.ResponseWriter, r *http.Request) {
	filters, err := rt.cfg.Service.ListSavedFilters(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, filters, "")
}

// handleGetSavedFilter returns one filter by id or name.
func (rt *Router) handleGetSavedFilter(w http.ResponseWriter, r *http.Request) {
	f, err := rt.cfg.Service.GetSavedFilter(r.Context(), r.PathValue("filter"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, f)
}

// handleCreateSavedFilter saves a new filter for the caller.
func (rt *Router) handleCreateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var in core.SavedFilterInput
	if !readJSON(w, r, &in) {
		return
	}
	f, err := rt.cfg.Service.CreateSavedFilter(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, f)
}

// handleUpdateSavedFilter renames, edits or shares a filter.
func (rt *Router) handleUpdateSavedFilter(w http.ResponseWriter, r *http.Request) {
	var in core.SavedFilterUpdate
	if !readJSON(w, r, &in) {
		return
	}
	f, err := rt.cfg.Service.UpdateSavedFilter(r.Context(), r.PathValue("filter"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, f)
}

// handleOrderSavedFilters puts the caller's named filters first.
func (rt *Router) handleOrderSavedFilters(w http.ResponseWriter, r *http.Request) {
	var in core.SavedFilterOrderInput
	if !readJSON(w, r, &in) {
		return
	}
	filters, err := rt.cfg.Service.OrderSavedFilters(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, filters, "")
}

// handleDeleteSavedFilter removes a filter.
func (rt *Router) handleDeleteSavedFilter(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteSavedFilter(r.Context(), r.PathValue("filter")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
