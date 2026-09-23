// SPDX-License-Identifier: AGPL-3.0-or-later

package outbox

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

type fakeReader struct {
	mu     sync.Mutex
	events []core.Event
}

func (f *fakeReader) add(evs ...core.Event) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, e := range evs {
		e.Seq = int64(len(f.events) + 1)
		f.events = append(f.events, e)
	}
}

func (f *fakeReader) ReadSince(_ context.Context, since int64, limit int) ([]core.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []core.Event
	for _, e := range f.events {
		if e.Seq > since {
			out = append(out, e)
		}
		if len(out) == limit {
			break
		}
	}
	return out, nil
}

func (f *fakeReader) Latest(context.Context) (int64, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return int64(len(f.events)), nil
}

func TestSubscribeDeliversNewEvents(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &fakeReader{}
	ch, err := NewTailer(r, time.Millisecond, 10).Subscribe(ctx, core.EventFilter{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"})

	select {
	case got := <-ch:
		if got.SubjectID != "t1" {
			t.Errorf("got %+v", got)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event delivered")
	}
}

// since_seq must replay from the durable log so a reconnect loses nothing.
func TestSubscribeReplaysFromCursor(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &fakeReader{}
	for _, id := range []string{"t1", "t2", "t3"} {
		r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: id})
	}

	ch, err := NewTailer(r, time.Millisecond, 10).Subscribe(ctx, core.EventFilter{SinceSeq: 1})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	var got []string
	for range 2 {
		select {
		case e := <-ch:
			got = append(got, e.SubjectID)
		case <-time.After(3 * time.Second):
			t.Fatalf("only replayed %v", got)
		}
	}
	if got[0] != "t2" || got[1] != "t3" {
		t.Errorf("replayed %v, want t2 then t3", got)
	}
}

// A subscriber starting with no cursor must not be flooded with history.
func TestSubscribeWithoutCursorStartsAtLatest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &fakeReader{}
	r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "old"})

	ch, err := NewTailer(r, time.Millisecond, 10).Subscribe(ctx, core.EventFilter{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "new"})

	select {
	case got := <-ch:
		if got.SubjectID != "new" {
			t.Errorf("delivered %q, want only events after subscription", got.SubjectID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event delivered")
	}
}

func TestSubscribeAppliesFilter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &fakeReader{}
	ch, err := NewTailer(r, time.Millisecond, 10).Subscribe(ctx,
		core.EventFilter{Types: []core.EventType{"task.claimed"}})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	r.add(
		core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "ignored"},
		core.Event{Type: core.EventTaskClaimed, SubjectType: "task", SubjectID: "wanted"},
	)

	select {
	case got := <-ch:
		if got.SubjectID != "wanted" {
			t.Errorf("filter let through %q", got.SubjectID)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("no event delivered")
	}
}

func TestSubscribeRejectsNegativeCursor(t *testing.T) {
	_, err := NewTailer(&fakeReader{}, time.Millisecond, 10).
		Subscribe(context.Background(), core.EventFilter{SinceSeq: -1})
	if !core.IsKind(err, core.KindInvalid) {
		t.Errorf("error = %v, want invalid", err)
	}
}

func TestSubscribeClosesOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	ch, err := NewTailer(&fakeReader{}, time.Millisecond, 10).Subscribe(ctx, core.EventFilter{})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel()

	select {
	case _, open := <-ch:
		if open {
			t.Error("channel delivered after cancellation")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("channel not closed after cancellation")
	}
}

func TestNewTailerDefaults(t *testing.T) {
	tl := NewTailer(&fakeReader{}, 0, 0)
	if tl.poll <= 0 || tl.batch <= 0 {
		t.Errorf("defaults not applied: poll=%v batch=%d", tl.poll, tl.batch)
	}
}

type fakeTx struct {
	core.TenantScope
	appended []core.Event
	err      error
}

func (f *fakeTx) Scope() core.TenantScope { return f.TenantScope }
func (f *fakeTx) AppendEvent(_ context.Context, e *core.Event) error {
	if f.err != nil {
		return f.err
	}
	f.appended = append(f.appended, *e)
	return nil
}

type fixedIDs struct{ v string }

func (f fixedIDs) New() string            { return f.v }
func (f fixedIDs) NewAt(time.Time) string { return f.v }

func TestAppendFillsIdentityAndTenant(t *testing.T) {
	tx := &fakeTx{TenantScope: core.TenantScope{TenantID: "t1"}}
	now := time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

	e := &core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "x1"}
	if err := appendTo(context.Background(), tx, fixedIDs{"ev1"}, now, e); err != nil {
		t.Fatalf("append: %v", err)
	}

	if e.ID != "ev1" {
		t.Errorf("ID = %q, want the generated one", e.ID)
	}
	if !e.OccurredAt.Equal(now) {
		t.Errorf("OccurredAt = %v, want %v", e.OccurredAt, now)
	}
	// The tenant comes from the transaction, never from the caller, so an event
	// cannot be attributed to a tenant the writer is not scoped to.
	if e.TenantID != "t1" {
		t.Errorf("TenantID = %q, want t1", e.TenantID)
	}
}

func TestAppendOverwritesCallerTenant(t *testing.T) {
	tx := &fakeTx{TenantScope: core.TenantScope{TenantID: "t1"}}
	e := &core.Event{
		Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "x1",
		TenantID: "someone-elses-tenant",
	}
	if err := appendTo(context.Background(), tx, fixedIDs{"ev1"}, time.Now(), e); err != nil {
		t.Fatalf("append: %v", err)
	}
	if e.TenantID != "t1" {
		t.Errorf("TenantID = %q, want the transaction's tenant", e.TenantID)
	}
}

