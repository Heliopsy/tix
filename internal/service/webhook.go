package service

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"net"
	"net/url"
	"strings"

	"github.com/thereisnotime/tix/internal/authz"
	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/store"
	sqlb "github.com/thereisnotime/tix/internal/store/sql"
	"github.com/thereisnotime/tix/internal/webhook"
)

// Event types for webhook administration.
const (
	eventWebhookPut         core.EventType = "webhook.put"
	eventWebhookDeleted     core.EventType = "webhook.deleted"
	eventWebhookRedelivered core.EventType = "webhook.redelivered"
)

// Audit actions for webhook administration.
const (
	auditWebhookPut       = "webhook.put"
	auditWebhookDelete    = "webhook.delete"
	auditWebhookRedeliver = "webhook.redeliver"
)

// webhookSecretBytes is the entropy behind a generated signing secret.
const webhookSecretBytes = 32

// webhookSubject names webhook endpoints in the audit log.
const webhookSubject = "webhook"

// deliverySubject names deliveries in the audit log.
const deliverySubject = "webhook_delivery"

var webhookSecretEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// deliverySortFields lists the fields a delivery listing may be ordered by.
var deliverySortFields = map[string]bool{"created_at": true, "next_attempt_at": true}

// newWebhookSecret mints a signing secret for an endpoint registered without
// one. The value is shown to its registrar once and is never readable again.
func newWebhookSecret() (string, error) {
	buf := make([]byte, webhookSecretBytes)
	if _, err := rand.Read(buf); err != nil {
		return "", core.Internal("generating webhook signing secret").Wrap(err)
	}
	return strings.ToLower(webhookSecretEncoding.EncodeToString(buf)), nil
}

// checkTransport refuses a plaintext target outside the loopback interface,
// because a delivery body carries task content and a signature that a network
// observer could replay. An operator with a terminating proxy opts out through
// the environment.
func checkTransport(raw string, allowInsecure bool) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return core.Invalid("webhook url %q is not a valid url", raw)
	}
	if u.Scheme != "http" {
		return nil
	}
	if isLoopbackHost(u.Hostname()) || allowInsecure {
		return nil
	}
	return core.Invalid("webhook url %q must use https; deliveries carry task content", raw)
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// webhookPayload describes an endpoint change to subscribers. It carries no
// signing secret, since an event is delivered to every matching endpoint.
func webhookPayload(e core.WebhookEndpoint) map[string]any {
	return map[string]any{
		"url":         e.URL,
		"active":      e.Active,
		"event_types": e.EventTypes,
	}
}

