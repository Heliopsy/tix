// SPDX-License-Identifier: AGPL-3.0-or-later

package server

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/outbox"

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
