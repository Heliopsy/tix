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

// maxChannelsListed bounds one channel listing; a person holds a handful.
const maxChannelsListed = 1000

var channelColumns = []string{
	"inbox_channels.id", "inbox_channels.actor_id", "inbox_channels.name", "inbox_channels.url",
	"inbox_channels.secret", "inbox_channels.format", "inbox_channels.kinds", "inbox_channels.active",
	"inbox_channels.created_at",
}

var channelDeliveryColumns = []string{
	"inbox_deliveries.id", "inbox_deliveries.channel_id", "inbox_deliveries.item_id",
	"inbox_deliveries.attempts", "inbox_deliveries.next_attempt_at", "inbox_deliveries.status",
	"inbox_deliveries.last_error", "inbox_deliveries.last_status_code", "inbox_deliveries.created_at",
}

func scanChannel(s scanner) (core.InboxChannel, error) {
	var (
		c       core.InboxChannel
		format  string
		kinds   string
		active  int64
		created sql.NullTime
	)
	if err := s.Scan(&c.ID, &c.ActorID, &c.Name, &c.URL, &c.Secret, &format, &kinds, &active, &created); err != nil {
		return core.InboxChannel{}, mapErr(err, "scanning inbox channel")
	}
	c.Format, c.Active = core.ChannelFormat(format), sqlb.Bool(active)
	if err := sqlb.ParseJSON(kinds, &c.Kinds); err != nil {
		return core.InboxChannel{}, core.Internal("reading the kinds of channel %q", c.ID).Wrap(err)
	}
	c.CreatedAt = scanTime(created)
	return c, nil
}

func scanChannelDelivery(s scanner) (core.ChannelDelivery, error) {
	var (
		d             core.ChannelDelivery
		item          sql.NullString
		status        string
		next, created sql.NullTime
	)
	if err := s.Scan(&d.ID, &d.ChannelID, &item, &d.Attempts, &next, &status, &d.LastError,
		&d.LastStatusCode, &created); err != nil {
		return core.ChannelDelivery{}, mapErr(err, "scanning channel delivery")
	}
	d.ItemID, d.Status = sqlb.Text(item), core.DeliveryStatus(status)
	d.NextAttemptAt, d.CreatedAt = scanTime(next), scanTime(created)
	return d, nil
}

// CreateInboxChannel inserts a channel, filling its identifier when empty.
func (t *tx) CreateInboxChannel(ctx context.Context, c *core.InboxChannel) error {
	if c.ID == "" {
		c.ID = id.New()
	}
	if c.CreatedAt.IsZero() {
		c.CreatedAt = t.store.clock.Now()
	}
	kinds, err := sqlb.JSONText(c.Kinds, "[]")
	if err != nil {
		return err
	}
	ins := t.insert("inbox_channels").
		Set("id", c.ID).
		Set("actor_id", c.ActorID).
		Set("name", c.Name).
		Set("url", c.URL).
		Set("secret", c.Secret).
		Set("format", string(c.Format)).
		Set("kinds", kinds).
		Set("active", sqlb.BoolInt(c.Active)).
		Set("created_at", timeArg(c.CreatedAt))
	_, err = t.execInsert(ctx, ins, "creating inbox channel %q", c.Name)
	return err
}

// UpdateInboxChannel rewrites a channel's name, url, format, kinds and state.
func (t *tx) UpdateInboxChannel(ctx context.Context, c *core.InboxChannel) error {
	kinds, err := sqlb.JSONText(c.Kinds, "[]")
	if err != nil {
		return err
	}
	b := t.builder("inbox_channels").
		Where("inbox_channels.id = ?", c.ID).
		Set("name", c.Name).
		Set("url", c.URL).
		Set("format", string(c.Format)).
		Set("kinds", kinds).
		Set("active", sqlb.BoolInt(c.Active))
	n, err := t.execUpdate(ctx, b, "updating inbox channel %q", c.ID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("inbox channel %q", c.ID)
	}
	return nil
}

