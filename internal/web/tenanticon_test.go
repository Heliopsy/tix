// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/web"
)

var tenantPNG = []byte("\x89PNG\r\n\x1a\ntenant in the browser")

// setTenantIcon sets a tenant's icon as its own system actor would.
func (w *switchWorld) setTenantIcon(tenantID string, in core.UpdateTenantInput) *core.Tenant {
	w.t.Helper()
	shell := core.WithDirectStore(core.WithActor(context.Background(), core.SystemActor(tenantID)))
	out, err := w.local.UpdateTenant(shell, "", in)
	if err != nil {
		w.t.Fatalf("setting the icon of %s: %v", tenantID, err)
	}
	return out
}

// tenantMarkOf is the mark drawn on the switcher row naming key.
func tenantMarkOf(t *testing.T, page, key string) string {
	t.Helper()
	mark := between(t, switcherRowOf(t, page, key), `<span class="tenantswitch-mark tenant-mark" data-tenant-mark>`, "</span>\n")
	if mark == "" {
		t.Fatalf("the row for %s draws no tenant mark", key)
	}
	return mark
}

func TestTheSwitcherDrawsEachTenantsIconOrMonogram(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")

	_, page := pat.do(http.MethodGet, web.RouteProjects, nil)
	brand := between(t, page, `<a class="brand"`, "</a>")
	if !strings.Contains(brand, `src="/assets/tix.svg"`) || strings.Contains(brand, "tenant-logo") {
		t.Errorf("a tenant with no icon has the brand %q, want the logo", brand)
	}

	test := w.setTenantIcon(w.test.ID, core.UpdateTenantInput{IconFile: &core.ProjectIconFile{Data: tenantPNG}})
	w.setTenantIcon(w.home.ID, core.UpdateTenantInput{IconName: new("rocket")})

	_, page = pat.do(http.MethodGet, web.RouteProjects, nil)
	if mark := tenantMarkOf(t, page, "test"); !strings.Contains(mark, `<img class="proj-icon proj-icon-img" src="/tenants/test/icon?v=`+test.IconImage+`"`) {
		t.Errorf("test's mark = %s, want its uploaded image", mark)
	}
	if mark := tenantMarkOf(t, page, "home"); !strings.Contains(mark, `#pi-rocket`) {
		t.Errorf("home's mark = %s, want the rocket", mark)
	}
	if mark := tenantMarkOf(t, page, "away"); !strings.Contains(mark, `>A</span>`) {
		t.Errorf("away's mark = %s, want the monogram A", mark)
	}
	open := between(t, page, `class="nav-tenant"`, "</a>")
	if !strings.Contains(open, "data-tenant-mark") || !strings.Contains(open, "#pi-rocket") {
		t.Errorf("the sidebar's tenant entry = %q, want home's rocket", open)
	}
	brand = between(t, page, `<a class="brand"`, "</a>")
	if !strings.Contains(brand, "tenant-logo") || !strings.Contains(brand, "#pi-rocket") || strings.Contains(brand, "tix.svg") {
		t.Errorf("the brand = %q, want home's rocket in place of the logo", brand)
	}

	resp, body := pat.do(http.MethodGet, "/tenants/test/icon?v="+test.IconImage, nil)
	if resp.StatusCode != http.StatusOK || body != string(tenantPNG) || resp.Header.Get("Content-Type") != "image/png" ||
		resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("a member reading another tenant's icon = %d %q %v", resp.StatusCode, resp.Header.Get("Content-Type"), resp.Header)
	}
	solo := w.visitor()
	solo.signIn("solo@example.test")
	if resp, _ := solo.do(http.MethodGet, "/tenants/test/icon", nil); resp.StatusCode != http.StatusNotFound {
		t.Errorf("somebody outside test reading its icon = %d, want 404", resp.StatusCode)
	}
}

