// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/lease"
	"github.com/heliopsy/tix/internal/store"
)

// setTenantLease sets this tenant's bounds through the service, so each case
// below is arranged the way an administrator would arrange it rather than by
// reaching past the validation that governs the real path.
func (f *claimFixture) setTenantLease(t *testing.T, def, max core.Duration) {
	t.Helper()
	if _, err := f.local.UpdateTenant(f.ctx, "", core.UpdateTenantInput{
		LeaseDefault: &def,
		LeaseMax:     &max,
	}); err != nil {
		t.Fatalf("setting tenant lease bounds: %v", err)
	}
}

// markWorkflowBuiltin makes the fixture's workflow the shipped template, which
// is what every tenant is actually created with. The distinction is the whole
// subject of TestTheBuiltinWorkflowsLeaseDoesNotMaskTheTenants.
func (f *claimFixture) markWorkflowBuiltin(t *testing.T) {
	t.Helper()
	ctx := context.Background()
	if err := f.local.store.Update(ctx, f.scope, func(tx store.Tx) error {
		f.workflow.Builtin = true
		return tx.PutWorkflow(ctx, f.workflow)
	}); err != nil {
		t.Fatalf("marking the workflow builtin: %v", err)
	}
}

func (f *claimFixture) claimHeldFor(t *testing.T, title string, ttl core.Duration) time.Duration {
	t.Helper()
	f.seedTask(t, title, "todo", core.PriorityNormal)
	claim, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{TTL: ttl})
	if err != nil {
		t.Fatalf("claiming %q: %v", title, err)
	}
	return claim.LeaseExpiresAt.Sub(f.clock.Now())
}

// TestTheTenantsLeaseDefaultGovernsWhenNoWorkflowAuthoredOne is the plain case
// the setting exists for: an administrator says how long claims hold on this
// tenant, and they hold that long.
func TestTheTenantsLeaseDefaultGovernsWhenNoWorkflowAuthoredOne(t *testing.T) {
	f := newClaimFixture(t, 0)
	f.setTenantLease(t, core.Duration(7*time.Minute), 0)

	if got := f.claimHeldFor(t, "a", 0); got != 7*time.Minute {
		t.Fatalf("lease = %s, want 7m", got)
	}
}

// TestTheBuiltinWorkflowsLeaseDoesNotMaskTheTenants is the reason
// workflowLease exists. Every tenant is created with the shipped workflow and
// its 30m default_lease; if that figure counted, the tenant setting above
// would be invisible on every project of an ordinary installation and the
// administrator who set it would have no way to tell why.
func TestTheBuiltinWorkflowsLeaseDoesNotMaskTheTenants(t *testing.T) {
	f := newClaimFixture(t, core.Duration(30*time.Minute))
	f.markWorkflowBuiltin(t)
	f.setTenantLease(t, core.Duration(3*time.Minute), 0)

	if got := f.claimHeldFor(t, "a", 0); got != 3*time.Minute {
		t.Fatalf("lease = %s, want 3m: the builtin template's 30m masked the tenant setting", got)
	}
}

// TestAnAuthoredWorkflowsLeaseOutranksTheTenants is the other half of that
// rule, and the one it must not break: a lease somebody wrote into a workflow
// is a decision about that kind of work, and it still wins.
func TestAnAuthoredWorkflowsLeaseOutranksTheTenants(t *testing.T) {
	f := newClaimFixture(t, core.Duration(2*time.Hour))
	f.setTenantLease(t, core.Duration(3*time.Minute), 0)

	if got := f.claimHeldFor(t, "a", 0); got != 2*time.Hour {
		t.Fatalf("lease = %s, want 2h: an authored workflow must outrank the tenant", got)
	}
}

// TestTheTenantsLeaseMaximumClampsALongerDefault proves the maximum is a clamp
// and not merely a validator: a workflow asking for longer than the tenant
// permits is shortened, not refused, because nobody typed that figure today.
func TestTheTenantsLeaseMaximumClampsALongerDefault(t *testing.T) {
	f := newClaimFixture(t, core.Duration(2*time.Hour))
	f.setTenantLease(t, 0, core.Duration(10*time.Minute))

	if got := f.claimHeldFor(t, "a", 0); got != 10*time.Minute {
		t.Fatalf("lease = %s, want 10m", got)
	}
}

// TestAClaimAskingAboveTheTenantMaximumIsRefused is the companion: a caller
// who named the figure is told, because the error is the answer they can act
// on. The two cases differ deliberately; see Policy.Resolve.
func TestAClaimAskingAboveTheTenantMaximumIsRefused(t *testing.T) {
	f := newClaimFixture(t, 0)
	f.setTenantLease(t, 0, core.Duration(10*time.Minute))
	f.seedTask(t, "a", "todo", core.PriorityNormal)

	_, err := f.local.ClaimNext(f.ctx, core.ClaimNextInput{TTL: core.Duration(2 * time.Hour)})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("claiming above the tenant maximum = %v, want an invalid error", err)
	}
}

// TestATenantMayLowerTheDeploymentCeilingButNotRaiseIt is the multi-tenant
// guarantee. The operator owns the machine, so a tenant tightening the bound
// is choosing for itself and a tenant loosening it would be choosing for
// somebody else.
func TestATenantMayLowerTheDeploymentCeilingButNotRaiseIt(t *testing.T) {
	deployment := lease.Policy{Default: 20 * time.Minute, Max: time.Hour}

	lowered := deployment.Narrow(0, core.Duration(10*time.Minute))
	if lowered.Max != 10*time.Minute {
		t.Fatalf("tightened ceiling = %s, want 10m", lowered.Max)
	}

	raised := deployment.Narrow(0, core.Duration(8*time.Hour))
	if raised.Max != time.Hour {
		t.Fatalf("ceiling after a tenant asked for 8h = %s, want the deployment's 1h", raised.Max)
	}
}

// TestADefaultAboveTheCeilingIsClampedRatherThanBreakingEveryClaim covers the
// drift an operator creates by lowering lease.max underneath a tenant that
// configured a longer default months earlier. Refusing would stop all work on
// that tenant until somebody noticed; shortening the lease does not.
func TestADefaultAboveTheCeilingIsClampedRatherThanBreakingEveryClaim(t *testing.T) {
	p := lease.Policy{Default: 4 * time.Hour, Max: time.Hour}

	got, err := p.Resolve(0, 0)
	if err != nil {
		t.Fatalf("resolving a default above the ceiling: %v, want it clamped", err)
	}
	if got != time.Hour {
		t.Fatalf("clamped lease = %s, want 1h", got)
	}
}
