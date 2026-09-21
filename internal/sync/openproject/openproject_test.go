package openproject

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	extsync "github.com/heliopsy/tix/internal/sync"
)

// workPackagePage is one recorded OpenProject collection answer.
func workPackagePage(offset, pageSize, total int, ids ...int) string {
	elements := make([]map[string]any, 0, len(ids))
	for _, wpID := range ids {
		elements = append(elements, map[string]any{
			"id":        wpID,
			"subject":   fmt.Sprintf("work package %d", wpID),
			"updatedAt": fmt.Sprintf("2026-01-%02dT00:00:00Z", wpID),
			"_links":    map[string]any{"status": map[string]any{"title": "New"}},
		})
	}
	raw, err := json.Marshal(map[string]any{
		"total": total, "count": len(ids), "offset": offset, "pageSize": pageSize,
		"_embedded": map[string]any{"elements": elements},
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func newImporter(t *testing.T, srv *httptest.Server, sleep func(time.Duration), query string) *Importer {
	t.Helper()
	cfg, err := extsync.LoadSourceConfig("ops", func(key string) string {
		switch key {
		case "TIX_SYNC_OPS_URL":
			return srv.URL
		case "TIX_SYNC_OPS_PROJECT":
			return "demo"
		case "TIX_SYNC_OPS_QUERY":
			return query
		case "TIX_SYNC_OPS_PAGE_SIZE":
			return "2"
		default:
			return ""
		}
	})
	if err != nil {
		t.Fatalf("LoadSourceConfig: %v", err)
	}
	i, err := New(Options{Config: cfg, Client: srv.Client(), Sleep: sleep, MaxRetries: 3, BaseDelay: time.Millisecond})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return i
}

func TestOpenProjectPagesThroughEveryResult(t *testing.T) {
	var paths []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		switch r.URL.Query().Get("offset") {
		case "1":
			_, _ = w.Write([]byte(workPackagePage(1, 2, 3, 1, 2)))
		default:
			_, _ = w.Write([]byte(workPackagePage(2, 2, 3, 3)))
		}
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {}, "")
	if i.System() != System {
		t.Errorf("System() = %q", i.System())
	}

	var ids []string
	opt := extsync.Options{}
	for {
		batch, err := i.Fetch(context.Background(), opt)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		for _, r := range batch.Records {
			ids = append(ids, extsync.Text(r.Fields["id"]))
		}
		if batch.Page == "" {
			break
		}
		opt.Page = batch.Page
	}
	if strings.Join(ids, ",") != "1,2,3" {
		t.Errorf("imported %v, want every work package from every page", ids)
	}
	for _, p := range paths {
		if !strings.Contains(p, "/projects/demo/work_packages") {
			t.Errorf("path %q does not scope to the configured project", p)
		}
	}
}

func TestOpenProjectRetriesAThrottledSource(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(workPackagePage(1, 2, 1, 1)))
	}))
	defer srv.Close()

	var slept []time.Duration
	i := newImporter(t, srv, func(d time.Duration) { slept = append(slept, d) }, "")
	batch, err := i.Fetch(context.Background(), extsync.Options{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(batch.Records) != 1 {
		t.Errorf("read %d records after backing off", len(batch.Records))
	}
	if len(slept) != 1 {
		t.Errorf("backoff = %v, want one wait", slept)
	}
	if i.Attempts() != 2 {
		t.Errorf("attempts = %d, want the retry counted", i.Attempts())
	}
}

func TestOpenProjectNarrowsARefreshToTheCursor(t *testing.T) {
	var filters string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters = r.URL.Query().Get("filters")
		_, _ = w.Write([]byte(workPackagePage(1, 2, 0)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {}, "")
	if _, err := i.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(filters, "updatedAt") {
		t.Errorf("filters %q do not narrow to the cursor", filters)
	}
	if _, err := i.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z", Full: true}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if strings.Contains(filters, "updatedAt") {
		t.Errorf("a full refresh still narrowed to the cursor: %q", filters)
	}
}

func TestOpenProjectMergesTheCursorIntoConfiguredFilters(t *testing.T) {
	var filters string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		filters = r.URL.Query().Get("filters")
		_, _ = w.Write([]byte(workPackagePage(1, 2, 0)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {}, `[{"type":{"operator":"=","values":["1"]}}]`)
	if _, err := i.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(filters, `"type"`) || !strings.Contains(filters, "updatedAt") {
		t.Errorf("filters %q dropped either the configured filter or the cursor", filters)
	}

	j := newImporter(t, srv, func(time.Duration) {}, "not-a-filter-set")
	if _, err := j.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(filters, "updatedAt") {
		t.Errorf("filters %q dropped the cursor", filters)
	}
}

func TestOpenProjectBuildsWorkPackageLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(workPackagePage(1, 2, 1, 7)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {}, "")
	batch, err := i.Fetch(context.Background(), extsync.Options{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := extsync.Text(batch.Records[0].Fields["url"]); !strings.HasSuffix(got, "/work_packages/7") {
		t.Errorf("url = %q, want a link back to the source record", got)
	}
	if batch.Cursor == "" {
		t.Error("batch carried no cursor")
	}
}

func TestOpenProjectRefusesIncompleteConfigurationAndBadTokens(t *testing.T) {
	if _, err := New(Options{Config: extsync.SourceConfig{}}); err == nil {
		t.Error("New accepted a source with no base url")
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(workPackagePage(1, 2, 0)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {}, "")
	if _, err := i.Fetch(context.Background(), extsync.Options{Page: "0"}); err == nil {
		t.Fatal("Fetch accepted a page number below one")
	}
}
