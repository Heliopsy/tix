// SPDX-License-Identifier: AGPL-3.0-or-later

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/server"
	"github.com/heliopsy/tix/internal/service"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"
	"github.com/heliopsy/tix/internal/web"
	"github.com/heliopsy/tix/internal/webhook"
	"github.com/heliopsy/tix/internal/wire"
)

// Waits are bounded generously rather than tuned: the tenant reader polls the
// outbox on eventPoll, so anything slower than these bounds is a real failure,
// not a slow machine.
//
// eventWait and deliveryWait are failure timeouts, not latency budgets. A wait
// returns the instant its event arrives, so the ceiling costs nothing when the
// system works; its only job is to decide when to call a broken system broken.
// Latency is asserted by the benchmark job against a seeded fixture, and that
// is where a performance regression belongs. Do not tighten these to make them
// do the benchmark's work: on a contended runner that only manufactures false
// failures.
const (
	eventPoll    = 25 * time.Millisecond
	eventWait    = 60 * time.Second
	deliveryWait = 60 * time.Second
	dialWait     = 10 * time.Second
)

// harness runs a real server over a temporary database and keeps a second,
// independent service open on the same file. The second service is what a
// command line writing straight to the database is: it has no handle on the
// server and no way to notify it.
type harness struct {
	t *testing.T

	tenantID string
	logs     *serverLog
	baseURL  string
	token    string
	scope    core.TenantScope
	direct   *service.Local
	server   *service.Local
	cliStore *sqlite.Store
	adminCtx context.Context
}

// harnessOption adjusts how the harness is wired.
type harnessOption func(*harnessConfig)

// harnessConfig carries the wiring choices a test makes.
type harnessConfig struct {
	directHooks      service.HookMode
	dispatchInterval time.Duration
}

// withDirectHooks sets who delivers webhooks for the direct-database writer.
// The server mode hands the queue to the running server, which is what a
// command line configured with `--webhook-mode server` does.
func withDirectHooks(m service.HookMode) harnessOption {
	return func(c *harnessConfig) { c.directHooks = m }
}

// withDispatchInterval paces the server's own webhook dispatcher.
func withDispatchInterval(d time.Duration) harnessOption {
	return func(c *harnessConfig) { c.dispatchInterval = d }
}

// newHarness boots the database, the server and the direct-database service.
func newHarness(t *testing.T, opts ...harnessOption) *harness {
	t.Helper()
	cfg := harnessConfig{directHooks: service.HookInline}
	for _, opt := range opts {
		opt(&cfg)
	}
	ctx := context.Background()
	clk := clock.New()
	logs := &serverLog{}
	path := filepath.Join(t.TempDir(), "tix.db")

	serverStore := openStore(t, path, clk)
	serverSvc := service.New(serverStore,
		service.WithClock(clk),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())))

	tenant, err := serverSvc.EnsureDefaults(ctx)
	if err != nil {
		t.Fatalf("bootstrapping defaults: %v", err)
	}
	adminCtx := core.WithSource(core.WithActor(ctx, core.SystemActor(tenant.ID)), core.SourceCLI)

	user, err := serverSvc.CreateUser(adminCtx, core.CreateUserInput{
		Email: "agent@example.test", Handle: "agent", Role: core.RoleAdmin,
	})
	if err != nil {
		t.Fatalf("creating the token holder: %v", err)
	}
	issued, err := serverSvc.CreateToken(adminCtx, core.CreateTokenInput{
		Name: "integration", ActorID: user.ID, Scopes: []core.Scope{core.ScopeAll},
	})
	if err != nil {
		t.Fatalf("minting a token: %v", err)
	}

	srv, err := server.Assemble(server.Options{
		Service:           serverSvc,
		Store:             serverStore,
		Clock:             clk,
		Logger:            slog.New(slog.NewTextHandler(logs, nil)),
		TenantID:          tenant.ID,
		WebHandler:        web.Handler(serverSvc),
		Addr:              "127.0.0.1:0",
		EventPollInterval: eventPoll,
		DispatchInterval:  cfg.dispatchInterval,
		DisableSweep:      true,
		DisablePrune:      true,
	})
	if err != nil {
		t.Fatalf("assembling the server: %v", err)
	}
	if err := srv.Listen(); err != nil {
		t.Fatalf("binding the server: %v", err)
	}

	serveCtx, stop := context.WithCancel(ctx)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = srv.Serve(serveCtx)
	}()
	t.Cleanup(func() {
		stop()
		wg.Wait()
	})

	cliActor := &core.Actor{
		ID: user.ID, TenantID: tenant.ID, Kind: core.ActorUser,
		Handle: "agent", Role: core.RoleAdmin, Scopes: []core.Scope{core.ScopeAll},
	}

	cliStore := openStore(t, path, clk)
	direct := service.New(cliStore,
		service.WithClock(clk),
		service.WithHooks(cfg.directHooks),
		service.WithHasher(auth.NewHasherWithParams(auth.TestParams())))

	return &harness{
		t:        t,
		tenantID: tenant.ID,
		logs:     logs,
		server:   serverSvc,
		baseURL:  "http://" + srv.Addr(),
		token:    issued.Token,
		scope:    core.TenantScope{TenantID: tenant.ID},
		direct:   direct,
		cliStore: cliStore,
		adminCtx: core.WithSource(core.WithActor(ctx, cliActor), core.SourceCLI),
	}
}

