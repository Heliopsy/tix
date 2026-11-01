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

func iconPath(ref string) string { return strings.Replace(wire.RouteProjectIcon, "{ref}", ref, 1) }

func TestProjectIconEndpoint(t *testing.T) {
	f := newFixture(t)
	projectPath := strings.Replace(wire.RouteProject, "{ref}", "infra", 1)

	resp := f.call(http.MethodGet, iconPath("infra"), nil)
	mustStatus(t, resp, http.StatusNotFound)
	if body := readBody(t, resp); !strings.Contains(body, `"code"`) {
		t.Fatalf("a project without an image answered %q, not the error envelope", body)
	}

	png := []byte("\x89PNG\r\n\x1a\nover the wire")
	resp = f.call(http.MethodPatch, projectPath, core.UpdateProjectInput{
		IconFile: &core.ProjectIconFile{ContentType: "image/png", Data: png}})
	mustStatus(t, resp, http.StatusOK)
	var p core.Project
	decodeBody(t, resp, &p)
	if p.IconImage != core.ProjectIconDigest(png) {
		t.Fatalf("patched project carries icon_image %q", p.IconImage)
	}

	resp = f.call(http.MethodGet, iconPath("infra"), nil)
	mustStatus(t, resp, http.StatusOK)
	h := resp.Header
	if body := readBody(t, resp); body != string(png) {
		t.Fatalf("icon body = %q", body)
	}
	want := map[string]string{
		"Content-Type":            "image/png",
		"X-Content-Type-Options":  "nosniff",
		"Content-Security-Policy": wire.IconPolicy,
		"ETag":                    `"` + p.IconImage + `"`,
	}
	for name, value := range want {
		if got := h.Get(name); got != value {
			t.Errorf("%s = %q, want %q", name, got, value)
		}
	}
	if !strings.Contains(wire.IconPolicy, "sandbox") || !strings.Contains(wire.IconPolicy, "default-src 'none'") {
		t.Errorf("icon policy %q does not forbid script", wire.IconPolicy)
	}

	req, err := http.NewRequest(http.MethodGet, f.server.URL+iconPath("infra"), nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	req.Header.Set("If-None-Match", h.Get("ETag"))
	cached, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(cached.Body)
	_ = cached.Body.Close()
	if cached.StatusCode != http.StatusNotModified || len(body) != 0 {
		t.Fatalf("revalidation answered %d with %d bytes", cached.StatusCode, len(body))
	}

	resp = f.do(http.MethodGet, iconPath("infra"), f.hostB, f.tokenB, nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPatch, projectPath, map[string]any{"icon_name": "rocket"})
	mustStatus(t, resp, http.StatusOK)
	var named core.Project
	decodeBody(t, resp, &named)
	if named.IconName != "rocket" || named.IconImage != "" {
		t.Fatalf("after a built-in icon: %+v", named)
	}
	resp = f.call(http.MethodGet, iconPath("infra"), nil)
	mustStatus(t, resp, http.StatusNotFound)
	_ = resp.Body.Close()

	resp = f.call(http.MethodPatch, projectPath, map[string]any{"icon_file": map[string]any{"data": "PHN2Zz48c2NyaXB0Lz48L3N2Zz4="}})
	mustStatus(t, resp, http.StatusBadRequest)
	_ = resp.Body.Close()
}
