// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// checkProjectLease refuses project lease bounds a claim could never be held
// to. A figure somebody types above the ceiling is refused here rather than
// clamped at claim time, so what the screen shows is what claims get; a
// ceiling lowered afterwards still clamps, see lease.Policy.Grant.
func (l *Local) checkProjectLease(ctx context.Context, tx store.Tx, p *core.Project) error {
	if p.LeaseDefault < 0 {
		return core.Invalid("lease default %s must be positive", p.LeaseDefault).WithDetail(core.DetailField, "lease_default")
	}
	if p.LeaseMax < 0 {
		return core.Invalid("lease maximum %s must be positive", p.LeaseMax).WithDetail(core.DetailField, "lease_max")
	}
	pol, err := l.leasePolicy(ctx, tx)
	if err != nil {
		return err
	}
	if p.LeaseMax > 0 && p.LeaseMax.D() > pol.Max {
		return core.Invalid("project lease maximum %s is above the tenant's %s; a project may only lower it",
			p.LeaseMax, core.Duration(pol.Max)).WithDetail(core.DetailField, "lease_max")
	}
	ceiling := pol.Cap(p.LeaseMax, core.LeaseFromProject).Max
	if p.LeaseDefault > 0 && p.LeaseDefault.D() > ceiling {
		return core.Invalid("project lease default %s is longer than the maximum of %s",
			p.LeaseDefault, core.Duration(ceiling)).WithDetail(core.DetailField, "lease_default")
	}
	return nil
}

// checkTaskLease refuses a task lease longer than the ceiling its project is
// held to now, for the reason checkProjectLease gives.
func (l *Local) checkTaskLease(ctx context.Context, tx store.Tx, t *core.Task) error {
	if t.Lease < 0 {
		return core.Invalid("task lease %s must be positive", t.Lease).WithDetail(core.DetailField, "lease")
	}
	if t.Lease == 0 {
		return nil
	}
	grant, err := l.leaseGrant(ctx, tx, t, 0)
	if err != nil {
		return err
	}
	if grant.Clamped {
		return core.Invalid("task lease %s is longer than the maximum of %s", t.Lease, grant.Max).
			WithDetail(core.DetailField, "lease")
	}
	return nil
}
