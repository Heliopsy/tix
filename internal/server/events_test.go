// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/heliopsy/tix/internal/outbox"
	"github.com/heliopsy/tix/internal/wire"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/store"
	"github.com/heliopsy/tix/internal/store/sqlite"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
)

// A tenant reader must exist only while someone is listening, so an idle server
// does no work and a busy one reads each tenant exactly once.
func TestPumpsStartAndStopWithTenantHooks(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond, nil)

	p.start("t1")
	p.start("t1")
	if got := p.active(); got != 1 {
		t.Errorf("active readers = %d, want 1 for a repeated start", got)
	}

	p.start("t2")
	if got := p.active(); got != 2 {
		t.Errorf("active readers = %d, want 2", got)
	}

	p.stop("t1")
	if got := p.active(); got != 1 {
		t.Errorf("active readers after stop = %d, want 1", got)
	}

	p.stop("t2")
	if got := p.active(); got != 0 {
		t.Errorf("active readers after all stopped = %d, want 0", got)
	}
	p.wait()
}

func TestPumpsIgnoreAnEmptyTenant(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond, nil)
	p.start("")
	if got := p.active(); got != 0 {
		t.Errorf("an empty tenant started %d readers, want 0", got)
	}
}

func TestPumpsStopIsSafeWhenNotRunning(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond, nil)

	// Asserting only that this does not panic would pass with stop() gutted to
	// a no-op, so the set it manages is what gets checked: stopping a tenant
	// that never ran must leave a running one alone.
	p.start("live")
	defer p.stop("live")
	if got := p.active(); got != 1 {
		t.Fatalf("active after starting one = %d, want 1", got)
	}

	p.stop("never-started")

	if got := p.active(); got != 1 {
		t.Fatalf("active after stopping a tenant that never ran = %d, want the live one untouched", got)
	}
}

// OldestSeq backs the pruned-cursor error, so a resuming subscriber is told
// rather than silently skipping events.
func TestOldestSeqOnAnEmptyLog(t *testing.T) {
	got, err := eventLog{store: testStore(t)}.OldestSeq(context.Background(), "missing-tenant")
	if err != nil {
		t.Fatalf("OldestSeq: %v", err)
	}
	if got != 0 {
		t.Errorf("OldestSeq on an empty log = %d, want 0", got)
	}
}

// eventLog must satisfy the contract the WebSocket handler expects.
var _ httpapi.EventLog = eventLog{}

var _ = core.Event{}

// testStore opens a migrated database for the event tests.
func testStore(t *testing.T) store.Store {
	t.Helper()
	st, err := sqlite.Open(filepath.Join(t.TempDir(), "tix.db"), clock.New())
	if err != nil {
		t.Fatalf("opening store: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("migrating: %v", err)
	}
	return st
}

// failingReader stands in for a tenant's durable log while the database is
// unavailable.
type failingReader struct {
	mu    sync.Mutex
	err   error
	reads int
}

func (r *failingReader) ReadSince(context.Context, int64, int) ([]core.Event, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.reads++
	return nil, r.err
}

func (r *failingReader) Latest(context.Context) (int64, error) { return 0, nil }

// A reader that gives up must release its slot. Holding the cancel func of a
// dead pump made start a no-op, so the tenant's stream stayed silent for the
// life of the process however many times a client reconnected.
func TestPumpsReleaseATenantWhoseReaderDied(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{}, time.Millisecond, nil)

	p.retryBackoff, p.retryMax, p.retryLimit = 0, 0, 1

	dead := make(chan struct{})
	p.reader = func(string) outbox.Reader {
		close(dead)
		return &failingReader{err: errors.New("database is unavailable")}
	}

	p.start("t1")
	<-dead
	p.wait()

	if got := p.active(); got != 0 {
		t.Fatalf("active readers after the reader died = %d, want 0", got)
	}

	restarted := make(chan struct{})
	p.reader = func(string) outbox.Reader {
		close(restarted)
		return &failingReader{err: errors.New("database is still unavailable")}
	}
	p.start("t1")
	select {
	case <-restarted:
	case <-time.After(5 * time.Second):
		t.Fatal("a tenant whose reader died was never given a new one")
	}
	p.shutdown()
}

