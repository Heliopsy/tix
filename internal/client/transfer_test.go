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

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
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
		w.Header().Set(httpapi.HeaderContentType, httpapi.ContentNDJSON)
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
	if got.path != httpapi.RouteExport || got.method != http.MethodPost {
		t.Fatalf("%s %s", got.method, got.path)
	}
	if got.header.Get(httpapi.HeaderAccept) != httpapi.ContentNDJSON {
		t.Fatalf("accept = %q", got.header.Get(httpapi.HeaderAccept))
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
		contentType = r.Header.Get(httpapi.HeaderContentType)

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
	if contentType != httpapi.ContentNDJSON {
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
