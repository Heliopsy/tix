// SPDX-License-Identifier: AGPL-3.0-or-later

package web

import (
	"net/http"

	"github.com/heliopsy/tix/internal/core"
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

// webhooksView is what the webhook screen renders. No stored signing secret
// reaches it, because the endpoint record does not carry one outward; the one
// exception is Secret below, which is a value the store has never disclosed.
type webhooksView struct {
	Endpoints  []core.WebhookEndpoint
	Deliveries []core.WebhookDelivery

	// Secret is a signing secret tix generated for an endpoint just saved,
	// shown once because nothing can read it back afterwards. It is nil
	// whenever the operator supplied their own, since then tix knows nothing
	// the operator does not.
	Secret *oneTimeSecret

	// Pager is the delivery log's position control. The endpoint table above
	// it is not paginated -- ListWebhooks returns the tenant's endpoints
	// whole -- so the one control on this screen belongs to the log.
	Pager pager
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
		Page: core.Page{Cursor: r.URL.Query().Get(CursorParam), Limit: h.rowsPerPage(r)}})
	if err != nil {
		return err
	}
	return h.render(w, r, "webhooks.html", "Webhooks",
		webhooksView{Endpoints: endpoints, Deliveries: deliveries,
			Secret: generatedWebhookSecret(generatedSecret(w, r)),
			Pager:  newPager(r, RouteWebhooks, next, len(deliveries), "deliveries", SizeParam)})
}

// generatedWebhookSecretCookie carries a generated signing secret to the
// screen that shows it once, so the value never appears in a URL or in the
// audit trail.
const generatedWebhookSecretCookie = "tix_webhook_secret"

// generatedSecret reads and clears the one-time signing secret cookie.
func generatedSecret(w http.ResponseWriter, r *http.Request) string {
	return readOneTimeCookie(w, r, generatedWebhookSecretCookie, RouteWebhooks)
}

// putWebhook registers or updates a delivery endpoint, and carries a secret
// tix generated to the screen that shows it once.
//
// The service returns a secret only when it minted one, which is only when the
// operator left the field blank. An operator who supplied their own is shown
// nothing: they already hold it, and echoing it back would put a value they
// chose into a response for no gain.
func (h *handler) putWebhook(w http.ResponseWriter, r *http.Request) error {
	in := core.WebhookInput{
		ID:         field(r, "id"),
		URL:        field(r, "url"),
		Secret:     r.PostFormValue("secret"),
		EventTypes: fieldList(r, "event_types"),
		Active:     checked(r, "active"),
	}
	saved, err := h.svc.PutWebhook(r.Context(), in)
	if err != nil {
		return err
	}
	if saved != nil && saved.Secret != "" {
		// #nosec G124 -- one-time value, HttpOnly, cleared by the screen that shows it
		http.SetCookie(w, oneTimeCookie(generatedWebhookSecretCookie, RouteWebhooks,
			saved.Secret, h.secureCookie(r)))
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
