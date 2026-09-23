// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
)

// GET /api/v1/domains/{hostname} is authenticated but carries no tenant-admin
// check, unlike its three siblings on the same path. Answering it from the
// unscoped resolver handed any credential of one tenant the record of another.
func TestResolveDomainRouteRefusesAnotherTenantsHostname(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodGet, "/api/v1/domains/"+f.hostB, nil)
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("status = %d, want %d: %s", resp.StatusCode, http.StatusNotFound, body)
	}
	for _, leaked := range []string{f.tenantB.ID, f.tenantB.Name} {
		if strings.Contains(body, leaked) {
			t.Errorf("response disclosed the other tenant's %q: %s", leaked, body)
		}
	}
}

// The tenant's own hostname still resolves, which is what the remote client
// calls the route for.
func TestResolveDomainRouteAnswersItsOwnHostname(t *testing.T) {
	f := newFixture(t)

	resp := f.call(http.MethodGet, "/api/v1/domains/"+f.hostA, nil)
	mustStatus(t, resp, http.StatusOK)

	var tenant core.Tenant
	decodeBody(t, resp, &tenant)
	if tenant.ID != f.tenantA.ID {
		t.Fatalf("resolved tenant %q, want %q", tenant.ID, f.tenantA.ID)
	}
}
