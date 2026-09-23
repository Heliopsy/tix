// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/outbox"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/webhook"
)

// mutation carries everything one write needs to record about itself.
type mutation struct {
	tx     store.Tx
	actor  *core.Actor
	now    time.Time
	local  *Local
	source core.Source

	events []*core.Event
	audits []*core.AuditEntry

	// queued counts the webhook deliveries this mutation enqueued, so the
	// caller knows whether an inline drain has anything to do.
	queued int
}

// Event queues a domain event. It is written before commit, in this same
// transaction, so an event can never exist without the rows it describes.
func (m *mutation) Event(typ core.EventType, subjectType, subjectID, projectID string, payload map[string]any) {
	m.events = append(m.events, &core.Event{
		Type:        typ,
		ProjectID:   projectID,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		ActorID:     m.actor.ID,
		Payload:     m.withActor(payload),
		OccurredAt:  m.now,
	})
}

// withActor names the actor in the payload. ActorID alone is a ULID, and a
// stream a person is watching to see which agent took which task is useless if
// every line identifies the agent by an identifier nobody recognises. The
// mutation already holds the actor, so this costs no query. An explicit
// actor_handle already in the payload wins, and an actor with no handle adds
// nothing rather than an empty key.
func (m *mutation) withActor(payload map[string]any) map[string]any {
	if m.actor == nil || m.actor.Handle == "" {
		return payload
	}
	if _, ok := payload["actor_handle"]; ok {
		return payload
	}
	out := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		out[k] = v
	}
	out["actor_handle"] = m.actor.Handle
	return out
}

// Audit queues an audit entry. before is nil for a creation, after is nil for a
// deletion. Neither may contain a secret.
func (m *mutation) Audit(action, subjectType, subjectID string, before, after any) error {
	b, err := redactedJSON(before)
	if err != nil {
		return err
	}
	a, err := redactedJSON(after)
	if err != nil {
		return err
	}
	m.audits = append(m.audits, &core.AuditEntry{
		ActorID:     m.actor.ID,
		Action:      action,
		SubjectType: subjectType,
		SubjectID:   subjectID,
		Before:      b,
		After:       a,
		Source:      m.source,
		OccurredAt:  m.now,
	})
	return nil
}

// Record queues the audit entry and the event for one change together, so a
// caller cannot write one and forget the other.
func (m *mutation) Record(action string, typ core.EventType, subjectType, subjectID, projectID string, before, after any, payload map[string]any) error {
	if err := m.Audit(action, subjectType, subjectID, before, after); err != nil {
		return err
	}
	m.Event(typ, subjectType, subjectID, projectID, withRef(payload, before, after))
	return nil
}

// withRef adds the human reference of the subject to an event payload when the
// snapshots carry one. A consumer of the stream sees "infra-42" rather than a
// ULID, which is the only form a person recognises. Doing it here rather than at
// each call site is what stops the next event type from forgetting: a claim, a
// release and a lease expiry all used to ship without a ref while a create
// shipped with one, for no reason anybody chose.
func withRef(payload map[string]any, before, after any) map[string]any {
	if payload != nil {
		if _, ok := payload["ref"]; ok {
			return payload
		}
	}
	ref := refOf(after)
	if ref == "" {
		ref = refOf(before)
	}
	if ref == "" {
		return payload
	}
	out := make(map[string]any, len(payload)+1)
	for k, v := range payload {
		out[k] = v
	}
	out["ref"] = ref
	return out
}

// refOf reads the Ref of a snapshot that has one, by value or by pointer.
func refOf(v any) string {
	switch t := v.(type) {
	case core.Task:
		return t.Ref
	case *core.Task:
		if t != nil {
			return t.Ref
		}
	}
	return ""
}

// flush writes the queued audit entries and events, and fans each event out to
// the endpoints matching it. Called before commit, so a delivery is exactly as
// durable as the event that caused it. Only the attempt happens after commit.
func (m *mutation) flush(ctx context.Context) error {
	for _, a := range m.audits {
		if err := m.tx.AppendAudit(ctx, a); err != nil {
			return err
		}
	}
	q := &hookQueue{Tx: m.tx}
	for _, e := range m.events {
		if err := outbox.Append(ctx, m.tx, m.local.ids, m.now, e); err != nil {
			return err
		}
		if m.local.hooks == HookOff {
			continue
		}
		n, err := webhook.Enqueue(ctx, q, *e)
		if err != nil {
			return err
		}
		m.queued += n
	}
	return nil
}

// write runs fn inside one transaction, flushing its audit entries and events
// before commit. Every mutating service method goes through here; there is no
// other way to reach a writable transaction.
func (l *Local) write(ctx context.Context, actor *core.Actor, fn func(*mutation) error) error {
	scope := core.TenantScope{TenantID: actor.TenantID}
	queued := 0
	err := l.store.Update(ctx, scope, func(tx store.Tx) error {
		m := &mutation{
			tx:     tx,
			actor:  actor,
			now:    l.clock.Now(),
			local:  l,
			source: core.SourceFrom(ctx),
		}
		if err := fn(m); err != nil {
			return err
		}
		if err := m.flush(ctx); err != nil {
			return err
		}
		queued = m.queued
		return nil
	})
	if err != nil {
		return err
	}
	l.drainHooks(ctx, scope, queued)
	return nil
}

// read runs fn inside a read-only transaction.
func (l *Local) read(ctx context.Context, actor *core.Actor, fn func(store.Tx) error) error {
	return l.store.View(ctx, core.TenantScope{TenantID: actor.TenantID}, fn)
}
