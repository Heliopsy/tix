// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/web"
)

// uploadImport starts an import of the infra project with a file or a list.
func (b *browser) uploadImport(fields map[string]string, filename, content string) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fields["csrf_token"] = b.csrf()
	for k, v := range fields {
		_ = w.WriteField(k, v)
	}
	if filename != "" {
		part, err := w.CreateFormFile("file", filename)
		if err != nil {
			b.t.Fatal(err)
		}
		_, _ = io.WriteString(part, content)
	}
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+"/projects/infra/import", &body)
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

// followTo reads a redirect's target and checks it is the step named.
func followTo(t *testing.T, resp *http.Response, step string) string {
	t.Helper()
	_ = resp.Body.Close()
	loc := resp.Header.Get("Location")
	if resp.StatusCode != http.StatusSeeOther || !strings.HasSuffix(strings.SplitN(loc, "?", 2)[0], "/"+step) &&
		(step != "import" || !strings.Contains(loc, "/import?id=")) {
		t.Fatalf("status %d to %q, want a redirect to the %s step", resp.StatusCode, loc, step)
	}
	return loc
}

const importCSV = "Summary,Status,Priority,Labels,Story\nFirst,In Progress,High,ops,notes one\nSecond,Done,Low,,\n"

// importFoot is the foot of a step's form, where Back and Cancel live.
func importFoot(t *testing.T, page string) string {
	t.Helper()
	return between(t, page, `class="cform-foot importfoot"`, "</form>")
}

func TestEveryImportStepHasBackAndCancel(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	pages := map[string]string{"upload": b.page("/projects/infra/import")}
	columns := followTo(t, b.uploadImport(map[string]string{"go": "next"}, "tasks.csv", importCSV), "columns")
	base := strings.TrimSuffix(columns, "/columns")
	pages["columns"] = b.page(columns)
	pages["values"] = b.page(base + "/values")
	pages["preview"] = b.page(base + "/preview")
	followTo(t, b.post(base+"/preview", url.Values{"go": {"import"}}), "result")
	pages["result"] = b.page(base + "/result")
	for step, page := range pages {
		foot := importFoot(t, page)
		if !strings.Contains(foot, "data-import-back") {
			t.Errorf("the %s step has no Back:\n%s", step, foot)
		}
		cancel := between(t, foot, `<a class="btn secondary" href="/projects/infra" data-form-cancel`, "</a>")
		if !strings.Contains(cancel, "data-import-cancel") {
			t.Errorf("the %s step has no Cancel back to the project that Escape follows:\n%s", step, foot)
		}
		if strings.Contains(between(t, page, `<nav aria-label="Import steps">`, `class="cform-foot importfoot"`), "<details") {
			t.Errorf("the %s step hides something in an expander", step)
		}
		if !strings.Contains(page, `aria-current="step"`) {
			t.Errorf("the %s step does not say which step it is", step)
		}
	}
}

func TestImportBackKeepsWhatWasChosen(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	columns := followTo(t, b.uploadImport(map[string]string{"go": "next"}, "tasks.csv", importCSV), "columns")
	base := strings.TrimSuffix(columns, "/columns")
	page := b.page(columns)
	if !strings.Contains(page, `<option value="title" selected>`) || !strings.Contains(page, `<option value="status" selected>`) {
		t.Fatalf("the suggestion did not map Summary and Status:\n%s", page)
	}
	values := followTo(t, b.post(base+"/columns", url.Values{"go": {"next"}, "col_4": {"ignore"}, "mode": {"skip"},
		"date_format": {"DD/MM/YYYY"}}), "values")
	followTo(t, b.post(values, url.Values{"go": {"back"}}), "columns")
	again := b.page(columns)
	story := between(t, again, `name="col_4"`, "</select>")
	if !strings.Contains(story, `<option value="ignore" selected>`) {
		t.Errorf("Back lost the column chosen:\n%s", story)
	}
	if !strings.Contains(again, `value="skip" checked`) || !strings.Contains(again, `value="DD/MM/YYYY"`) {
		t.Errorf("Back lost the mode or the date format")
	}
	upload := followTo(t, b.post(columns, url.Values{"go": {"back"}}), "import")
	if !strings.Contains(b.page(upload), "tasks.csv is uploaded") {
		t.Errorf("Back to the upload step forgot the file")
	}
}

func TestImportRunsFromAPastedListStraightToThePreview(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	preview := followTo(t, b.uploadImport(map[string]string{"go": "next", "lines": "Fix login #bug !high\nWrite notes"}, "", ""), "preview")
	page := b.page(preview)
	if !strings.Contains(page, "Fix login") || !strings.Contains(page, `name="go" value="import"`) {
		t.Fatalf("the preview of a list does not offer the import:\n%s", page)
	}
	followTo(t, b.post(preview, url.Values{"go": {"import"}}), "result")
	if !strings.Contains(b.page("/tasks?q=tag:bug"), "Fix login") {
		t.Errorf("the imported task is not on the list")
	}
}

