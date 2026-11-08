// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
)

// RouteSessionRevoke ends one of the reader's own sessions from the settings
// screen's Sessions section.
const RouteSessionRevoke = "/settings/sessions/revoke"

// sessionsSection is the settings screen's anchor for the reader's sessions.
const sessionsSection = "sessions"

// sessionsShown bounds how many sessions the section draws. More than that is
// a question for tix account sessions --cursor, and the section says so.
const sessionsShown = 100

// sessionRevokeRoutes is the Sessions section's one write.
func (h *handler) sessionRevokeRoutes() []route {
	return []route{post(RouteSessionRevoke, h.revokeSession, "RevokeSession")}
}

// sessionsView is the reader's own sessions. Available is false when the
// caller is not one who holds sessions, as in no-auth mode.
type sessionsView struct {
	Available bool
	Sessions  []core.SessionInfo
	More      bool
}

// sessionsFor reads the Sessions section.
func (h *handler) sessionsFor(r *http.Request) (sessionsView, error) {
	found, next, err := h.svc.ListSessions(r.Context(), core.Page{Limit: sessionsShown})
	if core.IsKind(err, core.KindInvalid) || core.IsKind(err, core.KindForbidden) {
		return sessionsView{}, nil
	}
	if err != nil {
		return sessionsView{}, err
	}
	return sessionsView{Available: true, Sessions: found, More: next != ""}, nil
}

// revokeSession ends the session the form names. Ending the one in use signs
// the reader out, and the redirect lands on the sign-in screen.
func (h *handler) revokeSession(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RevokeSession(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteSettings+"#"+sessionsSection, "session revoked")
	return nil
}
