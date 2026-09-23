// SPDX-License-Identifier: AGPL-3.0-or-later

package webhook

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

type captured struct {
	header http.Header
	body   []byte
}

type receiver struct {
	*httptest.Server
	mu     sync.Mutex
	got    []captured
	status int
	seen   chan string
}

func newReceiver(t *testing.T) *receiver {
	t.Helper()
	r := &receiver{status: http.StatusOK, seen: make(chan string, 64)}
	r.Server = httptest.NewServer(http.HandlerFunc(r.handle))
	t.Cleanup(r.Close)
	return r
}

func (r *receiver) handle(w http.ResponseWriter, req *http.Request) {
	body, _ := io.ReadAll(req.Body)
	r.mu.Lock()
	r.got = append(r.got, captured{header: req.Header.Clone(), body: body})
	status := r.status
	r.mu.Unlock()
	select {
	case r.seen <- req.Header.Get(HeaderDelivery):
	default:
	}
	w.WriteHeader(status)
}

func (r *receiver) setStatus(code int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.status = code
}

func (r *receiver) calls() []captured {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]captured(nil), r.got...)
}

func (r *receiver) count() int { return len(r.calls()) }

func TestDeliverySucceedsAndIsSigned(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	endpoint := f.addEndpoint(t, rec.URL, "top-secret", true, "task.*")
	event, queued := f.emit(t, core.EventTaskCreated)
	if queued != 1 {
		t.Fatalf("queued = %d, want 1", queued)
	}
	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()

	res, err := d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("draining: %v", err)
	}
	if res.Attempted != 1 || res.Delivered != 1 || res.Failed != 0 {
		t.Fatalf("result = %+v", res)
	}

	calls := rec.calls()
	if len(calls) != 1 {
		t.Fatalf("requests = %d, want 1", len(calls))
	}
	got := calls[0]
	if got.header.Get(HeaderEvent) != string(core.EventTaskCreated) {
		t.Fatalf("event header = %q", got.header.Get(HeaderEvent))
	}
	row := f.only(t)
	if got.header.Get(HeaderDelivery) != row.ID {
		t.Fatalf("delivery header = %q, want %q", got.header.Get(HeaderDelivery), row.ID)
	}
	if got.header.Get("Content-Type") != contentTypeJSON {
		t.Fatalf("content type = %q", got.header.Get("Content-Type"))
	}

	timestamp := got.header.Get(HeaderTimestamp)
	if _, err := time.Parse(TimestampLayout, timestamp); err != nil {
		t.Fatalf("timestamp header %q: %v", timestamp, err)
	}
	if !Verify(endpoint.Secret, timestamp, got.body, got.header.Get(HeaderSignature)) {
		t.Fatal("signature does not verify against the exact body and timestamp")
	}
	if Verify(endpoint.Secret, timestamp, append(got.body, ' '), got.header.Get(HeaderSignature)) {
		t.Fatal("a tampered body verified")
	}
	if Verify(endpoint.Secret, FormatTimestamp(f.clk.Now().Add(time.Second)), got.body, got.header.Get(HeaderSignature)) {
		t.Fatal("a replayed request verified under a new timestamp")
	}
	for _, v := range got.header {
		if strings.Contains(strings.Join(v, " "), endpoint.Secret) {
			t.Fatal("the signing secret was sent in a header")
		}
	}

	var sent core.Event
	if err := json.Unmarshal(got.body, &sent); err != nil {
		t.Fatalf("body is not a json event: %v", err)
	}
	if sent.Seq != event.Seq || sent.Type != event.Type {
		t.Fatalf("body event = %+v, want seq %d of type %q", sent, event.Seq, event.Type)
	}

	row = f.delivery(t, row.ID)
	if row.Status != core.DeliveryDelivered || row.Attempts != 1 || row.LastStatusCode != http.StatusOK {
		t.Fatalf("delivery = %+v", row)
	}

	res, err = d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("second drain: %v", err)
	}
	if res.Attempted != 0 || rec.count() != 1 {
		t.Fatalf("a delivered row was attempted again: %+v", res)
	}
}