// GetInboxChannel returns one channel of this tenant.
func (t *tx) GetInboxChannel(ctx context.Context, channelID string) (*core.InboxChannel, error) {
	b := t.builder("inbox_channels").Select(channelColumns...).Where("inbox_channels.id = ?", channelID).Limit(1)
	q, args := b.SelectQuery()
	c, err := scanChannel(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("inbox channel %q", channelID)
		}
		return nil, err
	}
	return &c, nil
}

// ListInboxChannels returns the channels of the given actors, by name.
func (t *tx) ListInboxChannels(ctx context.Context, actorIDs []string) ([]core.InboxChannel, error) {
	out := []core.InboxChannel{}
	if len(actorIDs) == 0 {
		return out, nil
	}
	b := t.builder("inbox_channels").Select(channelColumns...).
		OrderBy("inbox_channels.name", core.Ascending).
		OrderBy("inbox_channels.id", core.Ascending).
		Limit(maxChannelsListed)
	b.WhereIn("inbox_channels.actor_id", actorIDs)
	rows, err := t.query(ctx, b, "listing inbox channels")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, mapRowsErr(rows, "listing inbox channels")
}

// DeleteInboxChannel removes a channel; its deliveries cascade.
func (t *tx) DeleteInboxChannel(ctx context.Context, channelID string) error {
	n, err := t.execDelete(ctx, t.builder("inbox_channels").Where("inbox_channels.id = ?", channelID),
		"removing inbox channel %q", channelID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("inbox channel %q", channelID)
	}
	return nil
}

// EnqueueChannelDelivery queues one delivery for a channel.
func (t *tx) EnqueueChannelDelivery(ctx context.Context, d *core.ChannelDelivery) error {
	if d.ID == "" {
		d.ID = id.New()
	}
	now := t.store.clock.Now()
	if d.CreatedAt.IsZero() {
		d.CreatedAt = now
	}
	if d.NextAttemptAt.IsZero() {
		d.NextAttemptAt = now
	}
	if d.Status == "" {
		d.Status = core.DeliveryPending
	}
	ins := t.insert("inbox_deliveries").
		Set("id", d.ID).
		Set("channel_id", d.ChannelID).
		Set("item_id", sqlb.NullText(d.ItemID)).
		Set("attempts", d.Attempts).
		Set("next_attempt_at", timeArg(d.NextAttemptAt)).
		Set("status", string(d.Status)).
		Set("last_error", d.LastError).
		Set("last_status_code", d.LastStatusCode).
		Set("created_at", timeArg(d.CreatedAt))
	_, err := t.execInsert(ctx, ins, "queueing a delivery for channel %q", d.ChannelID)
	return err
}

// ClaimChannelDeliveries locks due deliveries so two processes cannot both send them.
func (t *tx) ClaimChannelDeliveries(ctx context.Context, owner string, now, until time.Time, limit int) ([]core.ChannelDelivery, error) {
	if limit <= 0 {
		limit = core.DefaultPageLimit
	}
	nowArg := timeArg(now)
	pick := t.builder("inbox_deliveries").
		Select("inbox_deliveries.id").
		Where("inbox_deliveries.status = ?", string(core.DeliveryPending)).
		Where("inbox_deliveries.next_attempt_at <= ?", nowArg).
		Where("(inbox_deliveries.locked_until IS NULL OR inbox_deliveries.locked_until <= ?)", nowArg).
		OrderBy("inbox_deliveries.next_attempt_at", core.Ascending).
		OrderBy("inbox_deliveries.id", core.Ascending).
		Limit(limit)
	ids, err := t.pickLocked(ctx, pick, "choosing channel deliveries")
	if err != nil || len(ids) == 0 {
		return []core.ChannelDelivery{}, err
	}
	upd := t.builder("inbox_deliveries")
	upd.WhereIn("inbox_deliveries.id", ids)
	upd.Set("locked_by", owner).Set("locked_until", timeArg(until))
	if _, err := t.execUpdate(ctx, upd, "claiming channel deliveries"); err != nil {
		return nil, err
	}
	return t.claimedChannelDeliveries(ctx, owner, timeArg(until))
}

