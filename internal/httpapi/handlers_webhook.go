package httpapi

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

// registerWebhookRoutes binds delivery endpoints and their queue.
func (rt *Router) registerWebhookRoutes() {
	rt.mux.HandleFunc("GET "+wire.RouteWebhooks, rt.handleListWebhooks)
	rt.mux.HandleFunc("PUT "+wire.RouteWebhooks, rt.handlePutWebhook)
	rt.mux.HandleFunc("DELETE "+wire.RouteWebhook, rt.handleDeleteWebhook)
	rt.mux.HandleFunc("GET "+wire.RouteDeliveries, rt.handleListDeliveries)
	rt.mux.HandleFunc("POST "+wire.RouteDeliveryRedeliver, rt.handleRedeliver)
}

// handleListWebhooks returns the tenant's delivery endpoints.
func (rt *Router) handleListWebhooks(w http.ResponseWriter, r *http.Request) {
	hooks, err := rt.cfg.Service.ListWebhooks(r.Context())
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, hooks, "")
}

// handlePutWebhook registers or updates a delivery endpoint.
func (rt *Router) handlePutWebhook(w http.ResponseWriter, r *http.Request) {
	var in core.WebhookInput
	if !readJSON(w, r, &in) {
		return
	}
	hook, err := rt.cfg.Service.PutWebhook(r.Context(), in)
	if err != nil {
		WriteError(w, err)
		return
	}
	WriteJSON(w, http.StatusOK, hook)
}

// handleDeleteWebhook removes a delivery endpoint.
func (rt *Router) handleDeleteWebhook(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.DeleteWebhook(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}

// handleListDeliveries returns a page of delivery attempts.
func (rt *Router) handleListDeliveries(w http.ResponseWriter, r *http.Request) {
	page, err := pageFrom(r)
	if err != nil {
		WriteError(w, err)
		return
	}
	f := core.DeliveryFilter{EndpointID: r.URL.Query().Get("endpoint_id"), Page: page}
	for _, s := range r.URL.Query()["status"] {
		f.Statuses = append(f.Statuses, core.DeliveryStatus(s))
	}
	deliveries, next, err := rt.cfg.Service.ListDeliveries(r.Context(), f)
	if err != nil {
		WriteError(w, err)
		return
	}
	writeList(w, r, deliveries, next)
}

// handleRedeliver queues one delivery for another attempt.
func (rt *Router) handleRedeliver(w http.ResponseWriter, r *http.Request) {
	if err := rt.cfg.Service.RedeliverWebhook(r.Context(), r.PathValue("id")); err != nil {
		WriteError(w, err)
		return
	}
	writeNoContent(w)
}
