package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

var eventColumns = []string{
	"seq", "id", "tenant_id", "type", "project_id", "subject_type", "subject_id",
	"actor_id", "payload", "occurred_at",
}

func scanEvent(s scanner) (core.Event, error) {
	var (
		e        core.Event
		project  sql.NullString
		actor    sql.NullString
		payload  string
		occurred sql.NullString
	)
	if err := s.Scan(&e.Seq, &e.ID, &e.TenantID, &e.Type, &project, &e.SubjectType,
		&e.SubjectID, &actor, &payload, &occurred); err != nil {
		return core.Event{}, mapErr(err, "scanning event")
	}
	e.ProjectID = sqlb.Text(project)
	e.ActorID = sqlb.Text(actor)
	if err := sqlb.ParseJSON(payload, &e.Payload); err != nil {
		return core.Event{}, core.Internal("decoding payload of event %q", e.ID).Wrap(err)
	}
	var err error
	if e.OccurredAt, err = sqlb.ScanTime(occurred); err != nil {
		return core.Event{}, err
	}
	return e, nil
}

// AppendEvent writes to the outbox in the caller's transaction.
func (t *tx) AppendEvent(ctx context.Context, e *core.Event) error {
	if e.ID == "" {
		e.ID = id.New()
	}
	e.TenantID = t.scope.TenantID
	if e.OccurredAt.IsZero() {
		e.OccurredAt = t.store.clock.Now()
	}
	payload, err := sqlb.JSONText(e.Payload, "{}")
	if err != nil {
		return core.Internal("encoding payload of event %q", e.Type).Wrap(err)
	}
	ins := t.insert("events").
		Set("id", e.ID).
		Set("type", string(e.Type)).
		Set("project_id", sqlb.NullText(e.ProjectID)).
		Set("subject_type", e.SubjectType).
		Set("subject_id", e.SubjectID).
		Set("actor_id", sqlb.NullText(e.ActorID)).
		Set("payload", payload).
		Set("occurred_at", sqlb.TimeText(e.OccurredAt))
	res, err := t.execInsert(ctx, ins, "appending event %q", string(e.Type))
	if err != nil {
		return err
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return mapErr(err, "reading the sequence of event %q", e.ID)
	}
	e.Seq = seq
	return nil
}

// ReadEvents returns this tenant's events after a sequence number.
func (t *tx) ReadEvents(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error) {
	if limit <= 0 {
		limit = core.DefaultPageLimit
	}
	b := t.builder("events").
		Select(eventColumns...).
		Where("seq > ?", sinceSeq).
		OrderBy("seq", core.Ascending).
		Limit(limit)
	rows, err := t.query(ctx, b, "reading events")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.Event{}
	for rows.Next() {
		v, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "reading events")
}

// LatestEventSeq returns the highest sequence number this tenant has written.
func (t *tx) LatestEventSeq(ctx context.Context) (int64, error) {
	b := t.builder("events").Select("COALESCE(MAX(seq), 0)")
	q, args := b.SelectQuery()
	var seq int64
	if err := t.ex.QueryRowContext(ctx, q, args...).Scan(&seq); err != nil {
		return 0, mapErr(err, "reading the latest event sequence")
	}
	return seq, nil
}

var auditColumns = []string{
	"seq", "tenant_id", "actor_id", "action", "subject_type", "subject_id",
	"before_state", "after_state", "source", "occurred_at",
}

func scanAudit(s scanner) (core.AuditEntry, error) {
	var (
		e        core.AuditEntry
		actor    sql.NullString
		before   sql.NullString
		after    sql.NullString
		occurred sql.NullString
	)
	if err := s.Scan(&e.Seq, &e.TenantID, &actor, &e.Action, &e.SubjectType, &e.SubjectID,
		&before, &after, &e.Source, &occurred); err != nil {
		return core.AuditEntry{}, mapErr(err, "scanning audit entry")
	}
	e.ActorID = sqlb.Text(actor)
	if s := sqlb.Text(before); s != "" {
		e.Before = json.RawMessage(s)
	}
	if s := sqlb.Text(after); s != "" {
		e.After = json.RawMessage(s)
	}
	var err error
	if e.OccurredAt, err = sqlb.ScanTime(occurred); err != nil {
		return core.AuditEntry{}, err
	}
	return e, nil
}

// AppendAudit writes one audit record in the caller's transaction.
func (t *tx) AppendAudit(ctx context.Context, e *core.AuditEntry) error {
	e.TenantID = t.scope.TenantID
	if e.OccurredAt.IsZero() {
		e.OccurredAt = t.store.clock.Now()
	}
	if e.Source == "" {
		e.Source = core.SourceSystem
	}
	ins := t.insert("audit_entries").
		Set("actor_id", sqlb.NullText(e.ActorID)).
		Set("action", e.Action).
		Set("subject_type", e.SubjectType).
		Set("subject_id", e.SubjectID).
		Set("before_state", sqlb.NullText(string(e.Before))).
		Set("after_state", sqlb.NullText(string(e.After))).
		Set("source", string(e.Source)).
		Set("occurred_at", sqlb.TimeText(e.OccurredAt))
	res, err := t.execInsert(ctx, ins, "appending audit entry %q", e.Action)
	if err != nil {
		return err
	}
	seq, err := res.LastInsertId()
	if err != nil {
		return mapErr(err, "reading the sequence of an audit entry")
	}
	e.Seq = seq
	return nil
}