// Every read failure must reach the log, so a stream that stops is visible.
func TestPumpsLogEveryReadFailure(t *testing.T) {
	var buf lockedBuffer
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{}, time.Millisecond,
		slog.New(slog.NewTextHandler(&buf, nil)))
	p.retryBackoff, p.retryMax, p.retryLimit = 0, 0, 1
	p.reader = func(string) outbox.Reader {
		return &failingReader{err: errors.New("database is unavailable")}
	}

	p.start("t1")
	p.wait()

	if got := buf.String(); !strings.Contains(got, "database is unavailable") {
		t.Errorf("logged output = %q, want the read failure", got)
	}
}

// Readers were tied to context.Background(), so they outlived the process they
// belonged to. They must end with the server.
func TestPumpsStopWithTheServerLifecycle(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond, nil)
	p.start("t1")
	p.start("t2")
	if got := p.active(); got != 2 {
		t.Fatalf("active readers = %d, want 2", got)
	}

	ctx, cancel := context.WithCancel(context.Background())
	served := make(chan error, 1)
	go func() { served <- EventPumpWorker(p).Run(ctx) }()

	cancel()
	if err := <-served; err != nil {
		t.Fatalf("the event pump worker returned %v", err)
	}
	if got := p.active(); got != 0 {
		t.Errorf("active readers after shutdown = %d, want 0", got)
	}
}

// A reader started after shutdown would never be stopped by anything.
func TestPumpsRefuseToStartAfterShutdown(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond, nil)
	p.shutdown()
	p.start("t1")
	if got := p.active(); got != 0 {
		t.Errorf("active readers after shutdown = %d, want 0", got)
	}
}

// lockedBuffer collects log output written from the pump goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A tenant's live cursor must exist before start returns. The caller is the
// hub, which registers, acknowledges and replays the connection afterwards, so
// a cursor taken later can sit above an event replay has already passed. The
// tailer never reads below the cursor it opened with, so such an event is lost
// rather than late.
//
// The guard is written from the subscriber's side deliberately: it commits an
// event in that window and requires the client to receive it. An assertion
// about which goroutine calls Latest would still pass a refactor that reopened
// the gap by another route.
func TestPumpTakesItsLiveCursorBeforeStartReturns(t *testing.T) {
	st := testStore(t)
	const tenantID = "cursor-window"
	makeTenant(t, st, tenantID)
	before := appendTaskEvent(t, st, tenantID, "before-anyone-listened")

	hub := httpapi.NewHub()
	p := newPumps(context.Background(), hub, eventLog{store: st}, time.Millisecond, nil)
	defer p.shutdown()

	held := &heldReader{
		inner:   tenantReader{log: eventLog{store: st}, tenantID: tenantID},
		entered: make(chan struct{}),
		proceed: make(chan struct{}),
	}
	p.reader = func(string) outbox.Reader { return held }

	// Closed once the hub's first-connection hook has handed back, which is the
	// moment the connection becomes servable.
	startReturned := make(chan struct{})
	var startedOnce sync.Once
	hub.SetTenantHooks(func(id string) {
		p.start(id)
		startedOnce.Do(func() { close(startReturned) })
	}, p.stop)

	stream := httpapi.NewEventStream(hub, eventLog{store: st})
	actor := &core.Actor{ID: "actor", TenantID: tenantID, Scopes: core.AllScopes}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		stream.ServeHTTP(w, r.WithContext(core.WithActor(r.Context(), actor)))
	}))
	defer srv.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	conn := dialEvents(t, ctx, srv.URL)
	defer func() { _ = conn.CloseNow() }()

	<-held.entered

	seen := make(map[int64]bool)
	settled := false
	select {
	case <-startReturned:
		// The hook handed back before the cursor existed, so the connection is
		// already being served: drive the subscription past its replay, which
		// is where the undelivered event sat.
		settleSubscription(t, ctx, conn, before, seen)
		settled = true
	default:
		// The cursor is being taken inside start, so the hub has not finished
		// registering and there is nothing for the client to settle yet.
	}

	live := appendTaskEvent(t, st, tenantID, "committed-in-the-window")
	close(held.proceed)

	if !settled {
		settleSubscription(t, ctx, conn, before, seen)
	}
	if seen[live] {
		return
	}

	readCtx, readCancel := context.WithTimeout(ctx, 10*time.Second)
	defer readCancel()
	for {
		m, err := readMessage(readCtx, conn)
		if err != nil {
			t.Fatalf("event %d, committed once the connection was registered, never arrived: %v", live, err)
		}
		if m.Type == httpapi.MsgEvent && m.Event != nil && m.Event.Seq == live {
			return
		}
	}
}

