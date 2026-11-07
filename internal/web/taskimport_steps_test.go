// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"testing"
)

// uploadImportParts starts an import with any file parts, keyed by part name.
func (b *browser) uploadImportParts(fields map[string]string, parts map[string][2]string) *http.Response {
	b.t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fields["csrf_token"] = b.csrf()
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	for name, file := range parts {
		part, err := w.CreateFormFile(name, file[0])
		if err != nil {
			b.t.Fatal(err)
		}
		_, _ = io.WriteString(part, file[1])
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+"/projects/infra/import", &buf)
	if err != nil {
		b.t.Fatal(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatal(err)
	}
	return resp
}

// The upload step comes back with the reason when there is nothing to read
// or the mapping file is not a mapping, and keeps the reader on the page.
func TestARefusedUploadIsDrawnAgainWithTheReason(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	empty := b.uploadImport(map[string]string{"go": "next"}, "", "")
	page := body(t, empty)
	if empty.StatusCode != http.StatusBadRequest || !strings.Contains(page, "choose a file, or paste a list of tasks") {
		t.Errorf("an empty upload answered %d:\n%s", empty.StatusCode, page)
	}

	bad := b.uploadImportParts(map[string]string{"go": "next"}, map[string][2]string{
		"file": {"t.csv", importCSV}, "mapping_file": {"map.yaml", "columns: [this is: not, a map"}})
	page = body(t, bad)
	if bad.StatusCode != http.StatusBadRequest || !strings.Contains(page, "reading the mapping") {
		t.Errorf("a broken mapping file answered %d:\n%s", bad.StatusCode, page)
	}
}

// A mapping file sent with the upload is the one in force, and the mapping
// step offers it back as a YAML download.
func TestTheMappingInForceDownloadsAsYAML(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	preview := followTo(t, b.uploadImportParts(map[string]string{"go": "preview"}, map[string][2]string{
		"file": {"t.csv", "Name\nFrom a mapping\n"}, "mapping_file": {"map.yaml", "columns:\n  Name: title\n"}}), "preview")
	page := b.page(preview)
	if !strings.Contains(page, "From a mapping") {
		t.Errorf("the uploaded mapping was not applied:\n%s", page)
	}
	resp := b.get(strings.TrimSuffix(preview, "/preview") + "/mapping.yaml")
	got := body(t, resp)
	if resp.Header.Get("Content-Type") != "application/yaml; charset=utf-8" ||
		resp.Header.Get("Content-Disposition") != `attachment; filename="infra-mapping.yaml"` {
		t.Errorf("headers = %v", resp.Header)
	}
	if !regexp.MustCompile(`Name:\s+title`).MatchString(got) {
		t.Errorf("the mapping download does not map Name to title:\n%s", got)
	}
}

// Cancel forgets the staged file; a step that does not exist is not found;
// the result step before an import goes back to the preview.
func TestImportStepsCancelAndRefuseUnknownSteps(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	columns := followTo(t, b.uploadImport(map[string]string{"go": "next"}, "t.csv", importCSV), "columns")
	base := strings.TrimSuffix(columns, "/columns")
	id := base[strings.LastIndex(base, "/")+1:]

	early := b.get(base + "/result")
	_ = early.Body.Close()
	if early.StatusCode != http.StatusSeeOther || !strings.HasSuffix(early.Header.Get("Location"), "/preview") {
		t.Errorf("the result before an import answered %d to %q", early.StatusCode, early.Header.Get("Location"))
	}
	unknown := b.get(base + "/sideways")
	_ = unknown.Body.Close()
	wantStatus(t, unknown, http.StatusNotFound)

	cancel := b.uploadImport(map[string]string{"go": "cancel", "id": id}, "", "")
	_ = cancel.Body.Close()
	if cancel.StatusCode != http.StatusSeeOther || cancel.Header.Get("Location") != "/projects/infra" {
		t.Errorf("cancel answered %d to %q", cancel.StatusCode, cancel.Header.Get("Location"))
	}
	gone := b.get(columns)
	_ = gone.Body.Close()
	wantStatus(t, gone, http.StatusNotFound)
}

// An export in display dates writes the reader's own format, never a
// relative one, which could not be read back.
func TestADisplayExportUsesTheReadersFormat(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	b.createTask("infra", "dated")
	resp := b.post("/projects/infra/tasks/export", url.Values{
		"format": {"csv"}, "column": {"title", "created_at"}, "finished": {"1"}, "subtasks": {"1"},
		"dates": {"display"}, "delimiter": {","},
	})
	got := body(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, got)
	}
	if !regexp.MustCompile(`dated,\d{4}-\d{2}-\d{2} \d{2}:\d{2}`).MatchString(got) {
		t.Errorf("a display export did not write the reader's iso style:\n%s", got)
	}
}
