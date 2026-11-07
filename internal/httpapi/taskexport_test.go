// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/wire"
)

func TestTaskExportAnswersInTheFormatAsked(t *testing.T) {
	f := newFixture(t)
	f.createTask("exported")
	route := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "infra", 1)
	for _, tc := range []struct {
		format core.TaskFormat
		media  string
		file   string
	}{
		{core.TaskFormatCSV, wire.ContentCSV, "infra-tasks.csv"},
		{core.TaskFormatJSON, wire.ContentJSON, "infra-tasks.json"},
		{core.TaskFormatNDJSON, wire.ContentNDJSON, "infra-tasks.ndjson"},
		{core.TaskFormatMarkdown, wire.ContentMarkdown, "infra-tasks.md"},
	} {
		t.Run(string(tc.format), func(t *testing.T) {
			resp := f.call(http.MethodPost, route, core.TaskExportInput{Format: tc.format, Columns: []string{"title"}})
			mustStatus(t, resp, http.StatusOK)
			if got := resp.Header.Get(wire.HeaderContentType); got != tc.media {
				t.Errorf("content type = %q, want %q", got, tc.media)
			}
			if got := resp.Header.Get(wire.HeaderContentDisposition); !strings.Contains(got, tc.file) {
				t.Errorf("content disposition = %q, want %s", got, tc.file)
			}
			if body := readBody(t, resp); !strings.Contains(body, "exported") {
				t.Errorf("body = %s", body)
			}
		})
	}
}

func TestTaskExportRefusesInTheErrorEnvelope(t *testing.T) {
	f := newFixture(t)
	route := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "infra", 1)
	resp := f.call(http.MethodPost, route, core.TaskExportInput{Columns: []string{"nope"}})
	mustStatus(t, resp, http.StatusBadRequest)
	if got := resp.Header.Get(wire.HeaderContentDisposition); got != "" {
		t.Errorf("a refusal was offered as a download: %q", got)
	}
	missing := strings.Replace(wire.RouteProjectTaskExport, "{ref}", "nope", 1)
	mustStatus(t, f.call(http.MethodPost, missing, nil), http.StatusNotFound)
}

func TestTaskImportTakesJSONOrAMultipartUpload(t *testing.T) {
	f := newFixture(t)
	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	resp := f.call(http.MethodPost, route, core.TaskImportInput{Content: "Title\nfrom json\n", DryRun: true})
	mustStatus(t, resp, http.StatusOK)
	var dry core.TaskImportResult
	decodeBody(t, resp, &dry)
	if !dry.DryRun || dry.Committed || dry.Created != 1 {
		t.Errorf("json dry run = %+v", dry)
	}

	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	_ = form.WriteField("mapping", "columns: {Name: title}\n")
	part, _ := form.CreateFormFile("file", "tasks.csv")
	_, _ = part.Write([]byte("Name\nfrom a form\n"))
	_ = form.Close()
	req, err := http.NewRequest(http.MethodPost, f.server.URL+route, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderContentType, form.FormDataContentType())
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	resp, err = f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	mustStatus(t, resp, http.StatusOK)
	var done core.TaskImportResult
	decodeBody(t, resp, &done)
	if !done.Committed || done.Created != 1 || done.Preview[0].Title != "from a form" {
		t.Errorf("multipart import = %+v", done)
	}
}

// postForm sends a multipart import built by fill to the project's route.
func postForm(t *testing.T, f *apiFixture, fill func(*multipart.Writer)) *http.Response {
	t.Helper()
	var body bytes.Buffer
	form := multipart.NewWriter(&body)
	fill(form)
	_ = form.Close()
	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	req, err := http.NewRequest(http.MethodPost, f.server.URL+route, &body)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderContentType, form.FormDataContentType())
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

// A form may carry the mapping as a file part and the tasks as a content
// field; its choices are read from fields, and a bad one names itself.
func TestTaskImportReadsEveryPartOfAForm(t *testing.T) {
	f := newFixture(t)
	resp := postForm(t, f, func(form *multipart.Writer) {
		mapping, _ := form.CreateFormFile("mapping", "map.yaml")
		_, _ = mapping.Write([]byte("columns: {Name: title}\n"))
		_ = form.WriteField("content", "Name\nfrom content\n")
		_ = form.WriteField("dry_run", "on")
		_ = form.WriteField("preview", "5")
		_ = form.WriteField("mode", "create")
	})
	mustStatus(t, resp, http.StatusOK)
	var dry core.TaskImportResult
	decodeBody(t, resp, &dry)
	if !dry.DryRun || dry.Committed || dry.Mode != core.TaskImportCreate || len(dry.Preview) != 1 ||
		dry.Preview[0].Title != "from content" {
		t.Errorf("form dry run = %+v", dry)
	}

	resp = postForm(t, f, func(form *multipart.Writer) { _ = form.WriteField("preview", "many") })
	mustStatus(t, resp, http.StatusBadRequest)
	if body := readBody(t, resp); !strings.Contains(body, "preview") {
		t.Errorf("a bad preview answered %s", body)
	}

	route := strings.Replace(wire.RouteProjectTaskImport, "{ref}", "infra", 1)
	req, _ := http.NewRequest(http.MethodPost, f.server.URL+route, strings.NewReader("not a form"))
	req.Host = f.hostA
	req.Header.Set(wire.HeaderContentType, "multipart/form-data; boundary=x")
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	resp, err := f.server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	mustStatus(t, resp, http.StatusBadRequest)
	if body := readBody(t, resp); !strings.Contains(body, "reading the upload") {
		t.Errorf("a broken form answered %s", body)
	}
}
