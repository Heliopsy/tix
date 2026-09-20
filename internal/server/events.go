package server

import (
	"context"
	"sync"
	"time"

	"github.com/thereisnotime/tix/internal/core"
	"github.com/thereisnotime/tix/internal/httpapi"
	"github.com/thereisnotime/tix/internal/outbox"
	"github.com/thereisnotime/tix/internal/store"
)

// eventLog reads the durable outbox for one tenant at a time.
type eventLog struct{ store store.Store }

// ReadSince returns committed events above sinceSeq for one tenant.
func (e eventLog) ReadSince(ctx context.Context, tenantID string, sinceSeq int64, limit int) ([]core.Event, error) {
	if e.store == nil {
		return nil, core.Internal("event log has no store")
	}
	var out []core.Event
	err := e.store.View(ctx, core.TenantScope{TenantID: tenantID}, func(tx store.Tx) error {
		var err error
		out, err = tx.ReadEvents(ctx, sinceSeq, limit)
		return err
	})
	return out, err
}

// OldestSeq reports the lowest retained sequence, so a resuming subscriber
// asking for a pruned cursor is told rather than silently missing events.
func (e eventLog) OldestSeq(ctx context.Context, tenantID string) (int64, error) {
	events, err := e.ReadSince(ctx, tenantID, 0, 1)
	if err != nil || len(events) == 0 {
		return 0, err
	}
	return events[0].Seq, nil
}

// tenantReader adapts eventLog to the cursor reader a tailer expects.
type tenantReader struct {
	log      eventLog
	tenantID string
}

func (r tenantReader) ReadSince(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error) {
	return r.log.ReadSince(ctx, r.tenantID, sinceSeq, limit)
}

func (r tenantReader) Latest(ctx context.Context) (int64, error) {
	if r.log.store == nil {
		return 0, core.Internal("event log has no store")
	}
	var latest int64
	err := r.log.store.View(ctx, core.TenantScope{TenantID: r.tenantID}, func(tx store.Tx) error {
		var err error
		latest, err = tx.LatestEventSeq(ctx)
		return err
	})
	return latest, err
}

// pumps runs one event reader per tenant that has a live connection.
//
// Events are read through a tenant-scoped transaction, so there is deliberately
// no way to read every tenant's events at once. Running a reader per listening
// tenant keeps that property and costs nothing while nobody is subscribed.
type pumps struct {
	hub  *httpapi.Hub
	log  eventLog
	poll time.Duration

	mu      sync.Mutex
	running map[string]context.CancelFunc
	ctx     context.Context
	wg      sync.WaitGroup
}

func newPumps(ctx context.Context, hub *httpapi.Hub, log eventLog, poll time.Duration) *pumps {
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	return &pumps{
		hub: hub, log: log, poll: poll,
		running: make(map[string]context.CancelFunc),
		ctx:     ctx,
	}
}

// start runs a reader for tenantID unless one is already running.
func (p *pumps) start(tenantID string) {
	if tenantID == "" {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if _, ok := p.running[tenantID]; ok {
		return
	}
	ctx, cancel := context.WithCancel(p.ctx)
	p.running[tenantID] = cancel

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		p.run(ctx, tenantID)
	}()
}

// stop ends the reader for tenantID.
func (p *pumps) stop(tenantID string) {
	p.mu.Lock()
	cancel, ok := p.running[tenantID]
	delete(p.running, tenantID)
	p.mu.Unlock()
	if ok {
		cancel()
	}
}

func (p *pumps) run(ctx context.Context, tenantID string) {
	tailer := outbox.NewTailer(tenantReader{log: p.log, tenantID: tenantID}, p.poll, 256)
	events, err := tailer.Subscribe(ctx, core.EventFilter{})
	if err != nil {
		return
	}
	for e := range events {
		p.hub.Broadcast(e)
	}
}

// wait blocks until every reader has stopped.
func (p *pumps) wait() { p.wg.Wait() }

// active reports how many tenants currently have a reader.
func (p *pumps) active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.running)
}
