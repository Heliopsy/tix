// SPDX-License-Identifier: AGPL-3.0-or-later

package client

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

type signalWriter struct {
	mu      sync.Mutex
	buf     bytes.Buffer
	written chan struct{}
}

func (w *signalWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case w.written <- struct{}{}:
	default:
	}
	return w.buf.Write(p)
}

func (w *signalWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buf.String()
}

func TestExportStreamsWithoutBuffering(t *testing.T) {
	var streamed bool
	var mu sync.Mutex
	out := &signalWriter{written: make(chan struct{}, 1)}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentNDJSON)
		_, _ = io.WriteString(w, `{"kind":"header"}`+"\n")
		w.(http.Flusher).Flush()

		select {
		case <-out.written:
			mu.Lock()
			streamed = true
			mu.Unlock()
		case <-time.After(3 * time.Second):
		}
		_, _ = io.WriteString(w, `{"kind":"task"}`+"\n")
	}))
	defer srv.Close()

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	if err := c.ExportTo(context.Background(), core.ExportInput{IncludeComments: true}, out); err != nil {
		t.Fatalf("export: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if !streamed {
		t.Fatal("client buffered the whole snapshot before writing")
	}
	if got := out.String(); !strings.Contains(got, `"header"`) || !strings.Contains(got, `"task"`) {
		t.Fatalf("body = %q", got)
	}
}

func TestExportSurfacesServerError(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, core.Forbidden("no export scope"))
	})
	err := c.ExportTo(context.Background(), core.ExportInput{}, io.Discard)
	if !core.IsKind(err, core.KindForbidden) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
	if got.path != wire.RouteExport || got.method != http.MethodPost {
		t.Fatalf("%s %s", got.method, got.path)
	}
	if got.header.Get(wire.HeaderAccept) != wire.ContentNDJSON {
		t.Fatalf("accept = %q", got.header.Get(wire.HeaderAccept))
	}
}

func TestImportStreamsRequestBody(t *testing.T) {
	var lines []string
	readFirst := make(chan struct{})
	var contentLength int64
	var query, contentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentLength = r.ContentLength
		query = r.URL.RawQuery
		contentType = r.Header.Get(wire.HeaderContentType)

		scanner := bufio.NewScanner(r.Body)
		for scanner.Scan() {
			lines = append(lines, scanner.Text())
			if len(lines) == 1 {
				close(readFirst)
			}
		}
		_ = json.NewEncoder(w).Encode(core.ImportResult{Created: map[string]int{"task": 2}, DryRun: true})
	}))
	defer srv.Close()

	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer func() { _ = c.Close() }()

	pr, pw := io.Pipe()
	go func() {
		_, _ = io.WriteString(pw, "{\"kind\":\"header\"}\n")
		select {
		case <-readFirst:
		case <-time.After(3 * time.Second):
		}
		_, _ = io.WriteString(pw, "{\"kind\":\"task\"}\n")
		_ = pw.Close()
	}()

	res, err := c.ImportFrom(context.Background(), pr, core.ImportInput{Mode: core.ImportMerge, DryRun: true})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if res.Created["task"] != 2 || !res.DryRun {
		t.Fatalf("result = %+v", res)
	}
	if len(lines) != 2 {
		t.Fatalf("lines = %v", lines)
	}
	if contentLength != -1 {
		t.Fatalf("content length = %d, want a streamed body", contentLength)
	}
	if !strings.Contains(query, "mode=merge") || !strings.Contains(query, "dry_run=true") {
		t.Fatalf("query = %q", query)
	}
	if contentType != wire.ContentNDJSON {
		t.Fatalf("content type = %q", contentType)
	}
}

func TestImportSurfacesServerError(t *testing.T) {
	c, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, core.Invalid("import mode is required"))
	})
	_, err := c.ImportFrom(context.Background(), strings.NewReader(""), core.ImportInput{})
	if !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("kind = %q", core.KindOf(err))
	}
}

func TestTransferTransportFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(okHandler))
	c, err := New(srv.URL, "t")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	srv.Close()

	if err := c.ExportTo(context.Background(), core.ExportInput{}, io.Discard); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("export kind = %q", core.KindOf(err))
	}
	if _, err := c.ImportFrom(context.Background(), strings.NewReader(""), core.ImportInput{}); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("import kind = %q", core.KindOf(err))
	}
}

