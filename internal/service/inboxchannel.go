// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"net/url"
	"slices"
	"strings"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Audit actions of the personal channels.
const (
	auditChannelCreate    = "inbox.channel.create"
	auditChannelUpdate    = "inbox.channel.update"
	auditChannelDelete    = "inbox.channel.delete"
	auditChannelTest      = "inbox.channel.test"
	auditChannelRedeliver = "inbox.channel.redeliver"
)

// queueChannels queues a delivery to every channel of the item's recipient
// that takes its kind, in the transaction that wrote the item. Like a tenant
// webhook it queues nothing when this process delivers nothing.
func (m *mutation) queueChannels(ctx context.Context, items []*core.InboxItem, recipients []string) error {
	if m.local.hooks == HookOff || len(items) == 0 {
		return nil
	}
	channels, err := m.tx.ListInboxChannels(ctx, recipients)
	if err != nil || len(channels) == 0 {
		return err
	}
	for _, it := range items {
		for _, c := range channels {
			if c.ActorID != it.RecipientID || !c.Receives(it.Kind) {
				continue
			}
			d := &core.ChannelDelivery{ChannelID: c.ID, ItemID: it.ID, NextAttemptAt: m.now}
			if err := m.tx.EnqueueChannelDelivery(ctx, d); err != nil {
				return err
			}
			m.queued++
		}
	}
	return nil
}