// openStore opens and migrates a store on its own connection pool.
func openStore(t *testing.T, path string, clk clock.Clock) *sqlite.Store {
	t.Helper()
	st, err := sqlite.Open(path, clk)
	if err != nil {
		t.Fatalf("opening the store at %q: %v", path, err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating %q: %v", path, err)
	}
	return st
}

// createTaskDirectly writes a task through the direct-database service, which
// is the whole point: nothing here talks to the running server.
func (h *harness) createTaskDirectly(title string) *core.Task {
	h.t.Helper()
	task, err := h.direct.CreateTask(h.adminCtx, core.CreateTaskInput{
		ProjectRef: service.DefaultProjectKey, Title: title,
	})
	if err != nil {
		h.t.Fatalf("creating task %q on the database directly: %v", title, err)
	}
	return task
}

// latestEvent reads the newest committed event straight out of the outbox.
func (h *harness) latestEvent() core.Event {
	h.t.Helper()
	ctx := context.Background()
	var out []core.Event
	err := h.cliStore.View(ctx, h.scope, func(tx store.Tx) error {
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		out, err = tx.ReadEvents(ctx, latest-1, 1)
		return err
	})
	if err != nil {
		h.t.Fatalf("reading the newest event: %v", err)
	}
	if len(out) == 0 {
		h.t.Fatal("the outbox is empty after a mutation committed")
	}
	return out[0]
}

// serverLog collects the server's own log for a failing test to print. The
// three lines that name a tenant reader which did not start, failed, or ended
// are written nowhere else, so discarding them left a delivery timeout with no
// cause attached to it.
type serverLog struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

// Write records one log line.
func (l *serverLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

// String returns everything logged so far.
func (l *serverLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.String()
}

// diagnose renders what an undelivered event needs to be explained: whether it
// reached the outbox at all, and whether the tenant reader said anything. A
// bare timeout cannot tell a write that never committed from one the reader
// skipped, and those have opposite causes.
func (h *harness) diagnose() string {
	var b strings.Builder
	b.WriteString("\ncommitted outbox tail:")
	ctx := context.Background()
	var out []core.Event
	err := h.cliStore.View(ctx, h.scope, func(tx store.Tx) error {
		latest, err := tx.LatestEventSeq(ctx)
		if err != nil {
			return err
		}
		from := max(latest-10, 0)
		out, err = tx.ReadEvents(ctx, from, 10)
		return err
	})
	switch {
	case err != nil:
		fmt.Fprintf(&b, " unreadable: %v", err)
	case len(out) == 0:
		b.WriteString(" empty")
	default:
		for _, e := range out {
			fmt.Fprintf(&b, "\n  seq=%d type=%s subject=%s", e.Seq, e.Type, e.SubjectID)
		}
	}
	if logged := h.logs.String(); logged != "" {
		fmt.Fprintf(&b, "\nserver log:\n%s", logged)
	} else {
		b.WriteString("\nserver log: silent, so the tenant reader reported no failure")
	}
	return b.String()
}

// wsClient is a real WebSocket client on the server's event route.
type wsClient struct {
	t    *testing.T
	h    *harness
	conn *websocket.Conn
}

// dial opens an authenticated event-stream connection.
func (h *harness) dial() *wsClient {
	h.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), dialWait)
	defer cancel()

	conn, resp, err := websocket.Dial(ctx, "ws"+h.baseURL[len("http"):]+wire.RouteEvents,
		&websocket.DialOptions{
			HTTPHeader: http.Header{"Authorization": []string{"Bearer " + h.token}},
		})
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		h.t.Fatalf("dialling the event stream: %v", err)
	}
	c := &wsClient{t: h.t, h: h, conn: conn}
	h.t.Cleanup(c.close)
	return c
}