// heldReader is a tenant's real reader whose cursor read can be held, standing
// in for a pump goroutine the scheduler has not run.
type heldReader struct {
	inner   outbox.Reader
	once    sync.Once
	entered chan struct{}
	proceed chan struct{}
}

func (r *heldReader) Latest(ctx context.Context) (int64, error) {
	r.once.Do(func() { close(r.entered) })
	<-r.proceed
	return r.inner.Latest(ctx)
}

func (r *heldReader) ReadSince(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error) {
	return r.inner.ReadSince(ctx, sinceSeq, limit)
}

func makeTenant(t *testing.T, st store.Store, id string) {
	t.Helper()
	ctx := context.Background()
	err := st.Unscoped(ctx, func(tx store.UnscopedTx) error {
		return tx.CreateTenant(ctx, &core.Tenant{ID: id, Key: id, Name: id})
	})
	if err != nil {
		t.Fatalf("creating tenant %q: %v", id, err)
	}
}

func appendTaskEvent(t *testing.T, st store.Store, tenantID, subject string) int64 {
	t.Helper()
	ctx := context.Background()
	var seq int64
	err := st.Update(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		e := &core.Event{
			Type:        core.EventTaskCreated,
			SubjectType: "task",
			SubjectID:   subject,
			OccurredAt:  time.Now().UTC(),
		}
		if err := tx.AppendEvent(ctx, e); err != nil {
			return err
		}
		seq = e.Seq
		return nil
	})
	if err != nil {
		t.Fatalf("appending event %q: %v", subject, err)
	}
	return seq
}

func dialEvents(t *testing.T, ctx context.Context, baseURL string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(baseURL, "http") + wire.RouteEvents
	conn, resp, err := websocket.Dial(ctx, url, nil)
	if resp != nil && resp.Body != nil {
		_ = resp.Body.Close()
	}
	if err != nil {
		t.Fatalf("dialling the event stream: %v", err)
	}
	return conn
}

func readMessage(ctx context.Context, conn *websocket.Conn) (httpapi.ServerMessage, error) {
	var m httpapi.ServerMessage
	_, raw, err := conn.Read(ctx)
	if err != nil {
		return m, err
	}
	return m, json.Unmarshal(raw, &m)
}

// settleSubscription subscribes from sinceSeq and returns only once the server
// has finished replaying and gone live. The pong answers on the same loop that
// handles subscribe, so receiving it proves replay is behind us rather than
// merely acknowledged.
func settleSubscription(t *testing.T, ctx context.Context, conn *websocket.Conn, sinceSeq int64, seen map[int64]bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	write(t, ctx, conn, httpapi.ClientMessage{Type: httpapi.MsgSubscribe, ID: "s1", SinceSeq: &sinceSeq})
	awaitMessage(t, ctx, conn, httpapi.MsgSubscribed, seen)
	write(t, ctx, conn, httpapi.ClientMessage{Type: httpapi.MsgPing, ID: "p1"})
	awaitMessage(t, ctx, conn, httpapi.MsgPong, seen)
}

func write(t *testing.T, ctx context.Context, conn *websocket.Conn, m httpapi.ClientMessage) {
	t.Helper()
	raw, err := json.Marshal(m)
	if err != nil {
		t.Fatalf("marshalling %s: %v", m.Type, err)
	}
	if err := conn.Write(ctx, websocket.MessageText, raw); err != nil {
		t.Fatalf("writing %s: %v", m.Type, err)
	}
}

func awaitMessage(t *testing.T, ctx context.Context, conn *websocket.Conn, want string, seen map[int64]bool) {
	t.Helper()
	for {
		m, err := readMessage(ctx, conn)
		if err != nil {
			t.Fatalf("waiting for %s: %v", want, err)
		}
		if m.Type == httpapi.MsgEvent && m.Event != nil {
			seen[m.Event.Seq] = true
		}
		if m.Type == httpapi.MsgError {
			t.Fatalf("the event stream answered %s with an error: %+v", want, m.Error)
		}
		if m.Type == want {
			return
		}
	}
}
