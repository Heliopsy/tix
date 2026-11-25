// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/wire"
)

// TestThePolicyRefusesAPersonTokenWith403 is the API's side of the second
// factor policy: a token minted for a person who holds no factor is refused
// with 403 and the reason a client can act on, while an agent's token in the
// same tenant goes on working.
func TestThePolicyRefusesAPersonTokenWith403(t *testing.T) {
	f := newFixtureOver(t, []service.Option{service.WithTOTPKey("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")})
	person := seedLinkedActor(t, f.store, f.tenantA.ID, "ada", "ada@acme.test", false)
	personToken := f.mintScoped(f.tenantA.ID, person.ID, core.ScopeAll)

	ctx := context.Background()
	if err := f.store.Unscoped(ctx, func(u store.UnscopedTx) error {
		tenant, err := u.GetTenantByID(ctx, f.tenantA.ID)
		if err != nil {
			return err
		}
		tenant.MFAPolicy = core.MFAPolicyRequired
		at := f.clock.Now()
		tenant.MFARequiredAt = &at
		return u.UpdateTenant(ctx, tenant)
	}); err != nil {
		t.Fatal(err)
	}

	refused := f.do(http.MethodGet, wire.RouteProjects, f.hostA, personToken, nil)
	mustStatus(t, refused, http.StatusForbidden)
	var body wire.ErrorEnvelope
	decodeBody(t, refused, &body)
	if body.Error.Details["reason"] != core.ReasonSecondFactorEnrollmentRequired {
		t.Fatalf("refusal = %+v, want reason %s", body.Error, core.ReasonSecondFactorEnrollmentRequired)
	}

	agent := f.do(http.MethodGet, wire.RouteProjects, f.hostA, f.tokenA, nil)
	mustStatus(t, agent, http.StatusOK)
	_ = agent.Body.Close()
}