// close ends the connection, tolerating one that is already gone.
func (c *wsClient) close() { _ = c.conn.CloseNow() }

// subscribe sends a subscription and waits for its acknowledgement. A nil
// sinceSeq subscribes live; a value replays the durable log from that cursor.
func (c *wsClient) subscribe(id string, sinceSeq *int64) {
	c.t.Helper()
	msg := httpapi.ClientMessage{Type: httpapi.MsgSubscribe, ID: id, SinceSeq: sinceSeq}
	raw, err := json.Marshal(msg)
	if err != nil {
		c.t.Fatalf("encoding subscribe: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), dialWait)
	defer cancel()
	if err := c.conn.Write(ctx, websocket.MessageText, raw); err != nil {
		c.t.Fatalf("sending subscribe: %v", err)
	}
	ack := c.read(dialWait)
	if ack.Type != httpapi.MsgSubscribed {
		c.t.Fatalf("subscribe was answered with %q (%+v), want %q", ack.Type, ack.Error, httpapi.MsgSubscribed)
	}
}

// read returns the next server message, failing the test on timeout.
func (c *wsClient) read(within time.Duration) httpapi.ServerMessage {
	c.t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), within)
	defer cancel()

	typ, raw, err := c.conn.Read(ctx)
	if err != nil {
		c.t.Fatalf("reading from the event stream within %s: %v%s", within, err, c.h.diagnose())
	}
	if typ != websocket.MessageText {
		c.t.Fatalf("event stream sent a %v frame, want text", typ)
	}
	var m httpapi.ServerMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		c.t.Fatalf("decoding a server message: %v", err)
	}
	return m
}

// awaitEvent reads until an event names subjectID, so unrelated traffic on the
// stream cannot make the assertion flaky.
func (c *wsClient) awaitEvent(subjectID string, within time.Duration) core.Event {
	c.t.Helper()
	deadline := time.Now().Add(within)
	for {
		remaining := time.Until(deadline)
		if remaining <= 0 {
			c.t.Fatalf("no event for subject %q arrived within %s; "+
				"a direct-database write must reach a server reading the same outbox%s",
				subjectID, within, c.h.diagnose())
		}
		m := c.read(remaining)
		switch {
		case m.Type == httpapi.MsgError:
			c.t.Fatalf("event stream reported an error: %+v", m.Error)
		case m.Type == httpapi.MsgEvent && m.Event != nil && m.Event.SubjectID == subjectID:
			return *m.Event
		}
	}
}

// hookRecorder is a webhook receiver that records what was posted to it.
type hookRecorder struct {
	server *httptest.Server

	mu       sync.Mutex
	received chan struct{}
	requests []hookRequest
}

// hookRequest is one delivery as the receiving end saw it.
type hookRequest struct {
	body      []byte
	timestamp string
	signature string
	eventType string
	delivery  string
}

// newHookRecorder starts an endpoint that accepts every delivery.
func newHookRecorder(t *testing.T) *hookRecorder {
	t.Helper()
	r := &hookRecorder{received: make(chan struct{}, 16)}
	r.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, err := io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		r.mu.Lock()
		r.requests = append(r.requests, hookRequest{
			body:      body,
			timestamp: req.Header.Get(webhook.HeaderTimestamp),
			signature: req.Header.Get(webhook.HeaderSignature),
			eventType: req.Header.Get(webhook.HeaderEvent),
			delivery:  req.Header.Get(webhook.HeaderDelivery),
		})
		r.mu.Unlock()
		select {
		case r.received <- struct{}{}:
		default:
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(r.server.Close)
	return r
}

// await blocks until a delivery lands or the bound elapses.
func (r *hookRecorder) await(t *testing.T, within time.Duration) hookRequest {
	t.Helper()
	select {
	case <-r.received:
	case <-time.After(within):
		t.Fatalf("no webhook delivery arrived within %s", within)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.requests[len(r.requests)-1]
}