// ListInboxChannels returns the caller's channels, without their secrets.
func (l *Local) ListInboxChannels(ctx context.Context) ([]core.InboxChannel, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out []core.InboxChannel
	err = l.read(ctx, actor, func(tx store.Tx) error {
		out, err = tx.ListInboxChannels(ctx, []string{actor.ID})
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// CreateInboxChannel adds a channel for the caller and returns its signing
// secret, which no later read returns.
func (l *Local) CreateInboxChannel(ctx context.Context, in core.InboxChannelInput) (*core.IssuedInboxChannel, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	c := core.InboxChannel{ActorID: actor.ID, Active: in.Active == nil || *in.Active}
	if err := l.applyChannel(&c, &in.Name, &in.URL, &in.Format, in.Kinds); err != nil {
		return nil, err
	}
	if in.Kinds == nil {
		c.Kinds = core.InboxKinds()
	}
	if c.Secret, err = newWebhookSecret(); err != nil {
		return nil, err
	}
	err = l.write(ctx, actor, func(m *mutation) error {
		have, err := m.tx.ListInboxChannels(ctx, []string{actor.ID})
		if err != nil {
			return err
		}
		if len(have) >= core.MaxChannelsPerPerson {
			return core.Precondition("you have %d channels, the most one person may keep", len(have))
		}
		for _, h := range have {
			if strings.EqualFold(h.Name, c.Name) {
				return core.Conflict("you already have a channel named %q", c.Name).WithDetail(core.DetailField, "name")
			}
		}
		if err := m.tx.CreateInboxChannel(ctx, &c); err != nil {
			return err
		}
		return m.Record(auditChannelCreate, core.EventInboxUpdated, inboxSubject, c.ID, "", nil, auditChannel(c),
			channelPayload(actor.ID, c))
	})
	if err != nil {
		return nil, err
	}
	return &core.IssuedInboxChannel{InboxChannel: c, SigningSecret: c.Secret}, nil
}

// UpdateInboxChannel changes one of the caller's channels. A change that
// changes nothing writes nothing.
func (l *Local) UpdateInboxChannel(ctx context.Context, id string, in core.InboxChannelUpdate) (*core.InboxChannel, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var out *core.InboxChannel
	err = l.write(ctx, actor, func(m *mutation) error {
		c, err := ownChannel(ctx, m.tx, actor.ID, id)
		if err != nil {
			return err
		}
		before := *c
		var kinds []string
		if in.Kinds != nil {
			kinds = *in.Kinds
			if kinds == nil {
				kinds = []string{}
			}
		}
		if err := l.applyChannel(c, in.Name, in.URL, in.Format, kinds); err != nil {
			return err
		}
		if in.Active != nil {
			c.Active = *in.Active
		}
		out = c
		if sameChannel(before, *c) {
			return nil
		}
		if c.Name != before.Name {
			have, err := m.tx.ListInboxChannels(ctx, []string{actor.ID})
			if err != nil {
				return err
			}
			for _, h := range have {
				if h.ID != c.ID && strings.EqualFold(h.Name, c.Name) {
					return core.Conflict("you already have a channel named %q", c.Name).WithDetail(core.DetailField, "name")
				}
			}
		}
		if err := m.tx.UpdateInboxChannel(ctx, c); err != nil {
			return err
		}
		return m.Record(auditChannelUpdate, core.EventInboxUpdated, inboxSubject, c.ID, "", auditChannel(before), auditChannel(*c),
			channelPayload(actor.ID, *c))
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// DeleteInboxChannel removes one of the caller's channels and its deliveries.
func (l *Local) DeleteInboxChannel(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		c, err := ownChannel(ctx, m.tx, actor.ID, id)
		if err != nil {
			return err
		}
		if err := m.tx.DeleteInboxChannel(ctx, c.ID); err != nil {
			return err
		}
		return m.Record(auditChannelDelete, core.EventInboxUpdated, inboxSubject, c.ID, "", auditChannel(*c), nil,
			channelPayload(actor.ID, *c))
	})
}

// TestInboxChannel queues a test message to one of the caller's channels and
// returns the delivery as it stands once this process has drained, which in
// the inline mode is after the attempt.
func (l *Local) TestInboxChannel(ctx context.Context, id string) (*core.ChannelDelivery, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var d core.ChannelDelivery
	err = l.write(ctx, actor, func(m *mutation) error {
		c, err := ownChannel(ctx, m.tx, actor.ID, id)
		if err != nil {
			return err
		}
		if !c.Active {
			return core.Precondition("channel %q is turned off; turn it on to test it", c.Name)
		}
		d = core.ChannelDelivery{ChannelID: c.ID, NextAttemptAt: m.now}
		if err := m.tx.EnqueueChannelDelivery(ctx, &d); err != nil {
			return err
		}
		m.queued++
		return m.Record(auditChannelTest, core.EventInboxUpdated, inboxSubject, c.ID, "", nil, d,
			channelPayload(actor.ID, *c))
	})
	if err != nil {
		return nil, err
	}
	return l.channelDelivery(ctx, actor, d.ID)
}

func (l *Local) channelDelivery(ctx context.Context, actor *core.Actor, id string) (*core.ChannelDelivery, error) {
	var out *core.ChannelDelivery
	err := l.read(ctx, actor, func(tx store.Tx) error {
		var err error
		out, err = tx.GetChannelDelivery(ctx, id)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// ListChannelDeliveries returns a page of one of the caller's channels'
// deliveries, newest first.
func (l *Local) ListChannelDeliveries(ctx context.Context, f core.ChannelDeliveryFilter) ([]core.ChannelDelivery, string, error) {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	f.Page.Sort, f.Page.Direction = core.SortCreatedAt, core.Descending
	page, err := f.Page.Normalize()
	if err != nil {
		return nil, "", err
	}
	if err := mustCursorMatch(page); err != nil {
		return nil, "", err
	}
	f.Page = page
	var out []core.ChannelDelivery
	err = l.read(ctx, actor, func(tx store.Tx) error {
		if _, err := ownChannel(ctx, tx, actor.ID, f.ChannelID); err != nil {
			return err
		}
		out, err = tx.ListChannelDeliveries(ctx, f)
		return err
	})
	if err != nil {
		return nil, "", err
	}
	next := ""
	if len(out) > 0 && len(out) >= page.Limit {
		last := out[len(out)-1]
		next = core.Cursor{SortValue: sqlb.TimeText(last.CreatedAt), ID: last.ID,
			Sort: page.Sort, Direction: page.Direction}.Encode()
	}
	return out, next, nil
}

// RedeliverChannelDelivery queues another attempt of one of the caller's
// deliveries; the attempt happens on the next drain.
func (l *Local) RedeliverChannelDelivery(ctx context.Context, id string) error {
	actor, err := l.authorize(ctx, authz.ActionInboxRead, authz.Resource{})
	if err != nil {
		return err
	}
	return l.write(ctx, actor, func(m *mutation) error {
		d, err := m.tx.GetChannelDelivery(ctx, strings.TrimSpace(id))
		if err != nil {
			return err
		}
		c, err := ownChannel(ctx, m.tx, actor.ID, d.ChannelID)
		if err != nil {
			return core.NotFound("channel delivery %q", id)
		}
		if err := m.tx.MarkChannelFailed(ctx, d.ID, 0, "", m.now, false); err != nil {
			return err
		}
		m.queued++
		after, err := m.tx.GetChannelDelivery(ctx, d.ID)
		if err != nil {
			return err
		}
		return m.Record(auditChannelRedeliver, core.EventInboxUpdated, inboxSubject, c.ID, "", *d, *after,
			channelPayload(actor.ID, *c))
	})
}

// ownChannel reads one of the actor's channels. Somebody else's is not found,
// which is what it is to this caller.
func ownChannel(ctx context.Context, tx store.Tx, actorID, id string) (*core.InboxChannel, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, core.Invalid("channel identifier is required")
	}
	c, err := tx.GetInboxChannel(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.ActorID != actorID {
		return nil, core.NotFound("inbox channel %q", id)
	}
	return c, nil
}

// applyChannel sets the fields given, validating each. A nil field is left.
func (l *Local) applyChannel(c *core.InboxChannel, name, target, format *string, kinds []string) error {
	if name != nil {
		n := strings.TrimSpace(*name)
		if n == "" {
			return core.Invalid("a channel needs a name").WithDetail(core.DetailField, "name")
		}
		if len(n) > core.MaxChannelName {
			return core.Invalid("a channel name must be at most %d characters", core.MaxChannelName).
				WithDetail(core.DetailField, "name")
		}
		c.Name = n
	}
	if target != nil {
		u := strings.TrimSpace(*target)
		if err := l.webhookGuard().CheckURL(u); err != nil {
			return core.OnField(err, "url")
		}
		if err := checkTransport(u, l.allowInsecureWebhooks); err != nil {
			return core.OnField(err, "url")
		}
		c.URL = u
	}
	if format != nil {
		f := core.ChannelFormat(strings.ToLower(strings.TrimSpace(*format)))
		if f == "" {
			f = core.ChannelGeneric
		}
		if !f.Valid() {
			return core.Invalid("unknown channel format %q: use generic, slack, discord or ntfy", *format).
				WithDetail(core.DetailField, "format")
		}
		c.Format = f
	}
	if kinds != nil {
		parsed, err := core.ParseInboxKinds(kinds)
		if err != nil {
			return core.OnField(err, "kinds")
		}
		if len(parsed) == 0 {
			return core.Invalid("a channel needs at least one kind of item").WithDetail(core.DetailField, "kinds")
		}
		c.Kinds = canonicalKinds(parsed)
	}
	return nil
}

func sameChannel(a, b core.InboxChannel) bool {
	return a.Name == b.Name && a.URL == b.URL && a.Format == b.Format && a.Active == b.Active &&
		slices.Equal(a.Kinds, b.Kinds)
}

// auditChannel is a channel as the audit log keeps it: its url cut to the
// scheme and host, because a Slack or Discord webhook url is itself the
// credential, and the log is read by people who are not the channel's owner.
func auditChannel(c core.InboxChannel) core.InboxChannel {
	if u, err := url.Parse(c.URL); err == nil && u.Host != "" {
		c.URL = u.Scheme + "://" + u.Host + "/…"
	} else {
		c.URL = ""
	}
	return c
}

// channelPayload is what the event about a channel says: whose it is and its
// name, never its url, which may carry a token in its path.
func channelPayload(actorID string, c core.InboxChannel) map[string]any {
	return map[string]any{"recipients": []string{actorID}, "channel": c.Name, "channel_id": c.ID}
}