// ListAudit returns audit entries matching the filter, keyset paginated.
func (t *tx) ListAudit(ctx context.Context, f core.AuditFilter) ([]core.AuditEntry, error) {
	spec, err := resolvePage(f.Page, "seq", map[string]string{
		"seq":         "seq",
		"occurred_at": "occurred_at",
	})
	if err != nil {
		return nil, err
	}
	b := t.builder("audit_entries").Select(auditColumns...)
	if f.SubjectType != "" {
		b.Where("subject_type = ?", f.SubjectType)
	}
	if f.SubjectID != "" {
		b.Where("subject_id = ?", f.SubjectID)
	}
	if len(f.ActorIDs) > 0 {
		b.WhereIn("actor_id", f.ActorIDs)
	}
	if len(f.Actions) > 0 {
		b.WhereIn("action", f.Actions)
	}
	if len(f.Sources) > 0 {
		sources := make([]string, len(f.Sources))
		for i, s := range f.Sources {
			sources[i] = string(s)
		}
		b.WhereIn("source", sources)
	}
	if f.Since != nil {
		b.Where("occurred_at >= ?", sqlb.TimeText(*f.Since))
	}
	if f.Until != nil {
		b.Where("occurred_at <= ?", sqlb.TimeText(*f.Until))
	}
	rows, err := t.query(ctx, spec.apply(b, "seq"), "listing audit entries")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := []core.AuditEntry{}
	for rows.Next() {
		v, err := scanAudit(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapRowsErr(rows, "listing audit entries")
}

// GetRetention returns this tenant's retention policy, or the shipped default.
func (t *tx) GetRetention(ctx context.Context) (*core.RetentionPolicy, error) {
	b := t.builder("retention_policies").
		Select("tenant_id", "events", "audit_entries", "webhook_deliveries").
		Limit(1)
	q, args := b.SelectQuery()

	var tenantID, events, audit, deliveries string
	err := t.ex.QueryRowContext(ctx, q, args...).Scan(&tenantID, &events, &audit, &deliveries)
	if err != nil {
		if mapped := mapErr(err, "reading retention policy"); core.IsKind(mapped, core.KindNotFound) {
			p := core.DefaultRetention(t.scope.TenantID)
			return &p, nil
		}
		return nil, mapErr(err, "reading retention policy")
	}

	p := core.RetentionPolicy{TenantID: tenantID}
	for _, f := range []struct {
		raw string
		dst *core.Duration
	}{{events, &p.Events}, {audit, &p.AuditEntries}, {deliveries, &p.WebhookDeliveries}} {
		d, err := time.ParseDuration(f.raw)
		if err != nil {
			return nil, core.Internal("parsing stored retention %q", f.raw).Wrap(err)
		}
		*f.dst = core.Duration(d)
	}
	return &p, nil
}

// PutRetention stores this tenant's retention policy.
func (t *tx) PutRetention(ctx context.Context, p *core.RetentionPolicy) error {
	p.TenantID = t.scope.TenantID
	upd := t.builder("retention_policies").
		Set("events", p.Events.String()).
		Set("audit_entries", p.AuditEntries.String()).
		Set("webhook_deliveries", p.WebhookDeliveries.String())
	n, err := t.execUpdate(ctx, upd, "updating retention policy")
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	ins := t.insert("retention_policies").
		Set("events", p.Events.String()).
		Set("audit_entries", p.AuditEntries.String()).
		Set("webhook_deliveries", p.WebhookDeliveries.String())
	_, err = t.execInsert(ctx, ins, "creating retention policy")
	return err
}

// PruneEvents deletes old events, never above the floor a subscriber still needs.
func (t *tx) PruneEvents(ctx context.Context, before time.Time, floorSeq int64, limit int) (int64, error) {
	if limit <= 0 {
		limit = core.MaxPageLimit
	}
	inner := t.builder("events").
		Select("seq").
		Where("occurred_at < ?", sqlb.TimeText(before)).
		Where("seq <= ?", floorSeq).
		OrderBy("seq", core.Ascending).
		Limit(limit)
	sub, subArgs := inner.SelectQuery()
	b := t.builder("events").Where("seq IN ("+sub+")", subArgs...)
	return t.execDelete(ctx, b, "pruning events")
}

// PruneAudit deletes old audit entries.
func (t *tx) PruneAudit(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = core.MaxPageLimit
	}
	inner := t.builder("audit_entries").
		Select("seq").
		Where("occurred_at < ?", sqlb.TimeText(before)).
		OrderBy("seq", core.Ascending).
		Limit(limit)
	sub, subArgs := inner.SelectQuery()
	b := t.builder("audit_entries").Where("seq IN ("+sub+")", subArgs...)
	return t.execDelete(ctx, b, "pruning audit entries")
}

// PruneDeliveries deletes old webhook delivery records.
func (t *tx) PruneDeliveries(ctx context.Context, before time.Time, limit int) (int64, error) {
	if limit <= 0 {
		limit = core.MaxPageLimit
	}
	inner := t.builder("webhook_deliveries").
		Select("id").
		Where("created_at < ?", sqlb.TimeText(before)).
		Where("status <> ?", string(core.DeliveryPending)).
		OrderBy("created_at", core.Ascending).
		Limit(limit)
	sub, subArgs := inner.SelectQuery()
	b := t.builder("webhook_deliveries").Where("id IN ("+sub+")", subArgs...)
	return t.execDelete(ctx, b, "pruning webhook deliveries")
}
