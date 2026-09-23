// SPDX-License-Identifier: AGPL-3.0-or-later

package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var webhookColumns = []string{
	"id", "tenant_id", "url", "secret", "event_types", "active", "created_at",
}

func scanWebhook(s scanner) (core.WebhookEndpoint, error) {
	var (
		e       core.WebhookEndpoint
		types   string
		created sql.NullTime
	)
	if err := s.Scan(&e.ID, &e.TenantID, &e.URL, &e.Secret, &types, &e.Active, &created); err != nil {
		return core.WebhookEndpoint{}, mapErr(err, "scanning webhook endpoint")
	}
	if err := sqlb.ParseJSON(types, &e.EventTypes); err != nil {
		return core.WebhookEndpoint{}, core.Internal("decoding event types of webhook %q", e.ID).Wrap(err)
	}
	e.CreatedAt = scanTime(created)
	return e, nil
}

// PutWebhook creates or replaces a delivery endpoint.
func (t *tx) PutWebhook(ctx context.Context, e *core.WebhookEndpoint) error {
	e.TenantID = t.scope.TenantID
	if len(e.EventTypes) == 0 {
		e.EventTypes = []string{"*"}
	}
	types, err := sqlb.JSONText(e.EventTypes, `["*"]`)
	if err != nil {
		return core.Internal("encoding event types of webhook %q", e.URL).Wrap(err)
	}
	if e.ID != "" {
		upd := t.builder("webhook_endpoints").
			Where("id = ?", e.ID).
			Set("url", e.URL).
			Set("secret", e.Secret).
			Set("event_types", types).
			Set("active", e.Active)
		n, err := t.execUpdate(ctx, upd, "updating webhook %q", e.ID)
		if err != nil {
			return err
		}
		if n > 0 {
			return nil
		}
	}
	if e.ID == "" {
		e.ID = id.New()
	}
	if e.CreatedAt.IsZero() {
		e.CreatedAt = t.store.clock.Now()
	}
	ins := t.insert("webhook_endpoints").
		Set("id", e.ID).
		Set("url", e.URL).
		Set("secret", e.Secret).
		Set("event_types", types).
		Set("active", e.Active).
		Set("created_at", timeArg(e.CreatedAt))
	_, err = t.execInsert(ctx, ins, "creating webhook %q", e.URL)
	return err
}

// GetWebhook returns one delivery endpoint.
func (t *tx) GetWebhook(ctx context.Context, endpointID string) (*core.WebhookEndpoint, error) {
	b := t.builder("webhook_endpoints").Select(webhookColumns...).Where("id = ?", endpointID).Limit(1)
	q, args := b.SelectQuery()
	e, err := scanWebhook(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("webhook %q", endpointID)
		}
		return nil, err
	}
	return &e, nil
}

