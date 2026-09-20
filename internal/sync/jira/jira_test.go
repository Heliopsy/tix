package jira

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

	extsync "github.com/thereisnotime/tix/internal/sync"
)

// issuePage is one recorded Jira search answer.
func issuePage(startAt, total int, keys ...string) string {
	issues := make([]map[string]any, 0, len(keys))
	for i, key := range keys {
		issues = append(issues, map[string]any{
			"key": key,
			"fields": map[string]any{
				"summary": "issue " + key,
				"status":  map[string]any{"name": "To Do"},
				"updated": fmt.Sprintf("2026-01-%02dT00:00:00Z", startAt+i+1),
			},
		})
	}
	raw, err := json.Marshal(map[string]any{
		"startAt": startAt, "maxResults": len(keys), "total": total, "issues": issues,
	})
	if err != nil {
		panic(err)
	}
	return string(raw)
}

func newImporter(t *testing.T, srv *httptest.Server, sleep func(time.Duration)) *Importer {
	t.Helper()
	cfg, err := extsync.LoadSourceConfig("ops", func(key string) string {
		switch key {
		case "TIX_SYNC_OPS_URL":
			return srv.URL
		case "TIX_SYNC_OPS_PROJECT":
			return "OPS"
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

func TestJiraPagesThroughEveryResult(t *testing.T) {
	var queries []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		queries = append(queries, r.URL.Query().Get("jql"))
		switch r.URL.Query().Get("startAt") {
		case "0":
			_, _ = w.Write([]byte(issuePage(0, 3, "OPS-1", "OPS-2")))
		default:
			_, _ = w.Write([]byte(issuePage(2, 3, "OPS-3")))
		}
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {})
	if i.System() != System {
		t.Errorf("System() = %q", i.System())
	}

	var keys []string
	opt := extsync.Options{}
	for {
		batch, err := i.Fetch(context.Background(), opt)
		if err != nil {
			t.Fatalf("Fetch: %v", err)
		}
		for _, r := range batch.Records {
			keys = append(keys, extsync.Text(r.Fields["key"]))
		}
		if batch.Page == "" {
			if batch.Cursor == "" {
				t.Error("the last page carried no cursor")
			}
			break
		}
		opt.Page = batch.Page
	}
	if strings.Join(keys, ",") != "OPS-1,OPS-2,OPS-3" {
		t.Errorf("imported %v, want every issue from every page", keys)
	}
	for _, q := range queries {
		if !strings.Contains(q, "ORDER BY updated ASC") {
			t.Errorf("jql %q does not order by update time", q)
		}
	}
}

func TestJiraRetriesAThrottledSource(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		if calls.Add(1) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(issuePage(0, 1, "OPS-1")))
	}))
	defer srv.Close()

	var slept []time.Duration
	i := newImporter(t, srv, func(d time.Duration) { slept = append(slept, d) })
	batch, err := i.Fetch(context.Background(), extsync.Options{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(batch.Records) != 1 {
		t.Errorf("read %d records after backing off", len(batch.Records))
	}
	if len(slept) != 1 || slept[0] != time.Second {
		t.Errorf("backoff = %v, want one wait of the requested second", slept)
	}
	if i.Attempts() != 2 {
		t.Errorf("attempts = %d, want the retry counted", i.Attempts())
	}
}

func TestJiraNarrowsARefreshToTheCursor(t *testing.T) {
	var jql string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		jql = r.URL.Query().Get("jql")
		_, _ = w.Write([]byte(issuePage(0, 0)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {})
	if _, err := i.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z"}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if !strings.Contains(jql, "updated >=") {
		t.Errorf("jql %q does not narrow to the cursor", jql)
	}

	if _, err := i.Fetch(context.Background(), extsync.Options{Cursor: "2026-02-01T10:00:00Z", Full: true}); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if strings.Contains(jql, "updated >=") {
		t.Errorf("a full refresh still narrowed to the cursor: %q", jql)
	}
}

func TestJiraBuildsBrowseLinks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(issuePage(0, 1, "OPS-1")))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {})
	batch, err := i.Fetch(context.Background(), extsync.Options{})
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if got := extsync.Text(batch.Records[0].Fields["url"]); !strings.HasSuffix(got, "/browse/OPS-1") {
		t.Errorf("url = %q, want a link back to the source record", got)
	}
}

func TestJiraRefusesIncompleteConfiguration(t *testing.T) {
	if _, err := New(Options{Config: extsync.SourceConfig{}}); err == nil {
		t.Error("New accepted a source with no base url")
	}
	if _, err := New(Options{Config: extsync.SourceConfig{BaseURL: "https://jira.test"}}); err == nil {
		t.Error("New accepted a source with neither a project nor a query")
	}
}

func TestJiraRejectsABadPageToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(issuePage(0, 0)))
	}))
	defer srv.Close()

	i := newImporter(t, srv, func(time.Duration) {})
	if _, err := i.Fetch(context.Background(), extsync.Options{Page: "soon"}); err == nil {
		t.Fatal("Fetch accepted a page token that is not an offset")
	}
}