func TestServerErrorSchedulesRetry(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	rec.setStatus(http.StatusInternalServerError)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	res, err := d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("draining: %v", err)
	}
	if res.Failed != 1 || res.Delivered != 0 {
		t.Fatalf("result = %+v", res)
	}

	row := f.only(t)
	want := f.clk.Now().Add(time.Second)
	if row.Status != core.DeliveryPending || row.Attempts != 1 {
		t.Fatalf("delivery = %+v, want one pending attempt", row)
	}
	if !row.NextAttemptAt.Equal(want) {
		t.Fatalf("next attempt at %v, want %v", row.NextAttemptAt, want)
	}
	if row.LastStatusCode != http.StatusInternalServerError || row.LastError == "" {
		t.Fatalf("delivery outcome = %+v", row)
	}

	res, err = d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("early drain: %v", err)
	}
	if res.Attempted != 0 || rec.count() != 1 {
		t.Fatalf("a delivery was retried before it was due: %+v", res)
	}

	f.clk.Advance(time.Second)
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("due drain: %v", err)
	}
	if rec.count() != 2 {
		t.Fatalf("requests = %d, want 2 once the backoff elapsed", rec.count())
	}
	if again := f.only(t); again.Attempts != 2 {
		t.Fatalf("attempts = %d, want 2", again.Attempts)
	}
}

func TestScheduleRunsOutAndFails(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	rec.setStatus(http.StatusBadGateway)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	schedule := d.Schedule()

	for step, backoff := range schedule {
		if _, err := d.Drain(ctx, DefaultBound()); err != nil {
			t.Fatalf("drain at step %d: %v", step, err)
		}
		row := f.only(t)
		if row.Attempts != step+1 {
			t.Fatalf("attempts after step %d = %d", step, row.Attempts)
		}
		if row.Status != core.DeliveryPending {
			t.Fatalf("status after step %d = %q, want pending", step, row.Status)
		}
		if want := f.clk.Now().Add(backoff); !row.NextAttemptAt.Equal(want) {
			t.Fatalf("next attempt after step %d = %v, want %v", step, row.NextAttemptAt, want)
		}
		f.clk.Advance(backoff)
	}

	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("final drain: %v", err)
	}
	row := f.only(t)
	if row.Status != core.DeliveryFailed {
		t.Fatalf("status after the last attempt = %q, want failed", row.Status)
	}
	if row.Attempts != schedule.MaxAttempts() {
		t.Fatalf("attempts = %d, want %d", row.Attempts, schedule.MaxAttempts())
	}
	sent := rec.count()

	f.clk.Advance(24 * time.Hour)
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("drain after failure: %v", err)
	}
	if rec.count() != sent {
		t.Fatal("a terminally failed delivery was retried automatically")
	}
}

func TestConcurrentDispatchersDoNotDoubleDeliver(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	for i := 0; i < 6; i++ {
		f.emit(t, core.EventTaskCreated)
	}

	first := NewDispatcher(f.store, f.scope, f.clk, WithOwner("one"))
	second := NewDispatcher(f.store, f.scope, f.clk, WithOwner("two"))
	defer first.Stop()
	defer second.Stop()

	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, d := range []*Dispatcher{first, second} {
		wg.Add(1)
		go func(d *Dispatcher) {
			defer wg.Done()
			if _, err := d.Drain(ctx, Bound{MaxDeliveries: 6}); err != nil {
				errs <- err
			}
		}(d)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent drain: %v", err)
	}

	counts := map[string]int{}
	for _, c := range rec.calls() {
		counts[c.header.Get(HeaderDelivery)]++
	}
	if len(counts) != 6 {
		t.Fatalf("distinct deliveries attempted = %d, want 6", len(counts))
	}
	for id, n := range counts {
		if n != 1 {
			t.Fatalf("delivery %q was attempted %d times", id, n)
		}
	}
	for _, row := range f.deliveries(t) {
		if row.Status != core.DeliveryDelivered {
			t.Fatalf("delivery %q = %q, want delivered", row.ID, row.Status)
		}
	}
}

func TestInactiveEndpointReceivesNothing(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	endpoint := f.addEndpoint(t, rec.URL, "s", true, "*")
	f.emit(t, core.EventTaskCreated)
	f.setActive(t, endpoint, false)

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("draining: %v", err)
	}
	if rec.count() != 0 {
		t.Fatal("an inactive endpoint was posted to")
	}
	row := f.only(t)
	if row.Status != core.DeliveryFailed || row.LastError != "endpoint is inactive" {
		t.Fatalf("delivery = %+v", row)
	}

	_, queued := f.emit(t, core.EventTaskUpdated)
	if queued != 0 {
		t.Fatalf("queued = %d for an inactive endpoint, want 0", queued)
	}
}

