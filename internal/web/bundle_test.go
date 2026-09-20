package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/web"
)

// minimalBundle is a bundle body carrying nothing but its header, so a
// validation test need not depend on an export first.
const minimalBundle = `{"record":"header","header":{"name":"starter","version":1}}` + "\n"

// downloadBundle submits the export form and returns the downloaded response.
func (b *browser) downloadBundle(form url.Values) *http.Response {
	b.t.Helper()
	return b.post(web.RouteBundleExport, form)
}

// uploadBundle submits the import form with the bundle as an uploaded file.
func (b *browser) uploadBundle(fields map[string]string, content string) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields["csrf_token"] = b.csrf()
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			b.t.Fatalf("writing field %q: %v", name, err)
		}
	}
	part, err := writer.CreateFormFile("bundle", "tix-bundle.ndjson")
	if err != nil {
		b.t.Fatalf("creating file part: %v", err)
	}
	if _, err := io.WriteString(part, content); err != nil {
		b.t.Fatalf("writing file part: %v", err)
	}
	if err := writer.Close(); err != nil {
		b.t.Fatalf("closing multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+web.RouteBundleImport, &body)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("posting bundle: %v", err)
	}
	return resp
}

// exportedBundle downloads a bundle carrying the named components.
func exportedBundle(t *testing.T, b *browser, name string) string {
	t.Helper()
	resp := b.downloadBundle(url.Values{"name": {name}, "kinds": {string(core.ComponentWorkflow)}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusOK)
	return readAll(t, resp)
}

func TestSharingScreenOffersEveryKindAndPolicy(t *testing.T) {
	f := newFixture(t)
	page := f.as("alice").page(web.RouteBundles)

	for _, kind := range core.ComponentKinds {
		if !strings.Contains(page, string(kind)) {
			t.Errorf("sharing screen does not offer the %q kind", kind)
		}
	}
	for _, policy := range []core.CollisionPolicy{core.CollisionSkip, core.CollisionRename, core.CollisionReplace} {
		if !strings.Contains(page, string(policy)) {
			t.Errorf("sharing screen does not offer the %q policy", policy)
		}
	}
	if !strings.Contains(page, `name="csrf_token"`) {
		t.Error("sharing screen forms carry no csrf token")
	}
	if !strings.Contains(page, `method="post"`) {
		t.Error("sharing screen has no plain form submission")
	}
}

func TestBundleDownloadsAsAFile(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").downloadBundle(url.Values{
		"name": {"starter kit"}, "kinds": {string(core.ComponentWorkflow)}})
	defer func() { _ = resp.Body.Close() }()
	wantStatus(t, resp, http.StatusOK)

	disposition := resp.Header.Get("Content-Disposition")
	if !strings.HasPrefix(disposition, "attachment;") || !strings.Contains(disposition, "filename=") {
		t.Errorf("content disposition = %q, want an attachment with a filename", disposition)
	}
	if got := readAll(t, resp); strings.TrimSpace(got) == "" {
		t.Error("the downloaded bundle is empty")
	}
}

func TestDownloadedBundleCanBeUploadedBack(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	bundle := exportedBundle(t, b, "starter")

	resp := b.uploadBundle(map[string]string{"on_collision": string(core.CollisionRename)}, bundle)
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(page, string(core.ActionRenamed)) && !strings.Contains(page, string(core.ActionCreated)) {
		t.Errorf("import result reports no action: %s", page)
	}
}

func TestBundleImportWithoutACollisionPolicyIsRefused(t *testing.T) {
	f := newFixture(t)

	resp := f.as("alice").uploadBundle(map[string]string{"on_collision": ""}, minimalBundle)
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, page)
	}
	if !strings.Contains(page, "collision policy") {
		t.Errorf("page does not name the collision policy: %s", page)
	}
}

func TestBundleImportWithoutAFileOrPasteIsRefused(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").post(web.RouteBundleImport,
		url.Values{"on_collision": {string(core.CollisionSkip)}})
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, page)
	}
}

func TestBundlePreviewShowsThePlanAndWritesNothing(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	bundle := exportedBundle(t, b, "starter")
	before := b.page(web.RouteWorkflows)

	resp := b.uploadBundle(map[string]string{
		"on_collision": string(core.CollisionRename), "preview": "1"}, bundle)
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(page, "nothing was written") {
		t.Errorf("preview does not say nothing was written: %s", page)
	}
	if !strings.Contains(page, string(core.ComponentWorkflow)) {
		t.Errorf("preview does not name the component kind: %s", page)
	}
	if after := b.page(web.RouteWorkflows); after != before {
		t.Error("a preview changed the workflow list")
	}
}

