package server

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/httpapi"
	"github.com/heliopsy/tix/internal/outbox"
	"github.com/heliopsy/tix/internal/store"
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
	hub    *httpapi.Hub
	log    eventLog
	poll   time.Duration
	logger *slog.Logger

	// reader builds the source a tenant's pump tails, so a test can supply one
	// that fails on demand.
	reader func(tenantID string) outbox.Reader

	// retryBackoff, retryMax and retryLimit shape how long a pump rides out a
	// failing database before it gives up and frees the tenant.
	retryBackoff time.Duration
	retryMax     time.Duration
	retryLimit   int

	mu      sync.Mutex
	running map[string]*pump
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// pump is one tenant's running reader.
type pump struct{ cancel context.CancelFunc }

func newPumps(ctx context.Context, hub *httpapi.Hub, log eventLog, poll time.Duration, logger *slog.Logger) *pumps {
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	if logger == nil {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithCancel(ctx)
	p := &pumps{
		hub: hub, log: log, poll: poll, logger: logger,
		running:      make(map[string]*pump),
		ctx:          ctx,
		cancel:       cancel,
		retryBackoff: outbox.DefaultRetryBackoff,
		retryMax:     outbox.DefaultMaxBackoff,
		retryLimit:   outbox.DefaultRetryLimit,
	}
	p.reader = func(tenantID string) outbox.Reader {
		return tenantReader{log: p.log, tenantID: tenantID}
	}
	return p
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
	if p.ctx.Err() != nil {
		return
	}
	ctx, cancel := context.WithCancel(p.ctx)
	cur := &pump{cancel: cancel}
	p.running[tenantID] = cur

	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		defer p.finish(tenantID, cur)
		p.run(ctx, tenantID)
	}()
}

// stop ends the reader for tenantID.
func (p *pumps) stop(tenantID string) {
	p.mu.Lock()
	cur, ok := p.running[tenantID]
	delete(p.running, tenantID)
	p.mu.Unlock()
	if ok {
		cur.cancel()
	}
}

// finish releases a reader that ended on its own, so the next listener on that
// tenant starts a fresh one instead of being refused by a stale entry.
func (p *pumps) finish(tenantID string, cur *pump) {
	cur.cancel()
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.running[tenantID] == cur {
		delete(p.running, tenantID)
	}
}

func (p *pumps) run(ctx context.Context, tenantID string) {
	tailer := outbox.NewTailer(p.reader(tenantID), p.poll, 256,
		outbox.WithRetry(p.retryBackoff, p.retryMax, p.retryLimit),
		outbox.WithErrorHandler(func(err error) {
			p.logger.Error("tenant event reader failed", "tenant", tenantID, "error", err.Error())
		}),
	)
	events, err := tailer.Subscribe(ctx, core.EventFilter{})
	if err != nil {
		p.logger.Error("tenant event reader did not start", "tenant", tenantID, "error", err.Error())
		return
	}
	for e := range events {
		p.hub.Broadcast(e)
	}
	if ctx.Err() == nil {
		p.logger.Warn("tenant event reader ended; it restarts on the next connection", "tenant", tenantID)
	}
}

// serve ties the readers to the caller's lifecycle, stopping every one of them
// once ctx is done.
func (p *pumps) serve(ctx context.Context) error {
	<-ctx.Done()
	p.shutdown()
	return nil
}

// shutdown cancels every reader, including any started concurrently, and waits.
func (p *pumps) shutdown() {
	p.cancel()
	p.mu.Lock()
	for tenantID, cur := range p.running {
		cur.cancel()
		delete(p.running, tenantID)
	}
	p.mu.Unlock()
	p.wg.Wait()
}

// wait blocks until every reader has stopped.
func (p *pumps) wait() { p.wg.Wait() }

// active reports how many tenants currently have a reader.
func (p *pumps) active() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.running)
}
