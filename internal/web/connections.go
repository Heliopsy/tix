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

// connectionRow is one live connection as the screen shows it. Mine marks a
// connection the signed-in actor holds, because ending one of those cuts the
// reader's own live updates and that is worth knowing before clicking.
type connectionRow struct {
	core.Connection
	Mine bool
}

// connectionsView is what the live connection screen renders. It carries this
// server's connections and no other's, which is why the server is named on it.
type connectionsView struct {
	ServerID    string
	Connections []connectionRow
	Counts      core.ConnectionCounts
}

// showConnections renders what this server is holding for the tenant.
func (h *handler) showConnections(w http.ResponseWriter, r *http.Request) error {
	list, err := h.svc.ListConnections(r.Context())
	if err != nil {
		return err
	}
	actor, _ := core.ActorFrom(r.Context())
	rows := make([]connectionRow, 0, len(list.Connections))
	for _, conn := range list.Connections {
		row := connectionRow{Connection: conn}
		if actor != nil && actor.ID == conn.ActorID {
			row.Mine = true
		}
		rows = append(rows, row)
	}
	return h.render(w, r, "connections.html", "Connections",
		connectionsView{ServerID: list.ServerID, Connections: rows, Counts: list.Counts})
}

// endConnection closes one live connection.
func (h *handler) endConnection(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.EndConnection(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteConnections, "connection ended")
	return nil
}
