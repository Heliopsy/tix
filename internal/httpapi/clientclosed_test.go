// SPDX-License-Identifier: AGPL-3.0-or-later

package httpapi_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/wire"
)

// loggedRecord is one record the router logged, flattened for assertions.
type loggedRecord struct {
	level slog.Level
	msg   string
	attrs map[string]string
}

// logSink hands every record the router logs to the test as it is written,
// so a test waits for the request line instead of sleeping for it.
type logSink struct{ records chan loggedRecord }

func newLogSink() *logSink { return &logSink{records: make(chan loggedRecord, 256)} }

func (s *logSink) Enabled(context.Context, slog.Level) bool { return true }

func (s *logSink) Handle(_ context.Context, r slog.Record) error {
	rec := loggedRecord{level: r.Level, msg: r.Message, attrs: map[string]string{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.String()
		return true
	})
	s.records <- rec
	return nil
}

func (s *logSink) WithAttrs([]slog.Attr) slog.Handler { return s }
func (s *logSink) WithGroup(string) slog.Handler      { return s }

// requestLine waits for the request record of path and fails on any record
// at error level logged before it.
func (s *logSink) requestLine(t *testing.T, path string) loggedRecord {
	t.Helper()
	timeout := time.After(10 * time.Second)
	for {
		select {
		case rec := <-s.records:
			if rec.level >= slog.LevelError {
				t.Errorf("logged at %s: %s %v", rec.level, rec.msg, rec.attrs)
			}
			if rec.msg == "request" && rec.attrs["path"] == path {
				return rec
			}
		case <-timeout:
			t.Fatalf("no request line for %s was logged", path)
		}
	}
}

// withLogSink routes the fixture's router logs into sink.
func withLogSink(sink *logSink) func(*apiFixture, *httpapi.Config) {
	return func(_ *apiFixture, cfg *httpapi.Config) { cfg.Logger = slog.New(sink) }
}

func TestALongPollTheClientAbandonsIsLoggedAsClientClosed(t *testing.T) {
	sink := newLogSink()
	f := newFixtureWith(t, withLogSink(sink))
	start := f.appendEvents(f.projectA.ID, core.EventTaskCreated)[0]

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet,
		f.server.URL+pollPath("wait=25s&since_seq="+strconv.FormatInt(start, 10)), nil)
	if err != nil {
		t.Fatalf("building request: %v", err)
	}
	req.Host = f.hostA
	req.Header.Set(wire.HeaderAuth, "Bearer "+f.tokenA)
	done := make(chan error, 1)
	go func() {
		resp, err := f.server.Client().Do(req)
		if err == nil {
			_ = resp.Body.Close()
		}
		done <- err
	}()

	deadline := time.Now().Add(10 * time.Second)
	for f.clock.Waiters() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("the poll never started waiting")
		}
		select {
		case err := <-done:
			t.Fatalf("the poll answered without waiting: %v", err)
		case <-time.After(time.Millisecond):
		}
	}
	cancel()
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("client error = %v, want the cancellation", err)
	}

	line := sink.requestLine(t, wire.RouteEventsPoll)
	if got := line.attrs["status"]; got != strconv.Itoa(httpapi.StatusClientClosed) {
		t.Errorf("logged status = %s, want 499", got)
	}
	if line.level != slog.LevelInfo || !strings.Contains(line.attrs["note"], "client closed") {
		t.Errorf("request line = %s %v, want info with a client closed note", line.level, line.attrs)
	}
}

func TestAServiceFailureIsStillAServerError(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{"unknown error", errors.New("disk on fire")},
		{"cancelled while the client waits", context.Canceled},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sink := newLogSink()
			f := newFixtureWith(t, withLogSink(sink), func(f *apiFixture, _ *httpapi.Config) { f.svc.fail = c.err })

			resp := f.call(http.MethodGet, wire.RouteLabels, nil)
			mustStatus(t, resp, http.StatusInternalServerError)
			_ = resp.Body.Close()
			if got := sink.requestLine(t, wire.RouteLabels).attrs["status"]; got != "500" {
				t.Errorf("logged status = %s, want 500", got)
			}
		})
	}
}

func TestTheServerTimeoutIsStillLoggedAs504(t *testing.T) {
	sink := newLogSink()
	f := newFixtureWith(t, withLogSink(sink), func(f *apiFixture, cfg *httpapi.Config) {
		cfg.RequestTimeout = 20 * time.Millisecond
		f.svc.block = make(chan struct{})
	})
	t.Cleanup(func() { close(f.svc.block) })

	resp := f.call(http.MethodGet, wire.RouteLabels, nil)
	mustStatus(t, resp, http.StatusGatewayTimeout)
	_ = resp.Body.Close()
	if got := sink.requestLine(t, wire.RouteLabels).attrs["status"]; got != "504" {
		t.Errorf("logged status = %s, want 504", got)
	}
}

// quietLog is an event log with nothing in it.
type quietLog struct{}

func (quietLog) ReadSince(context.Context, string, int64, int) ([]core.Event, error) { return nil, nil }
func (quietLog) OldestSeq(context.Context, string) (int64, error)                    { return 0, nil }

func TestAWebSocketClientThatDropsLogsNoError(t *testing.T) {
	sink := newLogSink()
	f := newFixtureWith(t, withLogSink(sink), func(_ *apiFixture, cfg *httpapi.Config) {
		cfg.EventHandler = httpapi.NewEventStream(httpapi.NewHub(), quietLog{})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(f.server.URL, "http") + wire.RouteEvents
	c, resp, err := websocket.Dial(ctx, url, &websocket.DialOptions{
		Host:       f.hostA,
		HTTPHeader: http.Header{wire.HeaderAuth: {"Bearer " + f.tokenA}},
	})
	if err != nil {
		t.Fatalf("dialling: %v", err)
	}
	if resp.Body != nil {
		_ = resp.Body.Close()
	}
	_ = c.CloseNow()

	if got := sink.requestLine(t, wire.RouteEvents).attrs["status"]; got != "101" {
		t.Errorf("logged status = %s, want 101", got)
	}
}

// The request record names the tenant the request was served in. The record
// is written outside the middleware that resolves the tenant, so it used to
// read a context the tenant was never put on and logged it empty.
func TestTheRequestLineNamesTheResolvedTenant(t *testing.T) {
	sink := newLogSink()
	f := newFixtureWith(t, withLogSink(sink))

	resp := f.call(http.MethodGet, wire.RouteLabels, nil)
	mustStatus(t, resp, http.StatusOK)
	_ = resp.Body.Close()
	if got := sink.requestLine(t, wire.RouteLabels).attrs["tenant"]; got != f.tenantA.ID {
		t.Errorf("logged tenant = %q, want the tenant served, %q", got, f.tenantA.ID)
	}
}
