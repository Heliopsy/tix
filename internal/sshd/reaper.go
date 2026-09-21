package sshd

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// reap deletes sandboxes nobody has visited for the time to live, until ctx
// is cancelled.
func (s *Server) reap(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-s.opts.Clock.After(s.opts.ReapInterval):
		}
		n, err := reapOnce(ctx, s.opts.Store, s.opts.Clock.Now(), s.opts.TenantTTL)
		switch {
		case err != nil:
			s.log.Error("reaping demo sandboxes failed", "error", err.Error())
		case n > 0:
			s.log.Info("reaped demo sandboxes", "count", n, "ttl", s.opts.TenantTTL.String())
		}
	}
}

// reapOnce deletes every sandbox last seen before now minus ttl, and reports
// how many went. The deletion is the store's hard delete, so the tenant's rows
// cascade away rather than lingering behind a soft-deleted marker.
func reapOnce(ctx context.Context, st store.Store, now time.Time, ttl time.Duration) (int, error) {
	cutoff := now.Add(-ttl)
	var stale []string
	if err := walkTenants(ctx, st, func(t core.Tenant) {
		if isSandbox(t.Key) && t.UpdatedAt.Before(cutoff) {
			stale = append(stale, t.ID)
		}
	}); err != nil {
		return 0, err
	}
	for _, tenantID := range stale {
		if err := st.Unscoped(ctx, func(u store.UnscopedTx) error {
			return u.DeleteTenant(ctx, tenantID)
		}); err != nil && !core.IsKind(err, core.KindNotFound) {
			return 0, err
		}
	}
	return len(stale), nil
}
