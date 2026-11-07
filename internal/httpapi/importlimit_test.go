// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/config"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// bigImportCSV is a CSV of tasks with long descriptions, of about size bytes.
func bigImportCSV(size int) string {
	var b strings.Builder
	b.WriteString("Title,Description\n")
	row := strings.Repeat("x", 100<<10)
	for i := 0; b.Len() < size; i++ {
		fmt.Fprintf(&b, "task %d,%s\n", i, row)
	}
	return b.String()
}

// postImportForm uploads content as the file part of a multipart import. A
// chunked upload has no Content-Length, so only the body reader can stop it.
func (f *apiFixture) postImportForm(content string, chunked bool) *http.Response {
	f.t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("dry_run", "true")
	part, _ := form.CreateFormFile("file", "tasks.csv")
	_, _ = io.WriteString(part, content)
	_ = form.Close()
	var reader io.Reader = &body
	if chunked {
		reader = io.MultiReader(&body)
	}
	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	req, err := http.NewRequest(http.MethodPost, f.server.URL+route, reader)
	if err != nil {
		f.t.Fatal(err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderContentType, form.FormDataContentType())
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp
}

// TestATaskImportIsNotHeldToTheBodyLimit uploads a 5 MiB file to a server
// whose body limit is 4 KiB, and expects the import, and only the import, to
// take it.
func TestATaskImportIsNotHeldToTheBodyLimit(t *testing.T) {
	f := newFixture(t)
	content := bigImportCSV(5 << 20)
	for _, chunked := range []bool{false, true} {
		resp := f.postImportForm(content, chunked)
		mustStatus(t, resp, http.StatusOK)
		var got core.TaskImportResult
		decodeBody(t, resp, &got)
		if want := strings.Count(content, "\n") - 1; got.Created != want {
			t.Errorf("chunked %v: the import created %d tasks, want %d", chunked, got.Created, want)
		}
	}

	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	resp := f.call(http.MethodPost, route, core.TaskImportInput{Content: content, DryRun: true})
	mustStatus(t, resp, http.StatusOK)
	_ = readBody(t, resp)

	for _, route := range []string{wire.RouteTasks, wire.RouteImport} {
		resp = f.call(http.MethodPost, route, core.CreateTaskInput{ProjectRef: "infra", Title: "t", Body: strings.Repeat("x", 8<<10)})
		mustStatus(t, resp, http.StatusRequestEntityTooLarge)
		if body := readBody(t, resp); !strings.Contains(body, "request body exceeds the configured limit") {
			t.Errorf("%s: a body over the general limit was refused with %s", route, body)
		}
	}
}

// TestATaskImportOverItsLimitIsRefusedNamingIt sends an import past the
// import limit, with and without a Content-Length, as a form and as JSON.
func TestATaskImportOverItsLimitIsRefusedNamingIt(t *testing.T) {
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) { cfg.MaxImportBytes = 256 << 10 })
	want := config.ImportLimitRefusal(256 << 10)
	content := bigImportCSV(512 << 10)
	check := func(name string, resp *http.Response) {
		t.Helper()
		mustStatus(t, resp, http.StatusRequestEntityTooLarge)
		var env wire.ErrorEnvelope
		decodeBody(t, resp, &env)
		if env.Error.Message != want {
			t.Errorf("%s: refused with %q, want %q", name, env.Error.Message, want)
		}
	}
	check("form", f.postImportForm(content, false))
	check("chunked form", f.postImportForm(content, true))
	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	check("json", f.call(http.MethodPost, route, core.TaskImportInput{Content: content, DryRun: true}))
}

// TestImportRoutesOfTheWebHandlerTakeTheImportLimit posts 5 MiB to the
// browser's import upload and to another browser route, through the router
// that serves both, and reads how much of each body the handler could read.
func TestImportRoutesOfTheWebHandlerTakeTheImportLimit(t *testing.T) {
	read := func(w http.ResponseWriter, r *http.Request) {
		n, err := io.Copy(io.Discard, r.Body)
		if err != nil {
			w.WriteHeader(http.StatusRequestEntityTooLarge)
			return
		}
		_, _ = fmt.Fprint(w, n)
	}
	f := newFixtureWith(t, func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.ImportRoutes = []string{"POST /projects/{key}/import"}
		cfg.WebHandler = http.HandlerFunc(read)
	})
	body := strings.Repeat("x", 5<<20)
	for _, tc := range []struct {
		path string
		want int
	}{
		{"/projects/infra/import", http.StatusOK},
		{"/projects/infra/settings", http.StatusRequestEntityTooLarge},
	} {
		req, err := http.NewRequest(http.MethodPost, f.server.URL+tc.path, io.MultiReader(strings.NewReader(body)))
		if err != nil {
			t.Fatal(err)
		}
		req.Host = f.hostA
		req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
		resp, err := f.server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != tc.want {
			t.Errorf("POST %s with 5 MiB = %d, want %d", tc.path, resp.StatusCode, tc.want)
		}
	}
}
