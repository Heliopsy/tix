// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"net/http"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestOnlyAGlobalAdminMarksADomainSharedOverHTTP(t *testing.T) {
	f := newFixture(t)
	path := "/api/v1/domains/" + f.hostB

	refused := f.do(http.MethodPatch, path, f.hostA, f.tokenA, map[string]any{"shared": true})
	mustStatus(t, refused, http.StatusForbidden)
	own := f.do(http.MethodPatch, "/api/v1/domains/"+f.hostA, f.hostA, f.tokenA, map[string]any{"shared": true})
	mustStatus(t, own, http.StatusForbidden)
	listedByAdmin := f.do(http.MethodGet, "/api/v1/tenants/other/domains", f.hostA, f.tokenA, nil)
	mustStatus(t, listedByAdmin, http.StatusForbidden)

	token := f.globalAdminToken()
	resp := f.do(http.MethodPatch, path, f.hostA, token, map[string]any{"shared": true})
	mustStatus(t, resp, http.StatusOK)
	var got core.Domain
	decodeBody(t, resp, &got)
	if !got.Shared || got.Hostname != f.hostB || got.TenantID != f.tenantB.ID {
		t.Fatalf("PATCH answered %+v, want %s shared and still tenant B's", got, f.hostB)
	}

	listed := f.do(http.MethodGet, "/api/v1/tenants/other/domains", f.hostA, token, nil)
	mustStatus(t, listed, http.StatusOK)
	var page wire.Page[core.Domain]
	decodeBody(t, listed, &page)
	if len(page.Items) != 1 || !page.Items[0].Shared {
		t.Fatalf("tenant B's domains = %+v, want its one domain shared", page.Items)
	}

	missing := f.do(http.MethodPatch, "/api/v1/domains/nosuch.test", f.hostA, token, map[string]any{"shared": true})
	mustStatus(t, missing, http.StatusNotFound)
}