func TestMissingEventFailsTerminally(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	endpoint := f.addEndpoint(t, rec.URL, "s", true, "*")

	var orphan core.WebhookDelivery
	if err := f.store.Update(ctx, f.scope, func(tx store.Tx) error {
		orphan = core.WebhookDelivery{EndpointID: endpoint.ID, EventSeq: 4242}
		return tx.EnqueueDelivery(ctx, &orphan)
	}); err != nil {
		t.Fatalf("enqueueing an orphan delivery: %v", err)
	}

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("draining: %v", err)
	}
	row := f.delivery(t, orphan.ID)
	if row.Status != core.DeliveryFailed || row.LastError != "event is no longer available" {
		t.Fatalf("delivery = %+v", row)
	}
	if rec.count() != 0 {
		t.Fatal("a delivery with no event was posted")
	}
}

func TestTimeoutIsAFailure(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-release
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	t.Cleanup(func() { close(release) })

	f.addEndpoint(t, srv.URL, "top-secret", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk, WithTimeout(50*time.Millisecond))
	defer d.Stop()
	res, err := d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("draining: %v", err)
	}
	if res.Failed != 1 {
		t.Fatalf("result = %+v, want one failure", res)
	}
	row := f.only(t)
	if row.Status != core.DeliveryPending || row.Attempts != 1 || row.LastStatusCode != 0 {
		t.Fatalf("delivery = %+v", row)
	}
	if row.LastError == "" || strings.Contains(row.LastError, "top-secret") {
		t.Fatalf("last error = %q", row.LastError)
	}
	if !row.NextAttemptAt.Equal(f.clk.Now().Add(time.Second)) {
		t.Fatalf("next attempt at %v", row.NextAttemptAt)
	}
}

func TestUnreachableTargetKeepsTheSecretOut(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	url := rec.URL
	rec.Close()
	f.addEndpoint(t, url, "top-secret", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("draining: %v", err)
	}
	row := f.only(t)
	if row.LastError == "" || strings.Contains(row.LastError, "top-secret") {
		t.Fatalf("last error = %q", row.LastError)
	}
	if len(row.LastError) > maxErrorLength {
		t.Fatalf("last error is %d bytes", len(row.LastError))
	}
}

// advancingTransport moves the fake clock on every request, so a time budget
// can be exercised without waiting for real time.
type advancingTransport struct {
	clk  *clock.Fake
	step time.Duration
	next http.RoundTripper
}

func (a advancingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := a.next.RoundTrip(req)
	a.clk.Advance(a.step)
	return resp, err
}

func TestInlineDrainIsBounded(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	for i := 0; i < 5; i++ {
		f.emit(t, core.EventTaskCreated)
	}

	byCount := NewDispatcher(f.store, f.scope, f.clk)
	defer byCount.Stop()
	res, err := byCount.Drain(ctx, Bound{MaxDeliveries: 2})
	if err != nil {
		t.Fatalf("bounded drain: %v", err)
	}
	if res.Attempted != 2 || rec.count() != 2 {
		t.Fatalf("count bound not honoured: %+v, %d requests", res, rec.count())
	}

	byTime := NewDispatcher(f.store, f.scope, f.clk, WithHTTPClient(&http.Client{
		Transport: advancingTransport{clk: f.clk, step: time.Second, next: http.DefaultTransport},
	}))
	defer byTime.Stop()
	res, err = byTime.Drain(ctx, Bound{Budget: time.Second})
	if err != nil {
		t.Fatalf("budgeted drain: %v", err)
	}
	if res.Attempted != 1 {
		t.Fatalf("budget bound not honoured: %+v", res)
	}

	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	res, err = byCount.Drain(cancelled, DefaultBound())
	if err != nil || res.Attempted != 0 {
		t.Fatalf("cancelled drain = %+v, %v; a command must still exit", res, err)
	}
}

