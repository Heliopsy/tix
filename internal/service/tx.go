package service

import (
	"context"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/outbox"
	"github.com/thereisnotime/tix/internal/store"
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
		Payload:     payload,
		OccurredAt:  m.now,
	})
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
	m.Event(typ, subjectType, subjectID, projectID, payload)
	return nil
}

// flush writes the queued audit entries and events. Called before commit.
func (m *mutation) flush(ctx context.Context) error {
	for _, a := range m.audits {
		if err := m.tx.AppendAudit(ctx, a); err != nil {
			return err
		}
	}
	for _, e := range m.events {
		if err := outbox.Append(ctx, m.tx, m.local.ids, m.now, e); err != nil {
			return err
		}
	}
	return nil
}

// write runs fn inside one transaction, flushing its audit entries and events
// before commit. Every mutating service method goes through here; there is no
// other way to reach a writable transaction.
func (l *Local) write(ctx context.Context, actor *core.Actor, fn func(*mutation) error) error {
	scope := core.TenantScope{TenantID: actor.TenantID}
	return l.store.Update(ctx, scope, func(tx store.Tx) error {
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
		return m.flush(ctx)
	})
}

// read runs fn inside a read-only transaction.
func (l *Local) read(ctx context.Context, actor *core.Actor, fn func(store.Tx) error) error {
	return l.store.View(ctx, core.TenantScope{TenantID: actor.TenantID}, fn)
}
