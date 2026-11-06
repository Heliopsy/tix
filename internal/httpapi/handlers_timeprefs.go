// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerTimeRoutes binds the caller's date format and timezone.
func (rt *Router) registerTimeRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteTimeSettings, rt.handleGetTimeSettings)
	rt.mux.HandleFunc("PATCH "+wire.RouteTimeSettings, rt.handleSetTimePrefs)
}

// handleGetTimeSettings returns the caller's date format and timezone.
func (rt *Router) handleGetTimeSettings(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.GetTimeSettings(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleSetTimePrefs changes the caller's own date format and timezone.
func (rt *Router) handleSetTimePrefs(w http.ResponseWriter, r *http.Request) {
	var in core.TimePrefsInput
	if !readJSON(w, r, &in) {
		return
	}
	out, err := rt.cfg.Service.SetTimePrefs(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}