func TestDrainForMode(t *testing.T) {
	ctx := context.Background()
	cases := []struct {
		mode Mode
		want int
	}{
		{ModeOff, 0},
		{ModeServer, 0},
		{ModeInline, 1},
	}
	for _, c := range cases {
		t.Run(string(c.mode), func(t *testing.T) {
			f := newFixture(t)
			rec := newReceiver(t)
			f.addEndpoint(t, rec.URL, "s", true, "*")
			f.emit(t, core.EventTaskCreated)

			d := NewDispatcher(f.store, f.scope, f.clk)
			defer d.Stop()
			res, err := d.DrainFor(ctx, c.mode, DefaultBound())
			if err != nil {
				t.Fatalf("draining in %q mode: %v", c.mode, err)
			}
			if res.Attempted != c.want || rec.count() != c.want {
				t.Fatalf("mode %q attempted %d requests, want %d", c.mode, rec.count(), c.want)
			}
			if c.want == 0 && f.only(t).Status != core.DeliveryPending {
				t.Fatalf("mode %q did not leave the delivery pending", c.mode)
			}
		})
	}
}

func TestRedeliverQueuesAnotherAttempt(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("first drain: %v", err)
	}
	first := f.only(t)
	if first.Status != core.DeliveryDelivered {
		t.Fatalf("delivery = %+v", first)
	}

	again, err := d.Redeliver(ctx, first.ID)
	if err != nil {
		t.Fatalf("redelivering: %v", err)
	}
	if again.ID != first.ID || again.Status != core.DeliveryPending {
		t.Fatalf("redelivery = %+v", again)
	}
	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("second drain: %v", err)
	}
	calls := rec.calls()
	if len(calls) != 2 {
		t.Fatalf("requests = %d, want 2", len(calls))
	}
	if calls[0].header.Get(HeaderDelivery) != calls[1].header.Get(HeaderDelivery) {
		t.Fatal("a redelivery changed the delivery identifier")
	}
	if string(calls[0].body) != string(calls[1].body) {
		t.Fatal("a redelivery changed the event payload")
	}
	if f.only(t).Status != core.DeliveryDelivered {
		t.Fatal("the redelivery was not recorded")
	}

	if _, err := d.Redeliver(ctx, "missing"); !core.IsKind(err, core.KindNotFound) {
		t.Fatalf("redelivering an unknown delivery = %v, want not found", err)
	}
}

func TestBacklogReport(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	rec := newReceiver(t)
	f.addEndpoint(t, rec.URL, "s", true, "*")

	d := NewDispatcher(f.store, f.scope, f.clk)
	defer d.Stop()
	empty, err := d.Backlog(ctx)
	if err != nil {
		t.Fatalf("reporting an empty backlog: %v", err)
	}
	if !empty.Empty() || empty.Pending != 0 || empty.OldestAge != 0 {
		t.Fatalf("backlog = %+v, want empty", empty)
	}

	f.emit(t, core.EventTaskCreated)
	oldest := f.clk.Now()
	f.clk.Advance(time.Hour)
	f.emit(t, core.EventTaskUpdated)

	got, err := d.Backlog(ctx)
	if err != nil {
		t.Fatalf("reporting the backlog: %v", err)
	}
	if got.Pending != 2 || got.Empty() {
		t.Fatalf("backlog = %+v, want two pending", got)
	}
	if !got.OldestPendingAt.Equal(oldest) || got.OldestAge != time.Hour {
		t.Fatalf("oldest pending = %v aged %v", got.OldestPendingAt, got.OldestAge)
	}

	if _, err := d.Drain(ctx, DefaultBound()); err != nil {
		t.Fatalf("draining the backlog: %v", err)
	}
	drained, err := d.Backlog(ctx)
	if err != nil {
		t.Fatalf("reporting after the drain: %v", err)
	}
	if !drained.Empty() {
		t.Fatalf("backlog after draining = %+v", drained)
	}
}

func TestRunDrainsOnEveryTick(t *testing.T) {
	f := newFixture(t)
	rec := newReceiver(t)
	f.addEndpoint(t, rec.URL, "s", true, "*")
	f.emit(t, core.EventTaskCreated)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	d := NewDispatcher(f.store, f.scope, f.clk, WithInterval(time.Second), WithBatch(4))
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()

	f.clk.Advance(time.Second)
	if id := <-rec.seen; id == "" {
		t.Fatal("a delivery arrived without its identifier")
	}

	f.emit(t, core.EventTaskUpdated)
	f.clk.Advance(time.Second)
	<-rec.seen

	cancel()
	if err := <-done; err == nil {
		t.Fatal("Run should report why it stopped")
	}
}

