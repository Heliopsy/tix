package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/wire"
)

// registerConnectionRoutes binds the live connection routes.
func (rt *Router) registerConnectionRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteConnections, rt.handleListConnections)
	rt.mux.HandleFunc("DELETE "+wire.RouteConnection, rt.handleEndConnection)
}

// handleListConnections answers with the connections this server holds for the
// caller's tenant. The body names the server, because another server against
// the same database holds connections this one knows nothing about.
func (rt *Router) handleListConnections(w http.ResponseWriter, r *http.Request) {
	list, err := rt.cfg.Service.ListConnections(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, list)
}

// handleEndConnection closes one live connection of the caller's tenant.
func (rt *Router) handleEndConnection(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.EndConnection(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
