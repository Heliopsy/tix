// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// connectionRoutes are the live connection screen and the form that ends one.
func (h *handler) connectionRoutes() []route {
	return []route{
		get(RouteConnections, "connections.html", h.showConnections, "ListConnections"),
		post(RouteConnectionEnd, h.endConnection, "EndConnection"),
	}
}

// connectionsView is what the live connection screen renders. It carries this
// server's connections and no other's, which is why the server is named on it.
type connectionsView struct {
	ServerID    string
	Connections []core.Connection
	Counts      core.ConnectionCounts
}

// showConnections renders what this server is holding for the tenant.
func (h *handler) showConnections(w http.ResponseWriter, r *http.Request) error {
	list, err := h.svc.ListConnections(r.Context())
	if err != nil {
		return err
	}
	return h.render(w, r, "connections.html", "Connections",
		connectionsView{ServerID: list.ServerID, Connections: list.Connections, Counts: list.Counts})
}

// endConnection closes one live connection.
func (h *handler) endConnection(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.EndConnection(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteConnections, "connection ended")
	return nil
}
