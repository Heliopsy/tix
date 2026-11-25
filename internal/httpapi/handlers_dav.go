// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerDAVRoutes binds the caller's app passwords and what a client is
// configured with. CalDAV itself is served by Config.CalDAV under core.DAVPath.
func (rt *Router) registerDAVRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteDAV, rt.handleGetDAVInfo)
	rt.mux.HandleFunc("GET "+wire.RouteDAVPasswords, rt.handleListDAVPasswords)
	rt.mux.HandleFunc("POST "+wire.RouteDAVPasswords, rt.handleCreateDAVPassword)
	rt.mux.HandleFunc("DELETE "+wire.RouteDAVPassword, rt.handleRevokeDAVPassword)
}

// handleGetDAVInfo answers whether CalDAV is on, its absolute address and the
// username to sign in with.
func (rt *Router) handleGetDAVInfo(w http.ResponseWriter, r *http.Request) {
	info, err := rt.cfg.Service.GetDAVInfo(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	info.URL = Origin(r) + info.Path
	WriteJSON(w, http.StatusOK, info)
}

// handleListDAVPasswords lists the caller's app passwords as metadata.
func (rt *Router) handleListDAVPasswords(w http.ResponseWriter, r *http.Request) {
	list, err := rt.cfg.Service.ListDAVPasswords(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, list, "")
}

// handleCreateDAVPassword issues an app password and answers its secret once.
func (rt *Router) handleCreateDAVPassword(w http.ResponseWriter, r *http.Request) {
	var in core.CreateDAVPasswordInput
	if !readJSON(w, r, &in) {
		return
	}
	issued, err := rt.cfg.Service.CreateDAVPassword(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	issued.URL = Origin(r) + issued.Path
	WriteJSON(w, http.StatusCreated, issued)
}

// handleRevokeDAVPassword revokes one of the caller's app passwords.
func (rt *Router) handleRevokeDAVPassword(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeDAVPassword(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleDAVWellKnown points a client that knows only the server at the
// CalDAV context root, as RFC 6764 asks, without needing a credential.
func handleDAVWellKnown(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Location", core.DAVRoot)
	w.WriteHeader(http.StatusMovedPermanently)
}
