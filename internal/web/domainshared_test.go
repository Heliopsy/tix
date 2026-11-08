// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// mapDomain maps a hostname to a tenant straight in the store.
func (f *fixture) mapDomain(tenantID, host string) {
	f.t.Helper()
	ctx := context.Background()
	if err := f.store.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		return tx.AddDomain(ctx, &core.Domain{Hostname: host})
	}); err != nil {
		f.t.Fatalf("mapping %s: %v", host, err)
	}
}

// domainRow is the row of the global admin's Domains section naming host.
func domainRow(t *testing.T, page, host string) string {
	t.Helper()
	return between(t, page, `<tr id="domain-`+host+`">`, "</tr>")
}

func TestTheGlobalAdminSharesAndUnsharesATenantsDomain(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.mapDomain(f.tenantB.ID, "other.example.com")
	f.makeGlobalAdmin()
	b := f.as("alice")

	row := domainRow(t, b.page("/admin/global/tenants/other"), "other.example.com")
	for _, want := range []string{"this tenant only", `action="/admin/global/domains"`, `name="shared" value="1"`,
		`name="target" value="other"`, `name="hostname" value="other.example.com"`} {
		if !strings.Contains(row, want) {
			t.Errorf("the unshared domain's row lacks %s:\n%s", want, row)
		}
	}

	shared := b.post("/admin/global/domains", url.Values{"target": {"other"}, "hostname": {"other.example.com"}, "shared": {"1"}})
	wantStatus(t, shared, http.StatusSeeOther)
	if loc := shared.Header.Get("Location"); !strings.HasPrefix(loc, "/admin/global/tenants/other?") || !strings.HasSuffix(loc, "#domains") {
		t.Errorf("sharing went to %q, want back to the tenant's Domains section", loc)
	}
	_ = shared.Body.Close()
	row = domainRow(t, b.page("/admin/global/tenants/other"), "other.example.com")
	if !strings.Contains(row, `class="state on"`) || !strings.Contains(row, `name="shared" value="0"`) {
		t.Errorf("after sharing, the row is not marked shared with a way to stop:\n%s", row)
	}

	unshared := b.post("/admin/global/domains", url.Values{"target": {"other"}, "hostname": {"other.example.com"}, "shared": {"0"}})
	wantStatus(t, unshared, http.StatusSeeOther)
	_ = unshared.Body.Close()
	if row = domainRow(t, b.page("/admin/global/tenants/other"), "other.example.com"); !strings.Contains(row, `name="shared" value="1"`) {
		t.Errorf("after unsharing, the row offers no way to share again:\n%s", row)
	}
}

func TestATenantAdminSeesButCannotSetSharedInTheBrowser(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	f.mapDomain(f.tenantA.ID, "acme.example.com")
	b := f.as("alice")

	page := b.page("/admin/domains")
	if !strings.Contains(page, "this tenant only") {
		t.Errorf("the tenant's own Domains screen does not say the domain is not shared")
	}
	if strings.Contains(page, `action="/admin/global/domains"`) {
		t.Errorf("the tenant's own Domains screen offers the global admin's share form")
	}
	resp := b.post("/admin/global/domains", url.Values{"target": {"acme"}, "hostname": {"acme.example.com"}, "shared": {"1"}})
	if resp.StatusCode == http.StatusSeeOther {
		t.Errorf("a tenant admin sharing their own domain succeeded")
	}
	_ = resp.Body.Close()
	if !strings.Contains(b.page("/admin/domains"), "this tenant only") {
		t.Errorf("the refused attempt changed the domain")
	}
}