func TestRunWithoutIntervalWaitsForCancellation(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	d := NewDispatcher(f.store, f.scope, f.clk, WithInterval(0))
	done := make(chan error, 1)
	go func() { done <- d.Run(ctx) }()
	cancel()
	if err := <-done; err == nil {
		t.Fatal("Run should report why it stopped")
	}
	d.Stop()
}

func TestDispatcherOptionsAndDefaults(t *testing.T) {
	f := newFixture(t)
	d := NewDispatcher(f.store, f.scope, nil, WithOwner(""), WithLease(0), WithBatch(0),
		WithTimeout(0), WithSchedule(nil), WithHTTPClient(nil), WithErrorHandler(func(error) {}))
	defer d.Stop()
	if d.Owner() == "" {
		t.Fatal("a dispatcher must name itself in the lock")
	}
	if len(d.Schedule()) != len(DefaultSchedule()) {
		t.Fatalf("schedule = %v", d.Schedule())
	}
	if d.lease != DefaultLease || d.batch != DefaultBatch || d.client.Timeout != DefaultTimeout {
		t.Fatalf("defaults were overwritten by empty options: %v %d %v", d.lease, d.batch, d.client.Timeout)
	}
	if d.clk == nil {
		t.Fatal("a nil clock must fall back to the real one")
	}
}

// A stored endpoint that points inside the network is refused at delivery as
// well as at registration, so an endpoint registered before the guard existed,
// or one whose name has since been pointed inward, never reaches the address.
func TestDeliveryToABlockedTargetIsRefusedAndSaysNothingAboutTheNetwork(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)
	f.addEndpoint(t, "http://169.254.169.254/latest/meta-data/", "top-secret", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk, WithGuard(Guard{}))
	defer d.Stop()
	res, err := d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("draining: %v", err)
	}
	if res.Delivered != 0 || res.Failed != 1 {
		t.Fatalf("result = %+v, want the attempt to have failed", res)
	}
	row := f.only(t)
	if row.LastError != "endpoint address is not permitted" {
		t.Errorf("last error = %q, want the generic refusal", row.LastError)
	}
	for _, leak := range []string{"169.254", "meta-data", "top-secret"} {
		if strings.Contains(row.LastError, leak) {
			t.Errorf("last error leaks %q: %q", leak, row.LastError)
		}
	}
}

// A receiver that answers a delivery with a redirect is not followed, so the
// signature headers never reach a host that passed no validation.
func TestDeliveryDoesNotFollowARedirect(t *testing.T) {
	ctx := context.Background()
	f := newFixture(t)

	var hops int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hops, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer target.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/latest/meta-data/", http.StatusFound)
	}))
	defer redirector.Close()

	f.addEndpoint(t, redirector.URL, "top-secret", true, "*")
	f.emit(t, core.EventTaskCreated)

	d := NewDispatcher(f.store, f.scope, f.clk, WithGuard(Guard{AllowPrivate: true}))
	defer d.Stop()
	res, err := d.Drain(ctx, DefaultBound())
	if err != nil {
		t.Fatalf("draining: %v", err)
	}
	if res.Delivered != 0 || res.Failed != 1 {
		t.Fatalf("result = %+v, want the redirect treated as a failed delivery", res)
	}
	if n := atomic.LoadInt32(&hops); n != 0 {
		t.Fatalf("the redirect target was posted to %d times", n)
	}
	row := f.only(t)
	if row.LastStatusCode != http.StatusFound {
		t.Errorf("last status = %d, want the redirect itself recorded", row.LastStatusCode)
	}
}

// A client supplied for another reason does not silently reopen redirects.
func TestAReplacementClientKeepsTheRedirectPolicy(t *testing.T) {
	f := newFixture(t)
	d := NewDispatcher(f.store, f.scope, f.clk, WithHTTPClient(&http.Client{}))
	defer d.Stop()
	if d.client.CheckRedirect == nil {
		t.Fatal("a replacement client was installed with no redirect policy")
	}
}
