// SPDX-License-Identifier: AGPL-3.0-or-later

package sqlite

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// WorkCounts totals what this tenant holds.
func (t *tx) WorkCounts(ctx context.Context, now time.Time) (store.WorkCounts, error) {
	at := sqlb.TimeText(now)
	var out store.WorkCounts
	for _, c := range []struct {
		into  *int
		what  string
		build func() *sqlb.Builder
	}{
		{&out.Projects, "counting projects", func() *sqlb.Builder {
			return t.builder("projects")
		}},
		{&out.Tasks, "counting tasks", func() *sqlb.Builder {
			return t.builder("tasks").Where("deleted_at IS NULL")
		}},
		{&out.Claimed, "counting claimed tasks", func() *sqlb.Builder {
			return t.builder("tasks").Where("deleted_at IS NULL").
				Where("claimed_by_actor_id IS NOT NULL").
				Where("lease_expires_at > ?", at)
		}},
		{&out.LeasesExpiredUnswept, "counting unswept leases", func() *sqlb.Builder {
			return t.builder("tasks").Where("deleted_at IS NULL").
				Where("claimed_by_actor_id IS NOT NULL").
				Where("lease_expires_at <= ?", at)
		}},
		{&out.WebhooksPending, "counting pending deliveries", func() *sqlb.Builder {
			return t.builder("webhook_deliveries").Where("status = ?", string(core.DeliveryPending))
		}},
		{&out.WebhooksFailed, "counting failed deliveries", func() *sqlb.Builder {
			return t.builder("webhook_deliveries").Where("status = ?", string(core.DeliveryFailed))
		}},
	} {
		n, err := t.count(ctx, c.build(), "%s", c.what)
		if err != nil {
			return store.WorkCounts{}, err
		}
		*c.into = n
	}
	return out, nil
}
