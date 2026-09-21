package sync

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// recorder collects the delays a client backs off for, so a test can assert on
// backoff without sleeping.
type recorder struct{ delays []time.Duration }

func (r *recorder) sleep(d time.Duration) { r.delays = append(r.delays, d) }

func newTestClient(t *testing.T, server *httptest.Server, rec *recorder) *HTTPClient {
	t.Helper()
	c, err := NewHTTPClient(HTTPOptions{
		System: "test", BaseURL: server.URL, Client: server.Client(),
		Sleep: rec.sleep, MaxRetries: 4, BaseDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	return c
}

func TestHTTPClientRequiresABaseURL(t *testing.T) {
	if _, err := NewHTTPClient(HTTPOptions{System: "test"}); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("NewHTTPClient = %v, want invalid", err)
	}
}

func TestHTTPClientSendsTheConfiguredCredential(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	rec := &recorder{}
	c, err := NewHTTPClient(HTTPOptions{
		System: "test", BaseURL: srv.URL, Client: srv.Client(), Sleep: rec.sleep,
		Config: SourceConfig{}.WithCredentials("tok", "", ""),
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	var out map[string]any
	if err := c.GetJSON(context.Background(), "/x", nil, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if seen != "Bearer tok" {
		t.Errorf("authorization = %q", seen)
	}

	basic, err := NewHTTPClient(HTTPOptions{
		System: "test", BaseURL: srv.URL, Client: srv.Client(), Sleep: rec.sleep,
		Config: SourceConfig{}.WithCredentials("", "apikey", "pw"),
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	if err := basic.GetJSON(context.Background(), "/x", nil, nil); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if !strings.HasPrefix(seen, "Basic ") {
		t.Errorf("authorization = %q, want basic", seen)
	}
}

// A throttled source must be waited out rather than abandoned, and the wait
// must honour the Retry-After the source asked for.
func TestHTTPClientBacksOffAndSucceedsAfterThrottling(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if calls.Add(1) <= 2 {
			w.Header().Set("Retry-After", "2")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		_, _ = w.Write([]byte(`{"value":"done"}`))
	}))
	defer srv.Close()

	rec := &recorder{}
	c := newTestClient(t, srv, rec)
	var out struct {
		Value string `json:"value"`
	}
	if err := c.GetJSON(context.Background(), "/search", url.Values{"q": {"x"}}, &out); err != nil {
		t.Fatalf("GetJSON: %v", err)
	}
	if out.Value != "done" {
		t.Errorf("value = %q", out.Value)
	}
	if len(rec.delays) != 2 {
		t.Fatalf("backed off %d times, want 2", len(rec.delays))
	}
	for _, d := range rec.delays {
		if d != 2*time.Second {
			t.Errorf("delay = %v, want the source's Retry-After", d)
		}
	}
	if c.Attempts() != 3 {
		t.Errorf("attempts = %d, want 3", c.Attempts())
	}
	if c.Waited() != 4*time.Second {
		t.Errorf("waited = %v", c.Waited())
	}
}

func TestHTTPClientRetriesServerErrorsThenGivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	rec := &recorder{}
	c := newTestClient(t, srv, rec)
	err := c.GetJSON(context.Background(), "/x", nil, nil)
	if err == nil {
		t.Fatal("GetJSON succeeded against a source that never answers")
	}
	if !strings.Contains(err.Error(), "after 5 attempts") {
		t.Errorf("error = %v, want the attempt count reported", err)
	}
	if len(rec.delays) != 4 {
		t.Errorf("backed off %d times, want 4", len(rec.delays))
	}
	if rec.delays[0] >= rec.delays[1] {
		t.Errorf("delays %v do not grow", rec.delays)
	}
}

// A rejected credential must be reported without the credential in the message.
func TestHTTPClientReportsRejectionWithoutTheCredential(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	rec := &recorder{}
	c, err := NewHTTPClient(HTTPOptions{
		System: "test", BaseURL: srv.URL, Client: srv.Client(), Sleep: rec.sleep,
		Config: SourceConfig{}.WithCredentials("s3cr3t", "", ""),
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	err = c.GetJSON(context.Background(), "/x", nil, nil)
	if !core.IsKind(err, core.KindUnauthenticated) {
		t.Fatalf("GetJSON = %v, want unauthenticated", err)
	}
	if strings.Contains(err.Error(), "s3cr3t") {
		t.Errorf("error disclosed the credential: %v", err)
	}
	if len(rec.delays) != 0 {
		t.Errorf("a rejected credential was retried %d times", len(rec.delays))
	}
}

func TestHTTPClientDoesNotRetryClientErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()

	rec := &recorder{}
	c := newTestClient(t, srv, rec)
	if err := c.GetJSON(context.Background(), "/x", nil, nil); !core.IsKind(err, core.KindInvalid) {
		t.Fatalf("GetJSON = %v, want invalid", err)
	}
	if len(rec.delays) != 0 {
		t.Errorf("a client error was retried %d times", len(rec.delays))
	}
}

func TestHTTPClientReportsUndecodableBodies(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	rec := &recorder{}
	c := newTestClient(t, srv, rec)
	var out map[string]any
	if err := c.GetJSON(context.Background(), "/x", nil, &out); !core.IsKind(err, core.KindInternal) {
		t.Fatalf("GetJSON = %v, want internal", err)
	}
}

func TestHTTPClientStopsWhenTheContextIsCancelled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	rec := &recorder{}
	c, err := NewHTTPClient(HTTPOptions{
		System: "test", BaseURL: srv.URL, Client: srv.Client(),
		Sleep: func(time.Duration) { cancel() }, MaxRetries: 3, BaseDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatalf("NewHTTPClient: %v", err)
	}
	if err := c.GetJSON(ctx, "/x", nil, nil); err == nil {
		t.Fatal("GetJSON ignored a cancelled context")
	}
	_ = rec
}

func TestRetryAfterReadsSecondsAndDates(t *testing.T) {
	if got := retryAfter("3"); got != 3*time.Second {
		t.Errorf("retryAfter(3) = %v", got)
	}
	if got := retryAfter(""); got != 0 {
		t.Errorf("retryAfter(empty) = %v", got)
	}
	if got := retryAfter("nonsense"); got != 0 {
		t.Errorf("retryAfter(nonsense) = %v", got)
	}
	future := time.Now().Add(90 * time.Second).UTC().Format(http.TimeFormat)
	if got := retryAfter(future); got <= 0 {
		t.Errorf("retryAfter(date) = %v", got)
	}
	past := time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat)
	if got := retryAfter(past); got != 0 {
		t.Errorf("retryAfter(past date) = %v", got)
	}
}

func TestPageSizeClamps(t *testing.T) {
	if got := PageSize(0); got != DefaultPageSize {
		t.Errorf("PageSize(0) = %d", got)
	}
	if got := PageSize(5000); got != 1000 {
		t.Errorf("PageSize(5000) = %d", got)
	}
	if got := PageSize(7); got != 7 {
		t.Errorf("PageSize(7) = %d", got)
	}
}

func TestBatchJSONRoundTrips(t *testing.T) {
	raw, err := json.Marshal(Batch{Records: []Record{{Fields: map[string]any{"a": "b"}}}, Page: "2"})
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var back Batch
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	if back.Page != "2" || len(back.Records) != 1 {
		t.Errorf("round trip = %+v", back)
	}
}
