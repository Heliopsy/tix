// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerInboxRoutes binds the caller's own inbox.
func (rt *Router) registerInboxRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteInbox, rt.handleListInbox)
	rt.mux.HandleFunc("GET "+wire.RouteInboxCount, rt.handleCountInbox)
	rt.mux.HandleFunc("PUT "+wire.RouteInboxRead, rt.handleReadInboxItem)
	rt.mux.HandleFunc("DELETE "+wire.RouteInboxRead, rt.handleUnreadInboxItem)
	rt.mux.HandleFunc("POST "+wire.RouteInboxReadAll, rt.handleReadAllInbox)
	rt.mux.HandleFunc("GET "+wire.RouteInboxPrefs, rt.handleGetInboxPrefs)
	rt.mux.HandleFunc("PUT "+wire.RouteInboxPrefs, rt.handlePutInboxPrefs)
}

// handleListInbox returns a page of the caller's items, newest first.
func (rt *Router) handleListInbox(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	items, next, err := rt.cfg.Service.ListInbox(r.Context(), core.InboxFilter{Unread: boolParam(r, "unread"), Page: page})
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, items, next)
}

// handleCountInbox counts the caller's unread items.
func (rt *Router) handleCountInbox(w http.ResponseWriter, r *http.Request) {
	n, err := rt.cfg.Service.CountInbox(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, n)
}

// handleReadInboxItem marks one of the caller's items read.
func (rt *Router) handleReadInboxItem(w http.ResponseWriter, r *http.Request) {
	item, err := rt.cfg.Service.ReadInboxItem(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, item)
}

// handleUnreadInboxItem marks one of the caller's items unread.
func (rt *Router) handleUnreadInboxItem(w http.ResponseWriter, r *http.Request) {
	item, err := rt.cfg.Service.UnreadInboxItem(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, item)
}

// handleReadAllInbox marks every one of the caller's items read.
func (rt *Router) handleReadAllInbox(w http.ResponseWriter, r *http.Request) {
	out, err := rt.cfg.Service.ReadAllInbox(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, out)
}

// handleGetInboxPrefs returns the kinds the caller muted.
func (rt *Router) handleGetInboxPrefs(w http.ResponseWriter, r *http.Request) {
	prefs, err := rt.cfg.Service.GetInboxPrefs(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, prefs)
}

// handlePutInboxPrefs replaces the kinds the caller muted.
func (rt *Router) handlePutInboxPrefs(w http.ResponseWriter, r *http.Request) {
	var in core.InboxPrefsInput
	if !readJSON(w, r, &in) {
		return
	}
	prefs, err := rt.cfg.Service.PutInboxPrefs(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, prefs)
}
