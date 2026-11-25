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

// CountTrashed counts the soft-deleted tasks, in one project when projectID is
// set.
func (t *tx) CountTrashed(ctx context.Context, projectID string) (int, error) {
	b := t.builder("tasks").Where("deleted_at IS NOT NULL")
	if projectID != "" {
		b = b.Where("project_id = ?", projectID)
	}
	return t.count(ctx, b, "counting deleted tasks")
}

// highestSeq is the highest number a task of the outer projects row holds.
const highestSeq = "(SELECT MAX(numbered.seq) FROM tasks numbered " +
	"WHERE numbered.tenant_id = projects.tenant_id AND numbered.project_id = projects.id)"

// numberingBehindLimit bounds the diagnostic, which names projects rather
// than paging through them.
const numberingBehindLimit = 100

// NumberingBehind lists the projects whose task number counter reads below the
// highest number one of their tasks holds.
func (t *tx) NumberingBehind(ctx context.Context) ([]core.NumberingLag, error) {
	b := t.builder("projects").
		Select("projects.key", "projects.seq_counter", highestSeq).
		Where("projects.seq_counter < "+highestSeq).
		OrderBy("projects.key", core.Ascending).
		Limit(numberingBehindLimit)
	rows, err := t.query(ctx, b, "listing projects whose task numbering is behind")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	out := []core.NumberingLag{}
	for rows.Next() {
		var lag core.NumberingLag
		if err := rows.Scan(&lag.Project, &lag.Counter, &lag.Highest); err != nil {
			return nil, mapErr(err, "reading a project whose task numbering is behind")
		}
		out = append(out, lag)
	}
	if err := rows.Err(); err != nil {
		return nil, mapErr(err, "listing projects whose task numbering is behind")
	}
	return out, nil
}
