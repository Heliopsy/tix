// SPDX-License-Identifier: AGPL-3.0-or-later

package store

import (
	"context"
	"sync"
)

// CommitWaker is implemented by a store that can tell a reader a transaction
// carrying events has committed, so the reader acts on the commit rather than
// on its next poll. It is optional in both directions: a store that does not
// implement it is polled as before, and a reader handed a signal still keeps
// its own ticker, because a wake that is dropped must cost latency and never an
// event.
type CommitWaker interface {
	// WakeOnCommit returns a channel that receives whenever a transaction
	// appending events to tenantID commits. Signals coalesce, so a receiver
	// learns that there is something to read rather than how much. The
	// registration is released when ctx ends.
	WakeOnCommit(ctx context.Context, tenantID string) (<-chan struct{}, error)
}

// Wakers fans a commit signal out to the readers watching one tenant. Both
// engine packages hold one: the difference between them is only where the
// signal comes from, not what is done with it.
type Wakers struct {
	mu       sync.Mutex
	byTenant map[string]map[chan struct{}]struct{}
}

// NewWakers builds an empty fan-out.
func NewWakers() *Wakers {
	return &Wakers{byTenant: map[string]map[chan struct{}]struct{}{}}
}

// Add registers a receiver for tenantID until ctx ends.
func (w *Wakers) Add(ctx context.Context, tenantID string) <-chan struct{} {
	ch := make(chan struct{}, 1)
	w.mu.Lock()
	if w.byTenant[tenantID] == nil {
		w.byTenant[tenantID] = map[chan struct{}]struct{}{}
	}
	w.byTenant[tenantID][ch] = struct{}{}
	w.mu.Unlock()

	go func() {
		<-ctx.Done()
		w.remove(tenantID, ch)
	}()
	return ch
}

func (w *Wakers) remove(tenantID string, ch chan struct{}) {
	w.mu.Lock()
	defer w.mu.Unlock()
	delete(w.byTenant[tenantID], ch)
	if len(w.byTenant[tenantID]) == 0 {
		delete(w.byTenant, tenantID)
	}
}

// Wake signals every receiver watching tenantID. A receiver whose buffer is
// already full is left alone: it has not yet read the position the earlier
// signal pointed at, so a second one tells it nothing new.
func (w *Wakers) Wake(tenantID string) {
	w.mu.Lock()
	defer w.mu.Unlock()
	for ch := range w.byTenant[tenantID] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
