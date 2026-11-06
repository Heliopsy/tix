// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"
)

const webMapping = `version: 1
project: infra
identity:
  id: id
fields:
  subject: title
status_field: status
statuses:
  New: todo
  Closed: done
default_status: todo
`

// webSecret is long enough to earn a hint, so a page showing the hint can be
// told apart from a page leaking the value.
const webSecret = "web-secret-api-key-0123456789-qrst"

// postSourceForm submits the source dialog as a browser does, multipart, with
// an uploaded mapping file when content is not empty.
func postSourceForm(b *browser, fields map[string]string, content string) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields["csrf_token"] = b.csrf()
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			b.t.Fatalf("writing field %q: %v", name, err)
		}
	}
	if content != "" {
		part, err := writer.CreateFormFile("mapping_file", "op.yaml")
		if err != nil {
			b.t.Fatalf("creating file part: %v", err)
		}
		if _, err := io.WriteString(part, content); err != nil {
			b.t.Fatalf("writing file part: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		b.t.Fatalf("closing multipart writer: %v", err)
	}
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+"/sync/sources", &body)
	if err != nil {
		b.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	b.stamp(req)
	resp, err := b.client.Do(req)
	if err != nil {
		b.t.Fatalf("posting the source dialog: %v", err)
	}
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(raw)
}

// previewBlock is the dialog's check result alone, so an assertion about it
// is not satisfied by the help text or the sources table.
func previewBlock(t *testing.T, page string) string {
	t.Helper()
	start := strings.Index(page, `class="syncpreview"`)
	if start < 0 {
		t.Fatalf("the page carries no check result")
	}
	block, _, _ := strings.Cut(page[start:], "</section>")
	return block
}

// sourceRow is the Sources table row of one source.
func sourceRow(t *testing.T, page, name string) string {
	t.Helper()
	table := section(t, page, "Sources")
	start := strings.Index(table, "<td>"+name+"</td>")
	if start < 0 {
		t.Fatalf("the Sources table has no row for %s", name)
	}
	row := table[start:]
	if end := strings.Index(row, "</tr>"); end >= 0 {
		row = row[:end]
	}
	return row
}

func TestSyncDialogChecksAnUploadedMappingWithoutSaving(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	b.page("/sync")
	resp := postSourceForm(b, map[string]string{"system": "openproject", "name": "op",
		"url": "https://op.example", "token": webSecret, "intent": "check"}, webMapping)
	wantStatus(t, resp, http.StatusOK)
	page := readBody(t, resp)

	preview := previewBlock(t, page)
	for _, want := range []string{"<code>infra</code>", "<code>subject -&gt; title</code>",
		"<code>Closed -&gt; done</code>", "stored on the source", "nothing, a run can start"} {
		if !strings.Contains(preview, want) {
			t.Errorf("the check result does not show %q:\n%s", want, preview)
		}
	}
	if !strings.Contains(page, "<textarea id=\"mapping\"") || !strings.Contains(page, "statuses:\n  New: todo") {
		t.Errorf("the dialog does not hold the uploaded mapping after the check")
	}
	if strings.Contains(page, webSecret) {
		t.Errorf("the check result echoes the credential")
	}
	if strings.Contains(section(t, b.page("/sync"), "Sources"), "<td>op</td>") {
		t.Errorf("a check saved the source")
	}
}

func TestSyncDialogRefusesAMappingIntoAMissingProject(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	b.page("/sync")
	resp := postSourceForm(b, map[string]string{"system": "openproject", "name": "op", "intent": "check"},
		strings.Replace(webMapping, "project: infra", "project: nowhere", 1))
	page := readBody(t, resp)
	if resp.StatusCode != http.StatusUnprocessableEntity && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want a refusal", resp.StatusCode)
	}
	if !strings.Contains(page, "nowhere") || !strings.Contains(page, `aria-invalid="true"`) {
		t.Errorf("the refusal does not name the project beside the mapping")
	}
}

func TestSyncDialogStoresTheMappingAndCredentialsWriteOnly(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	b.page("/sync")
	resp := postSourceForm(b, map[string]string{"system": "openproject", "name": "op",
		"url": "https://op.example", "user": "apikey", "password": webSecret}, webMapping)
	_ = resp.Body.Close()
	wantStatus(t, resp, http.StatusSeeOther)

	page := b.page("/sync")
	row := sourceRow(t, page, "op")
	for _, want := range []string{"stored on the source", "password <code>…qrst</code>"} {
		if !strings.Contains(row, want) {
			t.Errorf("the source row does not say %q:\n%s", want, row)
		}
	}
	if strings.Contains(row, "Missing on the server") {
		t.Errorf("a source with everything stored is reported as missing something:\n%s", row)
	}
	edit := b.page("/sync?dialog=source-new&source=" + sourceID(t, row))
	for name, got := range map[string]string{"list": page, "set up dialog": edit} {
		if strings.Contains(got, webSecret) {
			t.Errorf("the %s shows the stored credential", name)
		}
	}
	if !strings.Contains(edit, "statuses:\n  New: todo") || !strings.Contains(edit, `value="https://op.example"`) {
		t.Errorf("the Set up dialog is not filled in with the stored mapping and url")
	}
}

func TestSyncScreenSaysWhatTheServerIsMissing(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	addSource(t, b, "openproject", "bare")
	row := sourceRow(t, b.page("/sync"), "bare")
	block := row[max(0, strings.Index(row, `class="syncmissing"`)):]
	for _, want := range []string{"Missing on the server", "a mapping document", "credentials",
		"TIX_SYNC_BARE_TOKEN", "the OpenProject url"} {
		if !strings.Contains(block, want) {
			t.Errorf("the missing list does not name %q:\n%s", want, block)
		}
	}
}

// sourceID reads a source's identifier out of its row's Run form.
func sourceID(t *testing.T, row string) string {
	t.Helper()
	const marker = `name="source_id" value="`
	start := strings.Index(row, marker)
	if start < 0 {
		t.Fatalf("the row has no run form")
	}
	id, _, _ := strings.Cut(row[start+len(marker):], `"`)
	return id
}
