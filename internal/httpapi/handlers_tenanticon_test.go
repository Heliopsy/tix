// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func tenantIconPath(ref string) string { return strings.Replace(wire.RouteTenantIcon, "{ref}", ref, 1) }

func TestTenantIconEndpoint(t *testing.T) {
	f := newFixture(t)
	tenantPath := strings.Replace(wire.RouteTenant, "{ref}", wire.TenantSelf, 1)

	resp := f.call(http.MethodGet, tenantIconPath("acme"), nil)
	mustStatus(t, resp, http.StatusNotFound)
	if body := readBody(t, resp); !strings.Contains(body, `"code"`) {
		t.Fatalf("a tenant without an image answered %q, not the error envelope", body)
	}

	png := []byte("\x89PNG\r\n\x1a\ntenant over the wire")
	resp = f.call(http.MethodPatch, tenantPath, core.UpdateTenantInput{
		IconFile: &core.ProjectIconFile{ContentType: "image/png", Data: png}})
	mustStatus(t, resp, http.StatusOK)
	var tenant core.Tenant
	decodeBody(t, resp, &tenant)
	if tenant.IconImage != core.ProjectIconDigest(png) {
		t.Fatalf("patched tenant carries icon_image %q", tenant.IconImage)
	}

	for _, ref := range []string{"acme", wire.TenantSelf, f.tenantA.ID} {
		resp = f.call(http.MethodGet, tenantIconPath(ref), nil)
		mustStatus(t, resp, http.StatusOK)
		h := resp.Header
		if body := readBody(t, resp); body != string(png) {
			t.Fatalf("%s: icon body = %q", ref, body)
		}
		want := map[string]string{
			"Content-Type":            "image/png",
			"X-Content-Type-Options":  "nosniff",
			"Content-Security-Policy": wire.IconPolicy,
			"ETag":                    `"` + tenant.IconImage + `"`,
		}
		for name, value := range want {
			if got := h.Get(name); got != value {
				t.Errorf("%s: %s = %q, want %q", ref, name, got, value)
			}
		}
	}

	req, err := http.NewRequest(http.MethodGet, f.server.URL+tenantIconPath("acme"), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	req.Header.Set("If-None-Match", `"`+tenant.IconImage+`"`)
	cached, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(cached.Body)
	_ = cached.Body.Close()
	if cached.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("revalidation answered %d with %d bytes", cached.StatusCode, len(body))
	}

	resp = f.do(http.MethodGet, tenantIconPath("acme"), f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPatch, tenantPath, map[string]any{"icon_name": "rocket"})
	mustStatus(t, resp, http.StatusOK)
	var named core.Tenant
	decodeBody(t, resp, &named)
	if named.IconName != "rocket" || named.IconImage != "" {
		t.Fatalf("after a built-in icon: %+v", named)
	}
	resp = f.call(http.MethodGet, tenantIconPath("acme"), nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPatch, tenantPath, map[string]any{"icon": "A", "icon_name": "bug"})
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}