// ListWebhooks returns this tenant's delivery endpoints.
func (t *tx) ListWebhooks(ctx context.Context) ([]core.WebhookEndpoint, error) {
	b := t.builder("webhook_endpoints").
		Select(webhookColumns...).
		OrderBy("created_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "listing webhooks")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.WebhookEndpoint{}
	for rows.Next() {
		v, err := scanWebhook(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing webhooks")
}

// DeleteWebhook removes a delivery endpoint and its queue.
func (t *tx) DeleteWebhook(ctx context.Context, endpointID string) error {
	b := t.builder("webhook_endpoints").Where("id = ?", endpointID)
	n, err := t.execDelete(ctx, b, "deleting webhook %q", endpointID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("webhook %q", endpointID)
	}
	return nil
}

var deliveryColumns = []string{
	"id", "tenant_id", "endpoint_id", "event_seq", "attempts", "next_attempt_at",
	"status", "last_error", "last_status_code", "created_at",
}

func scanDelivery(s scanner) (core.WebhookDelivery, error) {
	var (
		d       core.WebhookDelivery
		next    sql.NullTime
		created sql.NullTime
	)
	if err := s.Scan(&d.ID, &d.TenantID, &d.EndpointID, &d.EventSeq, &d.Attempts, &next,
		&d.Status, &d.LastError, &d.LastStatusCode, &created); err != nil {
		return core.WebhookDelivery{}, mapErr(err, "scanning webhook delivery")
	}
	d.NextAttemptAt = scanTime(next)
	d.CreatedAt = scanTime(created)
	return d, nil
}

// EnqueueDelivery queues one event for one endpoint.
func (t *tx) EnqueueDelivery(ctx context.Context, d *core.WebhookDelivery) error {
	if d.ID == "" {
		d.ID = id.New()
	}
	d.TenantID = t.scope.TenantID
	if d.Status == "" {
		d.Status = core.DeliveryPending
	}
	now := t.store.clock.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = now
	}
	ins := t.insert("webhook_deliveries").
		Set("id", d.ID).
		Set("endpoint_id", d.EndpointID).
		Set("event_seq", d.EventSeq).
		Set("attempts", d.Attempts).
		Set("next_attempt_at", timeArg(d.NextAttemptAt)).
		Set("status", string(d.Status)).
		Set("last_error", d.LastError).
		Set("last_status_code", d.LastStatusCode).
		Set("created_at", timeArg(d.CreatedAt))
	_, err := t.execInsert(ctx, ins, "enqueueing delivery for endpoint %q", d.EndpointID)
	return err
}

// ClaimDeliveries locks pending deliveries so two processes cannot both send them.
func (t *tx) ClaimDeliveries(ctx context.Context, owner string, now, until time.Time, limit int) ([]core.WebhookDelivery, error) {
	if limit <= 0 {
		limit = core.DefaultPageLimit
	}
	nowArg := timeArg(now)
	pick := t.builder("webhook_deliveries").
		Select("id").
		Where("status = ?", string(core.DeliveryPending)).
		Where("next_attempt_at <= ?", nowArg).
		Where("(locked_until IS NULL OR locked_until <= ?)", nowArg).
		OrderBy("next_attempt_at", core.Ascending).
		OrderBy("id", core.Ascending).
		Limit(limit)
	ids, err := t.pickLocked(ctx, pick, "choosing webhook deliveries")
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return []core.WebhookDelivery{}, nil
	}

	upd := t.builder("webhook_deliveries")
	upd.WhereIn("id", ids)
	upd.Set("locked_by", owner).Set("locked_until", timeArg(until))
	if _, err := t.execUpdate(ctx, upd, "claiming webhook deliveries"); err != nil {
		return nil, err
	}

	b := t.builder("webhook_deliveries").
		Select(deliveryColumns...).
		Where("locked_by = ?", owner).
		Where("locked_until = ?", timeArg(until)).
		OrderBy("next_attempt_at", core.Ascending).
		OrderBy("id", core.Ascending)
	rows, err := t.query(ctx, b, "reading claimed deliveries")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.WebhookDelivery{}
	for rows.Next() {
		v, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "reading claimed deliveries")
}

// MarkDelivered records a successful delivery.
func (t *tx) MarkDelivered(ctx context.Context, deliveryID string, statusCode int, at time.Time) error {
	b := t.builder("webhook_deliveries").
		Where("id = ?", deliveryID).
		Set("status", string(core.DeliveryDelivered)).
		Set("last_status_code", statusCode).
		Set("last_error", "").
		Set("locked_by", nil).
		Set("locked_until", nil).
		Set("next_attempt_at", timeArg(at)).
		SetExpr("attempts", "attempts + 1")
	n, err := t.execUpdate(ctx, b, "marking delivery %q delivered", deliveryID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("webhook delivery %q", deliveryID)
	}
	return nil
}

// MarkFailed records a failed attempt, giving up when terminal is set.
func (t *tx) MarkFailed(ctx context.Context, deliveryID string, statusCode int, errMsg string, nextAttempt time.Time, terminal bool) error {
	status := core.DeliveryPending
	if terminal {
		status = core.DeliveryFailed
	}
	b := t.builder("webhook_deliveries").
		Where("id = ?", deliveryID).
		Set("status", string(status)).
		Set("last_status_code", statusCode).
		Set("last_error", errMsg).
		Set("locked_by", nil).
		Set("locked_until", nil).
		Set("next_attempt_at", timeArg(nextAttempt)).
		SetExpr("attempts", "attempts + 1")
	n, err := t.execUpdate(ctx, b, "marking delivery %q failed", deliveryID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("webhook delivery %q", deliveryID)
	}
	return nil
}

// GetDelivery returns one delivery record.
func (t *tx) GetDelivery(ctx context.Context, deliveryID string) (*core.WebhookDelivery, error) {
	b := t.builder("webhook_deliveries").Select(deliveryColumns...).Where("id = ?", deliveryID).Limit(1)
	q, args := b.SelectQuery()
	d, err := scanDelivery(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("webhook delivery %q", deliveryID)
		}
		return nil, err
	}
	return &d, nil
}

// ListDeliveries returns deliveries matching the filter, keyset paginated.
func (t *tx) ListDeliveries(ctx context.Context, f core.DeliveryFilter) ([]core.WebhookDelivery, error) {
	spec, err := resolvePage(f.Page, "created_at", map[string]string{
		"created_at":      "created_at",
		"next_attempt_at": "next_attempt_at",
	})
	if err != nil {
		return nil, err
	}
	b := t.builder("webhook_deliveries").Select(deliveryColumns...)
	if f.EndpointID != "" {
		b.Where("endpoint_id = ?", f.EndpointID)
	}
	if len(f.Statuses) > 0 {
		statuses := make([]string, len(f.Statuses))
		for i, s := range f.Statuses {
			statuses[i] = string(s)
		}
		b.WhereIn("status", statuses)
	}
	rows, err := t.query(ctx, spec.apply(b, "id"), "listing webhook deliveries")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.WebhookDelivery{}
	for rows.Next() {
		v, err := scanDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing webhook deliveries")
}
