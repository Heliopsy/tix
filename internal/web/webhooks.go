package web

import (
	"net/http"

	"github.com/thereisnotime/tix/internal/core"
)

// webhookRoutes are the webhook endpoint and delivery log screens.
func (h *handler) webhookRoutes() []route {
	return []route{
		get(RouteWebhooks, "webhooks.html", h.showWebhooks, "ListWebhooks", "ListDeliveries"),
		post(RouteWebhooks, h.putWebhook, "PutWebhook"),
		post(RouteWebhookDelete, h.deleteWebhook, "DeleteWebhook"),
		post(RouteRedeliver, h.redeliver, "RedeliverWebhook"),
	}
}

// webhooksView is what the webhook screen renders. No signing secret reaches
// it, because the endpoint record does not carry one outward.
type webhooksView struct {
	Endpoints  []core.WebhookEndpoint
	Deliveries []core.WebhookDelivery
	NextCursor string
}

// showWebhooks renders the endpoints and the delivery log.
func (h *handler) showWebhooks(w http.ResponseWriter, r *http.Request) error {
	endpoints, err := h.svc.ListWebhooks(r.Context())
	if err != nil {
		return err
	}
	for i := range endpoints {
		endpoints[i].Secret = ""
	}
	deliveries, next, err := h.svc.ListDeliveries(r.Context(), core.DeliveryFilter{
		Page: core.Page{Cursor: r.URL.Query().Get("cursor")}})
	if err != nil {
		return err
	}
	return h.render(w, r, "webhooks.html", "Webhooks",
		webhooksView{Endpoints: endpoints, Deliveries: deliveries, NextCursor: next})
}

// putWebhook registers or updates a delivery endpoint.
func (h *handler) putWebhook(w http.ResponseWriter, r *http.Request) error {
	in := core.WebhookInput{
		ID:         field(r, "id"),
		URL:        field(r, "url"),
		Secret:     r.PostFormValue("secret"),
		EventTypes: fieldList(r, "event_types"),
		Active:     checked(r, "active"),
	}
	if _, err := h.svc.PutWebhook(r.Context(), in); err != nil {
		return err
	}
	redirect(w, r, RouteWebhooks, "endpoint saved")
	return nil
}

// deleteWebhook removes a delivery endpoint.
func (h *handler) deleteWebhook(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.DeleteWebhook(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteWebhooks, "endpoint deleted")
	return nil
}

// redeliver queues a fresh attempt for one delivery.
func (h *handler) redeliver(w http.ResponseWriter, r *http.Request) error {
	if err := h.svc.RedeliverWebhook(r.Context(), field(r, "id")); err != nil {
		return err
	}
	redirect(w, r, RouteWebhooks, "redelivery queued")
	return nil
}