// postMultipart posts a multipart form, with an optional file under
// icon_file, as the tenant settings form does.
func (v *visitor) postMultipart(path string, form url.Values, file []byte) (*http.Response, string) {
	v.w.t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	form.Set("csrf_token", v.cookie(web.CSRFCookieName))
	for k, vals := range form {
		for _, val := range vals {
			_ = mw.WriteField(k, val)
		}
	}
	if file != nil {
		part, _ := mw.CreateFormFile("icon_file", "logo.png")
		_, _ = part.Write(file)
	}
	_ = mw.Close()
	req, err := http.NewRequest(http.MethodPost, v.w.srv.URL+path, &buf)
	if err != nil {
		v.w.t.Fatal(err)
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())
	resp, err := v.client.Do(req)
	if err != nil {
		v.w.t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	return resp, string(raw)
}

func TestTheTenantSettingsSetTheTenantsIcon(t *testing.T) {
	w := newSwitchWorld(t)
	pat := w.visitor()
	pat.signIn("pat@example.test")
	shell := core.WithDirectStore(core.WithActor(context.Background(), core.SystemActor(w.home.ID)))
	current := func() *core.Tenant {
		tenant, err := w.local.GetTenant(shell, "")
		if err != nil {
			t.Fatal(err)
		}
		return tenant
	}

	_, page := pat.do(http.MethodGet, web.RouteTenant, nil)
	form := between(t, page, `<form method="post" action="/admin/tenant"`, "</form>")
	if !strings.HasPrefix(form, `enctype="multipart/form-data">`) || !strings.Contains(form, `data-iconpick`) ||
		!strings.Contains(form, `name="icon_kind" value="none" checked`) || !strings.Contains(form, `data-iconpick-fallback="H"`) {
		t.Fatalf("the tenant form = %q, want a multipart form with the icon picker on No icon", form)
	}

	base := url.Values{"name": {"Home"}, "theme": {""}}
	steps := []struct {
		name string
		form url.Values
		file []byte
		want func(*core.Tenant) bool
	}{
		{"a built-in icon", url.Values{"icon_kind": {"builtin"}, "icon_name": {"bug"}}, nil,
			func(t *core.Tenant) bool { return t.IconName == "bug" }},
		{"no icon kind leaves it", url.Values{}, nil,
			func(t *core.Tenant) bool { return t.IconName == "bug" }},
		{"an upload", url.Values{"icon_kind": {"image"}}, tenantPNG,
			func(t *core.Tenant) bool { return t.IconImage == core.ProjectIconDigest(tenantPNG) && t.IconName == "" }},
		{"upload with no file keeps the image", url.Values{"icon_kind": {"image"}}, nil,
			func(t *core.Tenant) bool { return t.IconImage == core.ProjectIconDigest(tenantPNG) }},
		{"an emoji", url.Values{"icon_kind": {"text"}, "icon": {"H"}}, nil,
			func(t *core.Tenant) bool { return t.Icon == "H" && t.IconImage == "" }},
		{"no icon", url.Values{"icon_kind": {"none"}}, nil,
			func(t *core.Tenant) bool { return t.IconKind() == core.IconKindNone }},
	}
	for _, s := range steps {
		form := url.Values{}
		for k, v := range base {
			form[k] = v
		}
		for k, v := range s.form {
			form[k] = v
		}
		resp, page := pat.postMultipart(web.RouteTenant, form, s.file)
		if resp.StatusCode != http.StatusSeeOther {
			t.Fatalf("%s: saving = %d\n%s", s.name, resp.StatusCode, page)
		}
		if got := current(); !s.want(got) {
			t.Errorf("%s: tenant is %+v", s.name, got)
		}
	}

	w.setTenantIcon(w.home.ID, core.UpdateTenantInput{IconName: new("rocket")})
	_, page = pat.do(http.MethodGet, web.RouteTenant, nil)
	if form := between(t, page, `<form method="post" action="/admin/tenant"`, "</form>"); !strings.Contains(form, `name="icon_kind" value="builtin" checked`) ||
		!strings.Contains(form, `name="icon_name" value="rocket" checked`) {
		t.Errorf("the picker does not open on the tenant's rocket:\n%s", form)
	}
}
