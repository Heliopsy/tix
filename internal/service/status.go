// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/authz"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
	"github.com/heliopsy/tix/internal/version"
)

// statusServerPages bounds how many pages of servers one report walks. The
// listing is keyset-paged like every other one, and a status report is not a
// paged surface, so it reads to exhaustion; the bound is here so a table that
// has somehow grown absurd cannot make the command hang instead of answering.
const statusServerPages = 64

// statusServerSort is the order the listing pages in, named once so the cursor
// the reader builds cannot disagree with the ordering the store applied.
const statusServerSort = "started_at"

// Status reports what the installation holds and what is running in it.
//
// The two halves are authorized as one and read differently. The server list is
// installation state and is read through the cross-tenant face of the same
// transaction, because a server belongs to no tenant. The work counts are read
// through the scoped face and cover this tenant only, so one tenant's report
// never states another's size.
//
// Staleness is decided here, against one instant captured before the read, so
// every server in one report is judged against the same clock.
func (l *Local) Status(ctx context.Context) (*core.StatusReport, error) {
	actor, err := l.authorize(ctx, authz.ActionServerRead, authz.Resource{})
	if err != nil {
		return nil, err
	}
	now := l.clock.Now()

	out := &core.StatusReport{
		Installation: core.Installation{
			Version:    version.Version,
			Engine:     string(l.store.Dialect()),
			ObservedAt: now,
		},
		Servers: []core.ServerStatus{},
	}
	if v, err := l.store.SchemaVersion(ctx); err == nil {
		out.Installation.SchemaVersion = v
	}

	err = l.read(ctx, actor, func(tx store.Tx) error {
		u, err := asUnscoped(tx)
		if err != nil {
			return err
		}
		servers, err := readServers(ctx, u)
		if err != nil {
			return err
		}
		for _, s := range servers {
			out.Servers = append(out.Servers, s.StatusAt(now))
		}
		// The tenant count is what this reader could already list, which is the
		// scoped read ListTenants performs and not the whole table. An
		// installation-wide count would tell the administrator of one tenant
		// that six others exist, which nothing else in the product tells them,
		// and it would do it from a report they run to count their own tasks.
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		if tenant.DeletedAt == nil {
			out.Work.Tenants = 1
		}

		counts, err := tx.WorkCounts(ctx, now)
		if err != nil {
			return err
		}
		out.Work.Projects = counts.Projects
		out.Work.Tasks = counts.Tasks
		out.Work.Claimed = counts.Claimed
		out.Work.LeasesExpiredUnswept = counts.LeasesExpiredUnswept
		out.Work.WebhooksPending = counts.WebhooksPending
		out.Work.WebhooksFailed = counts.WebhooksFailed
		return nil
	})
	if err != nil {
		return nil, err
	}
	l.attachConnectionCount(out)
	return out, nil
}

// readServers walks every page of the server listing.
func readServers(ctx context.Context, u store.UnscopedTx) ([]core.Server, error) {
	out := []core.Server{}
	page := core.Page{Limit: core.MaxPageLimit, Sort: statusServerSort}
	for range statusServerPages {
		batch, err := u.ListServers(ctx, page)
		if err != nil {
			return nil, err
		}
		out = append(out, batch...)
		if len(batch) < page.Limit {
			return out, nil
		}
		last := batch[len(batch)-1]
		page.Cursor = core.Cursor{
			SortValue: sqlb.TimeText(last.StartedAt),
			ID:        last.ID,
			Sort:      statusServerSort,
		}.Encode()
	}
	return out, nil
}

// attachConnectionCount puts this process's live connection count against this
// process's own row, and against no other.
//
// A connection is held in one process's memory, so a count can only be true of
// the process that answered. Every other row leaves it unset, which reads as
// "not known" rather than as zero: a zero would be a claim, and it would be the
// wrong one. A report produced against a local database with no server running
// sets none at all.
func (l *Local) attachConnectionCount(out *core.StatusReport) {
	if l.conns == nil {
		return
	}
	self := l.conns.ServerID()
	for i := range out.Servers {
		if out.Servers[i].ID != self {
			continue
		}
		n := l.conns.Counts("").Process
		out.Servers[i].Connections = &n
		return
	}
}