// A task export posts its choices to the project's route and copies the
// server's file through untouched.
func TestExportTasksPostsTheChoicesAndCopiesTheFile(t *testing.T) {
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set(wire.HeaderContentType, wire.ContentCSV)
		_, _ = io.WriteString(w, "title\nFix login\n")
	})
	var out bytes.Buffer
	in := core.TaskExportInput{ProjectRef: "infra", Columns: []string{"title"}}
	if err := c.ExportTasks(context.Background(), in, &out); err != nil {
		t.Fatalf("ExportTasks: %v", err)
	}
	if out.String() != "title\nFix login\n" {
		t.Errorf("body = %q", out.String())
	}
	if got.method != http.MethodPost || got.path != strings.Replace(wire.RouteProjectTaskExport, "{ref}", "infra", 1) {
		t.Errorf("%s %s", got.method, got.path)
	}
	var sent core.TaskExportInput
	if err := json.Unmarshal([]byte(got.body), &sent); err != nil || len(sent.Columns) != 1 || sent.Columns[0] != "title" {
		t.Errorf("sent %q (%v)", got.body, err)
	}

	refused, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, core.NotFound("project %q", "ghost"))
	})
	if err := refused.ExportTasks(context.Background(), core.TaskExportInput{ProjectRef: "ghost"}, io.Discard); !core.IsKind(err, core.KindNotFound) {
		t.Errorf("a refused export answered %v", err)
	}
}

// A task import is a multipart form carrying every choice as a field and the
// file as its part; the content of the input stands in for a missing reader.
func TestImportTasksUploadsAMultipartForm(t *testing.T) {
	// newClient records the raw body before the handler runs, so the form is
	// checked from that copy.
	c, got := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(core.TaskImportResult{DryRun: true, Created: 1})
	})

	in := core.TaskImportInput{ProjectRef: "infra", Format: core.TaskFormatCSV, Mode: core.TaskImportSkip,
		Mapping: "columns: {Name: title}", DryRun: true, Preview: 5, Filename: "jira.csv"}
	res, err := c.ImportTasks(context.Background(), strings.NewReader("Name\nFix login\n"), in)
	if err != nil {
		t.Fatalf("ImportTasks: %v", err)
	}
	if !res.DryRun || res.Created != 1 {
		t.Errorf("result = %+v", res)
	}
	if !strings.Contains(got.body, `name="mapping"`) || !strings.Contains(got.body, "columns: {Name: title}") ||
		!strings.Contains(got.body, `filename="jira.csv"`) || !strings.Contains(got.body, "Name\nFix login\n") {
		t.Errorf("the form lacks a field or the file:\n%s", got.body)
	}
	for _, want := range []string{"csv", "skip", "5", "true"} {
		if !strings.Contains(got.body, "\r\n\r\n"+want+"\r\n") {
			t.Errorf("the form carries no field %q:\n%s", want, got.body)
		}
	}
	if !strings.HasPrefix(got.header.Get(wire.HeaderContentType), "multipart/form-data; boundary=") {
		t.Errorf("content type = %q", got.header.Get(wire.HeaderContentType))
	}

	if _, err := c.ImportTasks(context.Background(), nil, core.TaskImportInput{ProjectRef: "infra", Content: "from content"}); err != nil {
		t.Fatalf("ImportTasks from content: %v", err)
	}
	if !strings.Contains(got.body, `filename="tasks"`) || !strings.Contains(got.body, "from content") {
		t.Errorf("the content did not stand in for the file:\n%s", got.body)
	}

	garbled, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "not json") })
	if _, err := garbled.ImportTasks(context.Background(), nil, core.TaskImportInput{ProjectRef: "p"}); !core.IsKind(err, core.KindInternal) {
		t.Errorf("an unreadable report answered %v", err)
	}
	refused, _ := newClient(t, func(w http.ResponseWriter, _ *http.Request) {
		httpapi.WriteError(w, core.Forbidden("no import scope"))
	})
	if _, err := refused.ImportTasks(context.Background(), nil, core.TaskImportInput{ProjectRef: "p"}); !core.IsKind(err, core.KindForbidden) {
		t.Errorf("a refused import answered %v", err)
	}
}