func TestAppendRejectsIncompleteEvents(t *testing.T) {
	tx := &fakeTx{TenantScope: core.TenantScope{TenantID: "t1"}}
	tests := []struct {
		name string
		ev   core.Event
	}{
		{"no type", core.Event{SubjectType: "task", SubjectID: "x"}},
		{"no subject type", core.Event{Type: core.EventTaskCreated, SubjectID: "x"}},
		{"no subject id", core.Event{Type: core.EventTaskCreated, SubjectType: "task"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ev := tt.ev
			if err := appendTo(context.Background(), tx, fixedIDs{"ev1"}, time.Now(), &ev); err == nil {
				t.Error("expected an error")
			}
		})
	}
}

// flakyReader fails its first failures reads, then behaves.
type flakyReader struct {
	fakeReader

	failMu   sync.Mutex
	failures int
	seen     int
	always   bool
	err      error
}

func (f *flakyReader) ReadSince(ctx context.Context, since int64, limit int) ([]core.Event, error) {
	f.failMu.Lock()
	fail := f.always || f.seen < f.failures
	f.seen++
	f.failMu.Unlock()
	if fail {
		return nil, f.err
	}
	return f.fakeReader.ReadSince(ctx, since, limit)
}

func (f *flakyReader) reads() int {
	f.failMu.Lock()
	defer f.failMu.Unlock()
	return f.seen
}

// A read failure is usually a busy database, not a dead one. Returning on the
// first error left the tenant with no reader and nothing to restart it, so the
// stream went quiet forever with no error anywhere.
func TestSubscribeRetriesATransientReadFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &flakyReader{failures: 3, err: errors.New("database is locked")}
	r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t1"})

	reported := make(chan error, 8)
	ch, err := NewTailer(r, time.Millisecond, 10,
		WithRetry(0, 0, 0),
		WithErrorHandler(func(err error) { reported <- err }),
	).Subscribe(ctx, core.EventFilter{SinceSeq: 0})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	r.add(core.Event{Type: core.EventTaskCreated, SubjectType: "task", SubjectID: "t2"})

	select {
	case got := <-ch:
		if got.SubjectID != "t2" {
			t.Errorf("delivered %q, want the event committed after the failures", got.SubjectID)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream never recovered from a transient read failure")
	}

	if len(reported) == 0 {
		t.Error("a read failure was swallowed instead of reported")
	}
	for range len(reported) {
		if err := <-reported; err == nil || !strings.Contains(err.Error(), "database is locked") {
			t.Errorf("reported error = %v, want the reader's cause", err)
		}
	}
}

// A reader that never recovers must give up loudly and close its channel,
// rather than spinning or dying in silence.
func TestSubscribeGivesUpLoudlyOnAPermanentFailure(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r := &flakyReader{always: true, err: errors.New("schema is missing")}
	reported := make(chan error, 16)
	ch, err := NewTailer(r, time.Millisecond, 10,
		WithRetry(0, 0, 4),
		WithErrorHandler(func(err error) { reported <- err }),
	).Subscribe(ctx, core.EventFilter{SinceSeq: 1})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("a failing reader delivered an event")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stream neither recovered nor gave up")
	}

	if got := r.reads(); got != 4 {
		t.Errorf("reads before giving up = %d, want 4", got)
	}
	var fatal bool
	close(reported)
	var count int
	for err := range reported {
		count++
		if strings.Contains(err.Error(), "giving up") {
			fatal = true
		}
	}
	if count == 0 {
		t.Error("the failures were never reported")
	}
	if !fatal {
		t.Error("giving up on the stream was never reported")
	}
}

// Backoff grows per consecutive failure and stops at the ceiling.
func TestTailerBackoffGrowsToItsCeiling(t *testing.T) {
	tl := NewTailer(&fakeReader{}, time.Millisecond, 10, WithRetry(time.Second, 4*time.Second, 5))
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second, 4 * time.Second}
	for i, w := range want {
		if got := tl.backoffFor(i + 1); got != w {
			t.Errorf("backoffFor(%d) = %v, want %v", i+1, got, w)
		}
	}
}

func TestTailerRetryDefaults(t *testing.T) {
	tl := NewTailer(&fakeReader{}, 0, 0)
	if tl.retryLimit != DefaultRetryLimit || tl.backoff != DefaultRetryBackoff || tl.maxBackoff != DefaultMaxBackoff {
		t.Errorf("retry defaults = %d/%v/%v", tl.retryLimit, tl.backoff, tl.maxBackoff)
	}
	if tl.onError == nil {
		t.Error("a tailer must always have an error handler")
	}
	tl.onError(errors.New("boom"))
}

// A cancelled context ends the stream without reporting the cancellation as a
// read failure.
func TestSubscribeStopsQuietlyWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := &flakyReader{always: true, err: context.Canceled}
	reported := make(chan error, 4)
	ch, err := NewTailer(r, time.Millisecond, 10,
		WithRetry(time.Hour, time.Hour, 0),
		WithErrorHandler(func(err error) { reported <- err }),
	).Subscribe(ctx, core.EventFilter{SinceSeq: 1})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}
	cancel()
	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("a cancelled stream delivered an event")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelling the context did not end the stream")
	}
}
