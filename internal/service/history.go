// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/outbox"
	"github.com/heliopsy/tix/internal/retention"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// auditSortColumns are the orderings an audit listing accepts.
var auditSortColumns = map[string]bool{"seq": true, "occurred_at": true}

const defaultAuditSort = "seq"

// ListAudit returns one keyset-paginated page of audit entries plus the cursor
// that resumes the listing.
func (l *Local) ListAudit(ctx context.Context, f core.AuditFilter) ([]core.AuditEntry, string, error) {
	actor, err := l.authorize(ctx, authz.ActionAuditRead, authz.Resource{})
	if err != nil {
		return nil, "", err
	}
	page, err := normalizeAuditPage(f.Page)
	if err != nil {
		return nil, "", err
	}
	f.Page = page

	var entries []core.AuditEntry
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		found, err := tx.ListAudit(ctx, f)
		if err != nil {
			return err
		}
		entries = found
		return nil
	}); err != nil {
		return nil, "", err
	}
	if len(entries) < page.Limit {
		return entries, "", nil
	}
	return entries, auditCursor(entries[len(entries)-1], page), nil
}

// normalizeAuditPage clamps the page and rejects a cursor from another ordering.
func normalizeAuditPage(p core.Page) (core.Page, error) {
	if p.Sort == "" {
		p.Sort = defaultAuditSort
	}
	if !auditSortColumns[p.Sort] {
		return core.Page{}, core.Invalid("cannot sort audit entries by %q", p.Sort)
	}
	norm, err := p.Normalize()
	if err != nil {
		return core.Page{}, err
	}
	c, err := core.DecodeCursor(norm.Cursor)
	if err != nil {
		return core.Page{}, err
	}
	if err := c.CheckOrdering(norm.Sort, norm.Direction); err != nil {
		return core.Page{}, err
	}
	return norm, nil
}

// auditCursor encodes the position of the last entry on a page.
func auditCursor(e core.AuditEntry, p core.Page) string {
	seq := strconv.FormatInt(e.Seq, 10)
	value := seq
	if p.Sort == "occurred_at" {
		value = sqlb.TimeText(e.OccurredAt)
	}
	return core.Cursor{SortValue: value, ID: seq, Sort: p.Sort, Direction: p.Direction}.Encode()
}

// Subscribe streams committed events from the durable outbox. The returned
// channel closes when ctx is cancelled.
func (l *Local) Subscribe(ctx context.Context, f core.EventFilter) (<-chan core.Event, error) {
	actor, err := l.authorize(ctx, authz.ActionEventSubscribe, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if f.SinceSeq < 0 {
		return nil, core.Invalid("since_seq must not be negative")
	}
	reader := &eventReader{local: l, actor: actor}
	if f.SinceSeq == 0 {
		latest, err := reader.Latest(ctx)
		if err != nil {
			return nil, err
		}
		f.SinceSeq = latest
	}
	src, err := outbox.NewTailer(reader, 0, 0).Subscribe(ctx, f)
	if err != nil {
		return nil, err
	}

	sub := subscribers.add(actor.TenantID, f.SinceSeq)
	out := make(chan core.Event)
	go func() {
		defer close(out)
		defer subscribers.remove(actor.TenantID, sub)
		for e := range src {
			select {
			case out <- e:
				sub.advance(e.Seq)
			case <-ctx.Done():
				return
			}
		}
	}()
	return out, nil
}

// eventReader adapts the store to the outbox tailer for one tenant.
type eventReader struct {
	local *Local
	actor *core.Actor
}

// ReadSince returns committed events after a sequence number.
func (r *eventReader) ReadSince(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error) {
	var out []core.Event
	err := r.local.read(ctx, r.actor, func(tx store.Tx) error {
		events, err := tx.ReadEvents(ctx, sinceSeq, limit)
		if err != nil {
			return err
		}
		out = events
		return nil
	})
	return out, err
}

// Latest returns the highest committed sequence number.
func (r *eventReader) Latest(ctx context.Context) (int64, error) {
	var seq int64
	err := r.local.read(ctx, r.actor, func(tx store.Tx) error {
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		seq = latest
		return nil
	})
	return seq, err
}

// subscribers tracks the resume cursor of every live subscription in this
// process, because pruning must not remove an event one of them still needs.
var subscribers = newSubscriberRegistry()

// subscriber holds one live subscription's resume cursor.
type subscriber struct{ cursor atomic.Int64 }

func (s *subscriber) advance(seq int64) {
	for {
		cur := s.cursor.Load()
		if seq <= cur || s.cursor.CompareAndSwap(cur, seq) {
			return
		}
	}
}

func (s *subscriber) at() int64 { return s.cursor.Load() }

// subscriberRegistry groups live subscriptions by tenant.
type subscriberRegistry struct {
	mu       sync.Mutex
	byTenant map[string]map[*subscriber]struct{}
}

func newSubscriberRegistry() *subscriberRegistry {
	return &subscriberRegistry{byTenant: map[string]map[*subscriber]struct{}{}}
}

func (r *subscriberRegistry) add(tenantID string, cursor int64) *subscriber {
	s := &subscriber{}
	s.cursor.Store(cursor)
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.byTenant[tenantID] == nil {
		r.byTenant[tenantID] = map[*subscriber]struct{}{}
	}
	r.byTenant[tenantID][s] = struct{}{}
	return s
}

func (r *subscriberRegistry) remove(tenantID string, s *subscriber) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.byTenant[tenantID], s)
	if len(r.byTenant[tenantID]) == 0 {
		delete(r.byTenant, tenantID)
	}
}

// floor returns the lowest resume cursor held by a live subscriber.
func (r *subscriberRegistry) floor(tenantID string) (int64, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	subs := r.byTenant[tenantID]
	if len(subs) == 0 {
		return 0, false
	}
	lowest := int64(-1)
	for s := range subs {
		if at := s.at(); lowest < 0 || at < lowest {
			lowest = at
		}
	}
	return lowest, true
}

// GetRetention returns the tenant's effective retention policy.
func (l *Local) GetRetention(ctx context.Context) (*core.RetentionPolicy, error) {
	actor, err := l.authorize(ctx, authz.ActionTenantAdmin, authz.Resource{})
	if err != nil {
		return nil, err
	}
	var p core.RetentionPolicy
	if err := l.read(ctx, actor, func(tx store.Tx) error {
		stored, err := tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		p = retention.Effective(*stored)
		return nil
	}); err != nil {
		return nil, err
	}
	p.TenantID = actor.TenantID
	return &p, nil
}

// PutRetention stores the tenant's retention policy. Windows left unset keep
// their current value, so changing one class never disturbs another.
func (l *Local) PutRetention(ctx context.Context, next core.RetentionPolicy) (*core.RetentionPolicy, error) {
	actor, err := l.authorize(ctx, authz.ActionRetentionWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if err := retention.Validate(next); err != nil {
		return nil, err
	}
	next.TenantID = actor.TenantID

	var merged core.RetentionPolicy
	err = l.write(ctx, actor, func(m *mutation) error {
		stored, err := m.tx.GetRetention(ctx)
		if err != nil {
			return err
		}
		before := retention.Effective(*stored)
		merged = retention.Effective(retention.Merge(before, next))
		if err := m.tx.PutRetention(ctx, &merged); err != nil {
			return err
		}
		return m.Audit("retention.update", "tenant", actor.TenantID, before, merged)
	})
	if err != nil {
		return nil, err
	}
	return &merged, nil
}
