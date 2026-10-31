// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerViewRoutes binds the caller's own arrangement of a view's groups.
func (rt *Router) registerViewRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteViewGroups, rt.handleListGroupPrefs)
	rt.mux.HandleFunc("DELETE "+wire.RouteViewGroups, rt.handleResetGroupPrefs)
	rt.mux.HandleFunc("PATCH "+wire.RouteViewGroup, rt.handleSetGroupPref)
	rt.mux.HandleFunc("PUT "+wire.RouteViewOrder, rt.handleOrderGroups)
}

// handleListGroupPrefs returns the caller's preferences on one view.
func (rt *Router) handleListGroupPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := rt.cfg.Service.ListGroupPrefs(r.Context(), r.PathValue("view"))
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, prefs, "")
}

// handleSetGroupPref hides, collapses or stars one group for the caller.
func (rt *Router) handleSetGroupPref(w http.ResponseWriter, r *http.Request) {
	var in core.GroupPrefInput
	if !readJSON(w, r, &in) {
		return
	}
	pref, err := rt.cfg.Service.SetGroupPref(r.Context(), r.PathValue("view"), r.PathValue("group"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, pref)
}

// handleOrderGroups puts the named groups first, in the order given.
func (rt *Router) handleOrderGroups(w http.ResponseWriter, r *http.Request) {
	var in core.GroupOrderInput
	if !readJSON(w, r, &in) {
		return
	}
	prefs, err := rt.cfg.Service.OrderGroups(r.Context(), r.PathValue("view"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, prefs, "")
}

// handleResetGroupPrefs forgets the caller's preferences on a view, or on the
// one group the group query parameter names.
func (rt *Router) handleResetGroupPrefs(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.ResetGroupPrefs(r.Context(), r.PathValue("view"), r.URL.Query().Get("group")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
