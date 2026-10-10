// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// statusRoutes serves the installation screen.
func (h *handler) statusRoutes() []route {
	return []route{
		get(RouteStatus, "status.html", h.showStatus, "Status"),
	}
}

// statusView is what status.html is executed against.
//
// Attached and Stale are counted here rather than in the template, because a
// template that counts is a template that can count differently from the
// command line, and the number of servers up is the one figure this screen
// exists for.
type statusView struct {
	Report *core.StatusReport
	// Target describes the store this server is serving, which the service
	// does not know: it is a property of how the process was pointed at the
	// database rather than of the installation.
	Target   string
	Attached int
	Stale    int
}

// showStatus renders the installation, the servers and the work.
func (h *handler) showStatus(w http.ResponseWriter, r *http.Request) error {
	report, err := held(h.svc.Status(r.Context()))
	if err != nil {
		return err
	}
	attached := report.AttachedCount()
	return h.render(w, r, "status.html", "Status", statusView{
		Report:   report,
		Target:   h.targetDescribe,
		Attached: attached,
		Stale:    len(report.Servers) - attached,
	})
}
