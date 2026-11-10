// SPDX-License-Identifier: AGPL-3.0-or-later

package web_test

import (
	"bytes"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"testing"
)

// postArtifactFile submits the task's artifact form with a file of the given
// type, as a browser does.
func (b *browser) postArtifactFile(ref, filename, contentType, content string) *http.Response {
	b.t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	_ = w.WriteField("csrf_token", b.csrf())
	_ = w.WriteField("kind", "file")
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", `form-data; name="file"; filename="`+filename+`"`)
	h.Set("Content-Type", contentType)
	part, err := w.CreatePart(h)
	if err != nil {
		b.t.Fatal(err)
	}
	_, _ = io.WriteString(part, content)
	_ = w.Close()
	req, err := http.NewRequest(http.MethodPost, b.fix.server.URL+"/tasks/"+ref+"/artifacts", &body)
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

func TestTheArtifactFormStatesTheLimitsAndServesFilesAsDownloads(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")
	ref := b.createTask("infra", "with files")

	form := between(t, b.page("/tasks/"+ref), `action="/tasks/`+ref+`/artifacts"`, "</form>")
	for _, want := range []string{`enctype="multipart/form-data"`, `type="file"`,
		`accept="image/png,image/jpeg,`, "Files up to 1 MiB. Accepted types: image/png, image/jpeg"} {
		if !strings.Contains(html.UnescapeString(form), want) {
			t.Errorf("the artifact form lacks %q:\n%s", want, form)
		}
	}

	refused := b.postArtifactFile(ref, "page.html", "text/html", "<script>alert(1)</script>")
	text := html.UnescapeString(readAll(t, refused))
	if refused.StatusCode == http.StatusSeeOther || !strings.Contains(text, `"text/html" is not accepted on this tenant; accepted types: image/png`) {
		t.Fatalf("an html file was answered %d:\n%s", refused.StatusCode, text)
	}

	ok := b.postArtifactFile(ref, "notes.txt", "text/plain", "hello")
	_ = ok.Body.Close()
	wantStatus(t, ok, http.StatusSeeOther)
	page := b.page("/tasks/" + ref)
	link := between(t, page, `<a href="/tasks/`+ref+`/artifacts/`, `"`)
	if link == "" {
		t.Fatalf("the artifact has no download link:\n%s", page)
	}
	resp := b.get("/tasks/" + ref + "/artifacts/" + link)
	body := readAll(t, resp)
	if body != "hello" || !strings.HasPrefix(resp.Header.Get("Content-Disposition"), "attachment") ||
		resp.Header.Get("X-Content-Type-Options") != "nosniff" || resp.Header.Get("Content-Type") != "text/plain" {
		t.Fatalf("download answered %q with %v", body, resp.Header)
	}
}

func TestTenantArtifactLimitsAreEditedInTheTenantSettings(t *testing.T) {
	t.Parallel()
	f := newFixture(t)
	b := f.as("alice")

	page := b.page("/admin/tenant")
	for _, want := range []string{`name="artifact_max_bytes"`, `placeholder="1 MiB"`, `name="artifact_types"`} {
		if !strings.Contains(page, want) {
			t.Errorf("the tenant settings lack %q", want)
		}
	}

	refused := b.post("/admin/tenant", url.Values{"name": {"Acme"}, "artifact_max_bytes": {"5MiB"}})
	if text := html.UnescapeString(readAll(t, refused)); !strings.Contains(text, "above this server's limit of 1 MiB") {
		t.Fatalf("a size above the server's was answered %d:\n%s", refused.StatusCode, text)
	}

	saved := b.post("/admin/tenant", url.Values{"name": {"Acme"}, "artifact_max_bytes": {"2KiB"}, "artifact_types": {"application/pdf"}})
	_ = saved.Body.Close()
	wantStatus(t, saved, http.StatusSeeOther)
	ref := b.createTask("infra", "pdf only")
	form := between(t, b.page("/tasks/"+ref), `action="/tasks/`+ref+`/artifacts"`, "</form>")
	if !strings.Contains(form, `accept="application/pdf"`) || !strings.Contains(form, "Files up to 2 KiB. Accepted types: application/pdf.") {
		t.Fatalf("the form does not state the tenant's limits:\n%s", form)
	}
}