func TestImportOffersSkipToPreviewWhenEveryColumnIsKnown(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	columns := followTo(t, b.uploadImport(map[string]string{"go": "next"}, "t.csv", "Title,Status,Tags\nA,todo,x\n"), "columns")
	page := b.page(columns)
	if !strings.Contains(page, "data-import-confident") || !strings.Contains(importFoot(t, page), `value="preview"`) {
		t.Errorf("a file whose every column is known does not offer Skip to preview")
	}
	followTo(t, b.uploadImport(map[string]string{"go": "preview"}, "t.csv", "Title\nA\n"), "preview")
}

func TestImportIsTheUploadersAlone(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	columns := followTo(t, f.as("alice").uploadImport(map[string]string{"go": "next"}, "t.csv", "Title\nA\n"), "columns")
	resp := f.as("member:someone-else").get(columns)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusForbidden {
		t.Errorf("another reader opened somebody's import: %d", resp.StatusCode)
	}
	if page := f.as("viewer").page("/projects/infra"); strings.Contains(page, `href="/projects/infra/import"`) {
		t.Errorf("a viewer, who cannot import, is offered Import")
	}
	if page := f.as("alice").page("/projects/infra"); !strings.Contains(page, `href="/projects/infra/import"`) {
		t.Errorf("an admin is not offered Import")
	}
}

func TestImportErrorsDownloadAsCSV(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	preview := followTo(t, b.uploadImport(map[string]string{"go": "preview"}, "t.csv", "Title,Due\n=bad,not a date\n"), "preview")
	page := b.page(preview)
	if strings.Contains(page, `name="go" value="import"`) {
		t.Errorf("a file with a row in error offers the import")
	}
	resp := b.get(strings.TrimSuffix(preview, "/preview") + "/errors.csv")
	report := body(t, resp)
	if resp.Header.Get("Content-Type") != "text/csv; charset=utf-8" || !strings.HasPrefix(report, "line,error\n2,") {
		t.Errorf("error report = %q", report)
	}
}

// TestImportFilesDownloadAsAttachmentsTheBrowserWillNotDraw holds the two
// files an import offers to the headers that keep a mapping or a report, both
// text the uploader wrote, from being read as a page.
func TestImportFilesDownloadAsAttachmentsTheBrowserWillNotDraw(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	preview := followTo(t, b.uploadImport(map[string]string{"go": "preview"}, "t.csv", "Title,Due\n<script>x</script>,not a date\n"), "preview")
	base := strings.TrimSuffix(preview, "/preview")
	for _, tc := range []struct{ file, contentType, disposition string }{
		{"errors.csv", "text/csv; charset=utf-8", `attachment; filename="import-errors.csv"`},
		{"mapping.yaml", "application/yaml; charset=utf-8", `attachment; filename="infra-mapping.yaml"`},
	} {
		resp := b.get(base + "/" + tc.file)
		_ = body(t, resp)
		h := resp.Header
		if h.Get("Content-Type") != tc.contentType || h.Get("Content-Disposition") != tc.disposition ||
			h.Get("X-Content-Type-Options") != "nosniff" || !strings.Contains(h.Get("Content-Security-Policy"), "sandbox") {
			t.Errorf("%s: Content-Type %q, Content-Disposition %q, X-Content-Type-Options %q, CSP %q", tc.file,
				h.Get("Content-Type"), h.Get("Content-Disposition"), h.Get("X-Content-Type-Options"),
				h.Get("Content-Security-Policy"))
		}
	}
}

// bigCSV is a CSV of tasks with long descriptions, of about size bytes.
func bigCSV(size int) string {
	var b strings.Builder
	b.WriteString("Title,Description\n")
	row := strings.Repeat("x", 100<<10)
	for i := 0; b.Len() < size; i++ {
		b.WriteString("task " + strconv.Itoa(i) + "," + row + "\n")
	}
	return b.String()
}

// TestTheImportUploadTakesTheImportLimitAndNoOtherFormDoes uploads a 5 MiB
// file, which every other form's 1 MiB would refuse, then one over a lowered
// import limit, and a form elsewhere over its own limit.
func TestTheImportUploadTakesTheImportLimitAndNoOtherFormDoes(t *testing.T) {
	t.Parallel()
	b := newFixture(t).as("alice")
	preview := followTo(t, b.uploadImport(map[string]string{"go": "preview"}, "big.csv", bigCSV(5<<20)), "preview")
	if page := b.page(preview); !strings.Contains(page, "task 49") {
		t.Errorf("the preview of a 5 MiB upload does not show its rows")
	}

	small := newFixtureWith(t, web.WithImportLimit(256<<10)).as("alice")
	resp := small.uploadImport(map[string]string{}, "big.csv", bigCSV(512<<10))
	page := body(t, resp)
	if resp.StatusCode != http.StatusRequestEntityTooLarge ||
		!strings.Contains(html.UnescapeString(page), config.ImportLimitRefusal(256<<10)) {
		t.Errorf("an upload over the import limit answered %d without naming the limit and its key", resp.StatusCode)
	}

	resp = b.post("/timeformat", url.Values{"time_format": {strings.Repeat("x", 2<<20)}, "next": {"/tasks"}})
	page = body(t, resp)
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(page, "larger than 1 MiB") {
		t.Errorf("a 2 MiB form elsewhere answered %d", resp.StatusCode)
	}
}