func (t *tx) claimedChannelDeliveries(ctx context.Context, owner string, until any) ([]core.ChannelDelivery, error) {
	b := t.builder("inbox_deliveries").
		Select(channelDeliveryColumns...).
		Where("inbox_deliveries.locked_by = ?", owner).
		Where("inbox_deliveries.locked_until = ?", until).
		OrderBy("inbox_deliveries.next_attempt_at", core.Ascending).
		OrderBy("inbox_deliveries.id", core.Ascending)
	return t.channelDeliveries(ctx, b, "reading claimed channel deliveries")
}

func (t *tx) channelDeliveries(ctx context.Context, b *sqlb.Builder, what string) ([]core.ChannelDelivery, error) {
	rows, err := t.query(ctx, b, "%s", what)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.ChannelDelivery{}
	for rows.Next() {
		d, err := scanChannelDelivery(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, d)
	}
	return out, mapRowsErr(rows, what)
}

// MarkChannelDelivered records a successful delivery.
func (t *tx) MarkChannelDelivered(ctx context.Context, deliveryID string, statusCode int, at time.Time) error {
	return t.markChannel(ctx, deliveryID, core.DeliveryDelivered, statusCode, "", timeArg(at))
}

// MarkChannelFailed records a failed attempt, giving up when terminal is set.
func (t *tx) MarkChannelFailed(ctx context.Context, deliveryID string, statusCode int, errMsg string, nextAttempt time.Time, terminal bool) error {
	status := core.DeliveryPending
	if terminal {
		status = core.DeliveryFailed
	}
	return t.markChannel(ctx, deliveryID, status, statusCode, errMsg, timeArg(nextAttempt))
}

func (t *tx) markChannel(ctx context.Context, deliveryID string, status core.DeliveryStatus, code int, errMsg string, next any) error {
	b := t.builder("inbox_deliveries").
		Where("inbox_deliveries.id = ?", deliveryID).
		Set("status", string(status)).
		Set("last_status_code", code).
		Set("last_error", errMsg).
		Set("locked_by", nil).
		Set("locked_until", nil).
		Set("next_attempt_at", next).
		SetExpr("attempts", "attempts + 1")
	n, err := t.execUpdate(ctx, b, "recording channel delivery %q", deliveryID)
	if err != nil {
		return err
	}
	if n == 0 {
		return core.NotFound("channel delivery %q", deliveryID)
	}
	return nil
}

// GetChannelDelivery returns one delivery.
func (t *tx) GetChannelDelivery(ctx context.Context, deliveryID string) (*core.ChannelDelivery, error) {
	b := t.builder("inbox_deliveries").Select(channelDeliveryColumns...).
		Where("inbox_deliveries.id = ?", deliveryID).Limit(1)
	q, args := b.SelectQuery()
	d, err := scanChannelDelivery(t.ex.QueryRowContext(ctx, q, args...))
	if err != nil {
		if core.IsKind(err, core.KindNotFound) {
			return nil, core.NotFound("channel delivery %q", deliveryID)
		}
		return nil, err
	}
	return &d, nil
}

// ListChannelDeliveries returns a page of one channel's deliveries, newest first.
func (t *tx) ListChannelDeliveries(ctx context.Context, f core.ChannelDeliveryFilter) ([]core.ChannelDelivery, error) {
	spec, err := resolvePage(f.Page, core.SortCreatedAt, map[string]string{
		core.SortCreatedAt: "inbox_deliveries.created_at",
	})
	if err != nil {
		return nil, err
	}
	b := t.builder("inbox_deliveries").Select(channelDeliveryColumns...).
		Where("inbox_deliveries.channel_id = ?", f.ChannelID)
	return t.channelDeliveries(ctx, spec.apply(b, "inbox_deliveries.id"), "listing channel deliveries")
}
