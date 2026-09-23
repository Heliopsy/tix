// SPDX-License-Identifier: AGPL-3.0-or-later

// Package outbox appends domain events and replays them to subscribers.
package outbox

import (
	"context"
	"fmt"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
)

// eventWriter is the part of a transaction the outbox needs.
type eventWriter interface {
	Scope() core.TenantScope
	AppendEvent(ctx context.Context, e *core.Event) error
}

// Append writes an event inside the caller's transaction.
func Append(ctx context.Context, tx store.Tx, gen id.Generator, now time.Time, e *core.Event) error {
	return appendTo(ctx, tx, gen, now, e)
}

func appendTo(ctx context.Context, tx eventWriter, gen id.Generator, now time.Time, e *core.Event) error {
	if e.Type == "" {
		return core.Invalid("event type is required")
	}
	if e.SubjectType == "" || e.SubjectID == "" {
		return core.Invalid("event subject is required")
	}
	if e.ID == "" {
		e.ID = gen.New()
	}
	if e.OccurredAt.IsZero() {
		e.OccurredAt = now
	}
	e.TenantID = tx.Scope().TenantID
	return tx.AppendEvent(ctx, e)
}

// Reader supplies committed events to a tailer.
type Reader interface {
	ReadSince(ctx context.Context, sinceSeq int64, limit int) ([]core.Event, error)
	Latest(ctx context.Context) (int64, error)
}

// Defaults for a tailer's response to a failing reader.
const (
	DefaultRetryBackoff = 250 * time.Millisecond
	DefaultMaxBackoff   = 30 * time.Second
	DefaultRetryLimit   = 10
)

// Tailer streams committed events from a cursor.
type Tailer struct {
	reader Reader
	poll   time.Duration
	batch  int

	backoff    time.Duration
	maxBackoff time.Duration
	retryLimit int
	onError    func(error)
}

// TailerOption configures a tailer.
type TailerOption func(*Tailer)

// WithErrorHandler reports every read failure, so a stream that stops never
// stops silently. The handler must not block.
func WithErrorHandler(fn func(error)) TailerOption {
	return func(t *Tailer) {
		if fn != nil {
			t.onError = fn
		}
	}
}

// WithRetry sets the first backoff, its ceiling, and how many consecutive
// failures are tolerated before the stream is given up as fatal. A limit of
// zero or less retries forever.
func WithRetry(backoff, max time.Duration, limit int) TailerOption {
	return func(t *Tailer) {
		t.backoff, t.maxBackoff, t.retryLimit = backoff, max, limit
	}
}

// NewTailer builds a tailer polling at the given interval.
func NewTailer(r Reader, poll time.Duration, batch int, opts ...TailerOption) *Tailer {
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	if batch <= 0 {
		batch = 256
	}
	t := &Tailer{
		reader: r, poll: poll, batch: batch,
		backoff:    DefaultRetryBackoff,
		maxBackoff: DefaultMaxBackoff,
		retryLimit: DefaultRetryLimit,
		onError:    func(error) {},
	}
	for _, opt := range opts {
		opt(t)
	}
	if t.maxBackoff < t.backoff {
		t.maxBackoff = t.backoff
	}
	return t
}

// Subscribe delivers events matching the filter until ctx is cancelled. A
// filter carrying SinceSeq replays from the durable log, so a reconnect is
// gap-free.
func (t *Tailer) Subscribe(ctx context.Context, f core.EventFilter) (<-chan core.Event, error) {
	cursor := f.SinceSeq
	if cursor < 0 {
		return nil, core.Invalid("since_seq must not be negative")
	}
	if cursor == 0 {
		latest, err := t.reader.Latest(ctx)
		if err != nil {
			return nil, err
		}
		cursor = latest
	}

	out := make(chan core.Event)
	go t.run(ctx, f, cursor, out)
	return out, nil
}

func (t *Tailer) run(ctx context.Context, f core.EventFilter, cursor int64, out chan<- core.Event) {
	defer close(out)
	ticker := time.NewTicker(t.poll)
	defer ticker.Stop()

	failures := 0
	for {
		events, err := t.reader.ReadSince(ctx, cursor, t.batch)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			failures++
			t.onError(fmt.Errorf("reading events after seq %d (attempt %d): %w", cursor, failures, err))
			if t.retryLimit > 0 && failures >= t.retryLimit {
				t.onError(fmt.Errorf("giving up on the event stream after %d consecutive read failures: %w", failures, err))
				return
			}
			if !t.wait(ctx, t.backoffFor(failures)) {
				return
			}
			continue
		}
		failures = 0
		for _, e := range events {
			cursor = e.Seq
			if !f.Matches(e) {
				continue
			}
			select {
			case out <- e:
			case <-ctx.Done():
				return
			}
		}
		if len(events) == t.batch {
			continue
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			return
		}
	}
}

// backoffFor returns the delay before retry number n, doubling to the ceiling.
func (t *Tailer) backoffFor(n int) time.Duration {
	d := t.backoff
	for range n - 1 {
		if d >= t.maxBackoff {
			break
		}
		d *= 2
	}
	if d > t.maxBackoff {
		d = t.maxBackoff
	}
	return d
}

// wait sleeps for d, reporting whether the context outlived it.
func (t *Tailer) wait(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return true
	case <-ctx.Done():
		return false
	}
}
