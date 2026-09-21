// Package outbox appends domain events and replays them to subscribers.
package outbox

import (
	"context"
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

// Tailer streams committed events from a cursor.
type Tailer struct {
	reader Reader
	poll   time.Duration
	batch  int
}

// NewTailer builds a tailer polling at the given interval.
func NewTailer(r Reader, poll time.Duration, batch int) *Tailer {
	if poll <= 0 {
		poll = 250 * time.Millisecond
	}
	if batch <= 0 {
		batch = 256
	}
	return &Tailer{reader: r, poll: poll, batch: batch}
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

	for {
		events, err := t.reader.ReadSince(ctx, cursor, t.batch)
		if err != nil {
			return
		}
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
