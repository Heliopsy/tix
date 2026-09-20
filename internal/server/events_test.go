package server

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/thereisnotime/tix/internal/clock"
	"github.com/thereisnotime/tix/internal/store"
	"github.com/thereisnotime/tix/internal/store/sqlite"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
)

// A tenant reader must exist only while someone is listening, so an idle server
// does no work and a busy one reads each tenant exactly once.
func TestPumpsStartAndStopWithTenantHooks(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond)

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
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond)
	p.start("")
	if got := p.active(); got != 0 {
		t.Errorf("an empty tenant started %d readers, want 0", got)
	}
}

func TestPumpsStopIsSafeWhenNotRunning(t *testing.T) {
	p := newPumps(context.Background(), httpapi.NewHub(), eventLog{store: testStore(t)}, time.Millisecond)
	p.stop("never-started")
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
