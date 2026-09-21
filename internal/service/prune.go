package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/retention"
	"github.com/heliopsy/tix/internal/store"
)

// pruneWindow is one class of records and the instant before which it expired.
type pruneWindow struct {
	policy core.RetentionPolicy
	events time.Time
	audit  time.Time
	hooks  time.Time
	floor  int64
	limit  int
}

// Prune removes records past their retention window. A dry run reports what
// would go and writes nothing at all, not even an audit entry.
func (l *Local) Prune(ctx context.Context, in core.PruneInput) (*core.PruneResult, error) {
	actor, err := l.authorize(ctx, authz.ActionRetentionWrite, authz.Resource{})
	if err != nil {
		return nil, err
	}
	if in.Limit < 0 {
		return nil, core.Invalid("prune limit must not be negative")
	}
	res := &core.PruneResult{DryRun: in.DryRun}
	if in.DryRun {
		err = l.read(ctx, actor, func(tx store.Tx) error {
			w, err := l.pruneWindow(ctx, tx, actor, in.Limit)
			if err != nil {
				return err
			}
			return countExpired(ctx, tx, w, res)
		})
		if err != nil {
			return nil, err
		}
		return res, nil
	}

	err = l.write(ctx, actor, func(m *mutation) error {
		w, err := l.pruneWindow(ctx, m.tx, actor, in.Limit)
		if err != nil {
			return err
		}
		if res.Events, err = m.tx.PruneEvents(ctx, w.events, w.floor, w.limit); err != nil {
			return err
		}
		if res.AuditEntries, err = m.tx.PruneAudit(ctx, w.audit, w.limit); err != nil {
			return err
		}
		if res.WebhookDeliveries, err = m.tx.PruneDeliveries(ctx, w.hooks, w.limit); err != nil {
			return err
		}
		if res.RetainedForSubscribers, err = countRetained(ctx, m.tx, w); err != nil {
			return err
		}
		if res.Events+res.AuditEntries+res.WebhookDeliveries == 0 {
			return nil
		}
		return m.Audit("retention.prune", "tenant", actor.TenantID, nil, pruneSummary(w, res))
	})
	if err != nil {
		return nil, err
	}
	return res, nil
}

// pruneWindow resolves the effective policy, its cutoffs, and the highest event
// sequence pruning may touch without stranding a live subscriber.
func (l *Local) pruneWindow(ctx context.Context, tx store.Tx, actor *core.Actor, limit int) (pruneWindow, error) {
	stored, err := tx.GetRetention(ctx)
	if err != nil {
		return pruneWindow{}, err
	}
	policy := retention.Resolve(*stored, l.retentionDefaults)
	now := l.clock.Now()
	cutoffs := retention.Cutoffs(policy, now)

	latest, err := tx.LatestEventSeq(ctx)
	if err != nil {
		return pruneWindow{}, err
	}
	floor := latest
	if lowest, ok := subscribers.floor(actor.TenantID); ok && lowest-1 < floor {
		floor = lowest - 1
	}
	if limit <= 0 {
		limit = core.MaxPageLimit
	}
	return pruneWindow{
		policy: policy,
		events: cutoffs[retention.ClassEvents],
		audit:  cutoffs[retention.ClassAudit],
		hooks:  cutoffs[retention.ClassDeliveries],
		floor:  floor,
		limit:  limit,
	}, nil
}

// countExpired fills a dry-run result without touching a row.
func countExpired(ctx context.Context, tx store.Tx, w pruneWindow, res *core.PruneResult) error {
	events, err := tx.ReadEvents(ctx, 0, w.limit)
	if err != nil {
		return err
	}
	for _, e := range events {
		if !e.OccurredAt.Before(w.events) {
			continue
		}
		if e.Seq <= w.floor {
			res.Events++
			continue
		}
		res.RetainedForSubscribers++
	}

	until := w.audit.Add(-time.Nanosecond)
	entries, err := tx.ListAudit(ctx, core.AuditFilter{
		Until: &until,
		Page:  core.Page{Limit: w.limit, Sort: defaultAuditSort},
	})
	if err != nil {
		return err
	}
	res.AuditEntries = int64(len(entries))

	deliveries, err := tx.ListDeliveries(ctx, core.DeliveryFilter{
		Statuses: []core.DeliveryStatus{core.DeliveryDelivered, core.DeliveryFailed},
		Page:     core.Page{Limit: w.limit},
	})
	if err != nil {
		return err
	}
	for _, d := range deliveries {
		if d.CreatedAt.Before(w.hooks) {
			res.WebhookDeliveries++
		}
	}
	return nil
}

// countRetained reports how many expired events a live subscriber holds back.
func countRetained(ctx context.Context, tx store.Tx, w pruneWindow) (int64, error) {
	events, err := tx.ReadEvents(ctx, w.floor, w.limit)
	if err != nil {
		return 0, err
	}
	var n int64
	for _, e := range events {
		if e.OccurredAt.Before(w.events) {
			n++
		}
	}
	return n, nil
}

// pruneSummary is the audit payload of a pruning run.
func pruneSummary(w pruneWindow, res *core.PruneResult) map[string]any {
	return map[string]any{
		"windows": map[string]string{
			string(retention.ClassEvents):     w.policy.Events.String(),
			string(retention.ClassAudit):      w.policy.AuditEntries.String(),
			string(retention.ClassDeliveries): w.policy.WebhookDeliveries.String(),
		},
		"removed": map[string]int64{
			string(retention.ClassEvents):     res.Events,
			string(retention.ClassAudit):      res.AuditEntries,
			string(retention.ClassDeliveries): res.WebhookDeliveries,
		},
		"retained_for_subscribers": res.RetainedForSubscribers,
		"limit":                    w.limit,
	}
}
