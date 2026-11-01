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

	rt.mux.HandleFunc("GET "+wire.RouteChannels, rt.handleListChannels)
	rt.mux.HandleFunc("POST "+wire.RouteChannels, rt.handleCreateChannel)
	rt.mux.HandleFunc("PATCH "+wire.RouteChannel, rt.handleUpdateChannel)
	rt.mux.HandleFunc("DELETE "+wire.RouteChannel, rt.handleDeleteChannel)
	rt.mux.HandleFunc("POST "+wire.RouteChannelTest, rt.handleTestChannel)
	rt.mux.HandleFunc("GET "+wire.RouteChannelDeliveries, rt.handleListChannelDeliveries)
	rt.mux.HandleFunc("POST "+wire.RouteChannelRedeliver, rt.handleRedeliverChannel)
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

// handleListChannels returns the caller's notification channels.
func (rt *Router) handleListChannels(w http.ResponseWriter, r *http.Request) {
	channels, err := rt.cfg.Service.ListInboxChannels(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, channels, "")
}

// handleCreateChannel adds a channel; the answer is the only one carrying its secret.
func (rt *Router) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	var in core.InboxChannelInput
	if !readJSON(w, r, &in) {
		return
	}
	issued, err := rt.cfg.Service.CreateInboxChannel(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusCreated, issued)
}

// handleUpdateChannel changes one of the caller's channels.
func (rt *Router) handleUpdateChannel(w http.ResponseWriter, r *http.Request) {
	var in core.InboxChannelUpdate
	if !readJSON(w, r, &in) {
		return
	}
	c, err := rt.cfg.Service.UpdateInboxChannel(r.Context(), r.PathValue("id"), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, c)
}

// handleDeleteChannel removes one of the caller's channels.
func (rt *Router) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteInboxChannel(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleTestChannel sends a test message to one of the caller's channels.
func (rt *Router) handleTestChannel(w http.ResponseWriter, r *http.Request) {
	d, err := rt.cfg.Service.TestInboxChannel(r.Context(), r.PathValue("id"))
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, d)
}

// handleListChannelDeliveries returns a page of one channel's deliveries.
func (rt *Router) handleListChannelDeliveries(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	out, next, err := rt.cfg.Service.ListChannelDeliveries(r.Context(),
		core.ChannelDeliveryFilter{ChannelID: r.PathValue("id"), Page: page})
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, out, next)
}

// handleRedeliverChannel queues another attempt of a channel delivery.
func (rt *Router) handleRedeliverChannel(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RedeliverChannelDelivery(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