func TestBundleNameContainingMarkupIsEscaped(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	bundle := exportedBundle(t, b, `<script>alert(1)</script>`)

	resp := b.uploadBundle(map[string]string{
		"on_collision": string(core.CollisionSkip), "preview": "1"}, bundle)
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if strings.Contains(page, "<script>alert(1)</script>") {
		t.Errorf("a bundle name was rendered as markup: %s", page)
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Errorf("the escaped bundle name is missing: %s", page)
	}
}

func TestBundleFormsRefuseATenantOverride(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")

	cases := []struct {
		name string
		path string
		form url.Values
	}{
		{"export form", web.RouteBundleExport,
			url.Values{"tenant": {"other"}, "kinds": {string(core.ComponentWorkflow)}}},
		{"export query", web.RouteBundleExport + "?tenant_key=other", url.Values{}},
		{"import form", web.RouteBundleImport,
			url.Values{"tenant_id": {"other"}, "on_collision": {string(core.CollisionSkip)}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			resp := b.post(tc.path, tc.form)
			page := body(t, resp)
			if resp.StatusCode != http.StatusForbidden {
				t.Fatalf("status = %d, want 403: %s", resp.StatusCode, page)
			}
		})
	}
}

func TestBundleRoutesRequireASignedInActor(t *testing.T) {
	f := newFixture(t)
	anon := f.as("")
	for _, path := range []string{web.RouteBundles, web.RouteBundleExport, web.RouteBundleImport} {
		resp := anon.get(path)
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther && resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("GET %s as an anonymous browser = %d", path, resp.StatusCode)
		}
	}
}

func TestBundleExportRefusesAnUnknownKind(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").downloadBundle(url.Values{"kinds": {"saved_filter"}})
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, page)
	}
}

func TestBundlePasteIsAcceptedWithoutAFilePicker(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	bundle := exportedBundle(t, b, "starter")

	resp := b.post(web.RouteBundleImport, url.Values{
		"bundle_text":  {bundle},
		"on_collision": {string(core.CollisionSkip)},
		"preview":      {"1"},
	})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(page, "nothing was written") {
		t.Errorf("pasted preview did not render: %s", page)
	}
}

func TestUnnamedBundleDownloadsUnderADefaultName(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").downloadBundle(url.Values{})
	_ = body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if got := resp.Header.Get("Content-Disposition"); got != `attachment; filename="tix-bundle.ndjson"` {
		t.Errorf("content disposition = %q, want the default bundle filename", got)
	}
}

func TestBundleFilenameDropsPunctuationFromTheName(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").downloadBundle(url.Values{"name": {`Ops "kit"; rm -rf /`}})
	_ = body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	got := resp.Header.Get("Content-Disposition")
	if strings.ContainsAny(got, ";/") == false || !strings.HasSuffix(got, `.ndjson"`) {
		t.Errorf("content disposition = %q, want a bundle filename", got)
	}
	if strings.Contains(got, `rm -rf`) || strings.Contains(got, `"kit"`) {
		t.Errorf("content disposition = %q, still carries the raw bundle name", got)
	}
}

func TestBundleImportRefusesAnUnknownCollisionPolicy(t *testing.T) {
	f := newFixture(t)
	resp := f.as("alice").uploadBundle(map[string]string{"on_collision": "clobber"}, minimalBundle)
	page := body(t, resp)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", resp.StatusCode, page)
	}
}

func TestBundleImportTargetsANamedProject(t *testing.T) {
	f := newFixture(t)
	b := f.as("alice")
	resp := b.post(web.RouteBundleImport, url.Values{
		"bundle_text":  {minimalBundle},
		"on_collision": {string(core.CollisionSkip)},
		"project_ref":  {"infra"},
		"preview":      {"1"},
	})
	page := body(t, resp)
	wantStatus(t, resp, http.StatusOK)
	if !strings.Contains(page, "nothing was written") {
		t.Errorf("preview did not render: %s", page)
	}
	if !strings.Contains(page, "carries no components") {
		t.Errorf("an empty bundle did not say so: %s", page)
	}
}
