// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/wire"
)

// registerStatusRoutes binds the installation read.
func (rt *Router) registerStatusRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteStatus, rt.handleStatus)
}

// handleStatus reports what the installation holds and what is running in it.
func (rt *Router) handleStatus(w http.ResponseWriter, r *http.Request) {
	report, err := rt.cfg.Service.Status(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, report)
}