// PutWebhook registers a delivery endpoint or updates one by identifier. A
// registration that supplies no secret gets a generated one, returned in the
// result exactly once so it can be configured on the receiving end; no later
// read ever discloses it.
func (l *Local) PutWebhook(ctx context.Context, in core.WebhookInput) (*core.WebhookEndpoint, error) {
	actor, err := l.authorize(ctx, authz.ActionWebhookAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := in.Validate(); err != nil {
		return nil, err
	}

	var (
		out       core.WebhookEndpoint
		generated bool
	)
	err = l.write(ctx, actor, func(m *mutation) error {
		var before any
		secret := strings.TrimSpace(in.Secret)
		if in.ID != "" {
			existing, err := m.tx.GetWebhook(ctx, in.ID)
			if err != nil {
				return err
			}
			before = webhook.Redact(*existing)
			if secret == "" {
				secret = existing.Secret
			}
		}
		if secret == "" {
			secret, err = newWebhookSecret()
			if err != nil {
				return err
			}
			generated = true
		}

		e := core.WebhookEndpoint{
			ID:         in.ID,
			URL:        strings.TrimSpace(in.URL),
			Secret:     secret,
			EventTypes: in.EventTypes,
			Active:     in.Active,
		}
		if err := webhook.ValidateEndpoint(e); err != nil {
			return err
		}
		if err := checkTransport(e.URL, l.allowInsecureWebhooks); err != nil {
			return err
		}
		if err := m.tx.PutWebhook(ctx, &e); err != nil {
			return err
		}
		out = e
		return m.Record(auditWebhookPut, eventWebhookPut, webhookSubject, e.ID, "",
			before, webhook.Redact(e), webhookPayload(e))
	})
	if err != nil {
		return nil, err
	}

	result := webhook.Redact(out)
	if generated {
		result.Secret = out.Secret
	}
	return &result, nil
}

// ListWebhooks returns this tenant's delivery endpoints without their secrets.
func (l *Local) ListWebhooks(ctx context.Context) ([]core.WebhookEndpoint, error) {
	actor, err := l.authorize(ctx, authz.ActionWebhookAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	out := []core.WebhookEndpoint{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		endpoints, err := tx.ListWebhooks(ctx)
		if err != nil {
			return err
		}
		out = webhook.RedactAll(endpoints)
		return nil
	}); err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteWebhook removes a delivery endpoint and its queue.
func (l *Local) DeleteWebhook(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionWebhookAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	if strings.TrimSpace(id) == "" {
		return core.Invalid("webhook identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		existing, err := m.tx.GetWebhook(ctx, id)
		if err != nil {
			return err
		}
		if err := m.tx.DeleteWebhook(ctx, id); err != nil {
			return err
		}
		return m.Record(auditWebhookDelete, eventWebhookDeleted, webhookSubject, id, "",
			webhook.Redact(*existing), nil, webhookPayload(*existing))
	})
}

// deliveryPage normalizes a delivery filter's page, defaulting to the newest
// attempts first.
func deliveryPage(f core.DeliveryFilter) (core.DeliveryFilter, error) {
	if f.Page.Sort == "" {
		f.Page.Sort = core.SortCreatedAt
	}
	if f.Page.Direction == "" {
		f.Page.Direction = core.Descending
	}
	if !deliverySortFields[f.Page.Sort] {
		return f, core.Invalid("cannot sort deliveries by %q", f.Page.Sort)
	}
	page, err := f.Page.Normalize()
	if err != nil {
		return f, err
	}
	if err := mustCursorMatch(page); err != nil {
		return f, err
	}
	f.Page = page
	return f, nil
}

// nextDeliveryCursor returns the cursor for the page after these deliveries.
func nextDeliveryCursor(page core.Page, items []core.WebhookDelivery) string {
	if len(items) == 0 || len(items) < page.Limit {
		return ""
	}
	last := items[len(items)-1]
	value := sqlb.TimeText(last.CreatedAt)
	if page.Sort == "next_attempt_at" {
		value = sqlb.TimeText(last.NextAttemptAt)
	}
	return core.Cursor{
		SortValue: value,
		ID:        last.ID,
		Sort:      page.Sort,
		Direction: page.Direction,
	}.Encode()
}

// ListDeliveries returns one keyset-paginated page of the delivery log and the
// cursor for the next.
func (l *Local) ListDeliveries(ctx context.Context, f core.DeliveryFilter) ([]core.WebhookDelivery, string, error) {
	actor, err := l.authorize(ctx, authz.ActionWebhookAdmin, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	filter, err := deliveryPage(f)
	if err != nil {
		return nil, "", err
	}
	out := []core.WebhookDelivery{}
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		rows, err := tx.ListDeliveries(ctx, filter)
		if err != nil {
			return err
		}
		out = rows
		return nil
	}); err != nil {
		return nil, "", err
	}
	return out, nextDeliveryCursor(filter.Page, out), nil
}

// RedeliverWebhook queues an existing delivery for another attempt against the
// same endpoint and the same event. The attempt itself happens on the next
// drain, after this transaction commits, never inside it.
func (l *Local) RedeliverWebhook(ctx context.Context, deliveryID string) error {
	actor, err := l.authorize(ctx, authz.ActionWebhookAdmin, authz.Resource{})
	if err != nil {
		return err
	}
	if strings.TrimSpace(deliveryID) == "" {
		return core.Invalid("delivery identifier is required")
	}
	return l.write(ctx, actor, func(m *mutation) error {
		existing, err := m.tx.GetDelivery(ctx, deliveryID)
		if err != nil {
			return err
		}
		if err := m.tx.MarkFailed(ctx, existing.ID, 0, "", m.now, false); err != nil {
			return err
		}
		requeued, err := m.tx.GetDelivery(ctx, existing.ID)
		if err != nil {
			return err
		}
		return m.Record(auditWebhookRedeliver, eventWebhookRedelivered, deliverySubject, existing.ID, "",
			existing, requeued, map[string]any{
				"endpoint_id": requeued.EndpointID,
				"event_seq":   requeued.EventSeq,
			})
	})
}
