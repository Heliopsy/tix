// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"
	"strings"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerInviteRoutes binds invite administration and the public routes an
// invited person previews and accepts an invite at.
func (rt *Router) registerInviteRoutes() {
	rt.mux.HandleFunc("POST "+wire.RouteInvites, rt.handleCreateInvite)
	rt.mux.HandleFunc("GET "+wire.RouteInvites, rt.handleListInvites)
	rt.mux.HandleFunc("DELETE "+wire.RouteInvite, rt.handleRevokeInvite)
	rt.mux.HandleFunc("POST "+wire.RouteInviteReissue, rt.handleReissueInvite)
	rt.mux.HandleFunc("GET "+wire.RouteInviteToken, rt.handlePreviewInvite)
	rt.mux.HandleFunc("POST "+wire.RouteInviteToken, rt.handleAcceptInvite)
}

// isInviteTokenPath reports whether a request names an invite secret, which is
// public and must never reach the request log.
func isInviteTokenPath(path string) bool {
	rest, ok := strings.CutPrefix(path, wire.InviteTokenPrefix)
	return ok && rest != "" && !strings.Contains(rest, "/")
}

// InviteURL is the absolute address of an invite link as this request reached
// the deployment.
func InviteURL(r *http.Request, token string) string {
	return origin(r) + core.InvitePath(token)
}

// handleCreateInvite invites an address and answers its link once.
func (rt *Router) handleCreateInvite(w http.ResponseWriter, r *http.Request) {
	var in core.CreateInviteInput
	if !readJSON(w, r, &in) {
		return
	}
	issued, err := rt.cfg.Service.CreateInvite(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	issued.URL = InviteURL(r, issued.Token)
	WriteJSON(w, http.StatusCreated, issued)
}

// handleListInvites returns the pending invites, without their secrets.
func (rt *Router) handleListInvites(w http.ResponseWriter, r *http.Request) {
	invites, err := rt.cfg.Service.ListInvites(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, invites, "")
}

// handleRevokeInvite stops a pending invite's link.
func (rt *Router) handleRevokeInvite(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RevokeInvite(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleReissueInvite replaces an invite's link and answers the new one once.
func (rt *Router) handleReissueInvite(w http.ResponseWriter, r *http.Request) {
	var in core.ReissueInviteInput
	if !readOptionalJSON(w, r, &in) {
		return
	}
	issued, err := rt.cfg.Service.ReissueInvite(r.Context(), r.PathValue("id"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	issued.URL = InviteURL(r, issued.Token)
	WriteJSON(w, http.StatusOK, issued)
}

// handlePreviewInvite describes the invite a secret accepts.
func (rt *Router) handlePreviewInvite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	preview, err := rt.cfg.Service.PreviewInvite(r.Context(), r.PathValue("token"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, preview)
}

// handleAcceptInvite accepts an invite and, when nothing else is needed,
// sets the session cookie. It answers as handleLogin does.
func (rt *Router) handleAcceptInvite(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	var in core.AcceptInviteInput
	if !readJSON(w, r, &in) {
		return
	}
	result, err := rt.cfg.Service.AcceptInvite(r.Context(), r.PathValue("token"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	if result.Session != nil {
		http.SetCookie(w, auth.NewSessionCookie(
			result.Session.Token, result.Session.ExpiresAt, rt.secureCookie(r)))
	}
	WriteJSON(w, http.StatusOK, result)
}
