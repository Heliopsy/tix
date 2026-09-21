package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/id"
	"github.com/heliopsy/tix/internal/store"
	sqlb "github.com/heliopsy/tix/internal/store/sql"
)

// Dispatcher defaults.
const (
	DefaultTimeout   = 10 * time.Second
	DefaultLease     = 30 * time.Second
	DefaultBatch     = 16
	DefaultInterval  = time.Second
	maxErrorLength   = 512
	maxBacklogPages  = 1000
	contentTypeJSON  = "application/json"
	userAgentDefault = "tix-webhook/1"
)

// Result counts the outcome of one drain.
type Result struct {
	Attempted int `json:"attempted" yaml:"attempted"`
	Delivered int `json:"delivered" yaml:"delivered"`
	Failed    int `json:"failed" yaml:"failed"`
}

func (r *Result) add(other Result) {
	r.Attempted += other.Attempted
	r.Delivered += other.Delivered
	r.Failed += other.Failed
}

// Bound limits an opportunistic drain so a short-lived process still exits.
type Bound struct {
	MaxDeliveries int           `json:"max_deliveries" yaml:"max_deliveries"`
	Budget        time.Duration `json:"budget" yaml:"budget"`
}

// DefaultBound is the bound an inline drain uses when none is configured.
func DefaultBound() Bound { return Bound{MaxDeliveries: 8, Budget: 2 * time.Second} }

// Backlog reports the depth and age of the pending queue.
type Backlog struct {
	Pending         int           `json:"pending" yaml:"pending"`
	OldestPendingAt time.Time     `json:"oldest_pending_at,omitempty" yaml:"oldest_pending_at,omitempty"`
	OldestAge       time.Duration `json:"oldest_age,omitempty" yaml:"oldest_age,omitempty"`
}

// Empty reports whether nothing is waiting to be delivered.
func (b Backlog) Empty() bool { return b.Pending == 0 }

// Dispatcher claims pending deliveries and posts them to their endpoints.
type Dispatcher struct {
	store    store.Store
	scope    core.TenantScope
	clk      clock.Clock
	client   *http.Client
	schedule Schedule
	owner    string
	lease    time.Duration
	batch    int
	ticker   clock.Ticker
	onError  func(error)
}

// Option configures a Dispatcher.
type Option func(*Dispatcher)

// WithHTTPClient replaces the client used for delivery.
func WithHTTPClient(c *http.Client) Option {
	return func(d *Dispatcher) {
		if c != nil {
			d.client = c
		}
	}
}

// WithTimeout bounds how long one delivery attempt may take.
func WithTimeout(t time.Duration) Option {
	return func(d *Dispatcher) {
		if t > 0 {
			d.client.Timeout = t
		}
	}
}

// WithSchedule replaces the retry backoff.
func WithSchedule(s Schedule) Option {
	return func(d *Dispatcher) {
		if len(s) > 0 {
			d.schedule = s
		}
	}
}

// WithOwner names this dispatcher in the delivery lock.
func WithOwner(owner string) Option {
	return func(d *Dispatcher) {
		if owner != "" {
			d.owner = owner
		}
	}
}

// WithLease sets how long a claimed delivery stays locked to this dispatcher.
func WithLease(l time.Duration) Option {
	return func(d *Dispatcher) {
		if l > 0 {
			d.lease = l
		}
	}
}

// WithBatch sets how many deliveries one server-mode pass claims.
func WithBatch(n int) Option {
	return func(d *Dispatcher) {
		if n > 0 {
			d.batch = n
		}
	}
}

// WithErrorHandler reports a failed pass. A failed pass never stops the loop.
func WithErrorHandler(fn func(error)) Option { return func(d *Dispatcher) { d.onError = fn } }

// WithInterval arms the server-mode ticker. A non-positive interval leaves
// Run idle, so only on-demand drains happen.
func WithInterval(i time.Duration) Option {
	return func(d *Dispatcher) {
		if d.ticker != nil {
			d.ticker.Stop()
			d.ticker = nil
		}
		if i > 0 {
			d.ticker = d.clk.NewTicker(i)
		}
	}
}

// NewDispatcher builds a dispatcher for one tenant's queue.
func NewDispatcher(s store.Store, scope core.TenantScope, clk clock.Clock, opts ...Option) *Dispatcher {
	if clk == nil {
		clk = clock.New()
	}
	d := &Dispatcher{
		store:    s,
		scope:    scope,
		clk:      clk,
		client:   &http.Client{Timeout: DefaultTimeout},
		schedule: DefaultSchedule(),
		owner:    id.New(),
		lease:    DefaultLease,
		batch:    DefaultBatch,
	}
	d.ticker = clk.NewTicker(DefaultInterval)
	for _, opt := range opts {
		opt(d)
	}
	return d
}

// Owner returns the name this dispatcher writes into the delivery lock.
func (d *Dispatcher) Owner() string { return d.owner }

// Schedule returns the retry backoff in force.
func (d *Dispatcher) Schedule() Schedule { return d.schedule }

// Stop releases the server-mode ticker.
func (d *Dispatcher) Stop() {
	if d.ticker != nil {
		d.ticker.Stop()
	}
}

// job is one claimed delivery with everything the attempt needs.
type job struct {
	delivery         core.WebhookDelivery
	endpoint         core.WebhookEndpoint
	event            core.Event
	outcomeDelivered bool
}

// Run drains on every tick until ctx is cancelled. A failed pass is reported
// and the loop continues, so delivery never takes the server down with it.
func (d *Dispatcher) Run(ctx context.Context) error {
	if d.ticker == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	defer d.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-d.ticker.C():
			if _, err := d.DrainOnce(ctx); err != nil && d.onError != nil {
				d.onError(err)
			}
		}
	}
}

// DrainOnce claims one batch of due deliveries and attempts each of them.
func (d *Dispatcher) DrainOnce(ctx context.Context) (Result, error) {
	res, _, err := d.pass(ctx, d.batch)
	return res, err
}

// Drain attempts due deliveries until the bound is spent or the queue is
// empty. It is what a command-line process runs after its own commit.
func (d *Dispatcher) Drain(ctx context.Context, b Bound) (Result, error) {
	var res Result
	start := d.clk.Now()
	for n := 0; b.MaxDeliveries <= 0 || n < b.MaxDeliveries; n++ {
		select {
		case <-ctx.Done():
			return res, nil
		default:
		}
		if b.Budget > 0 && d.clk.Since(start) >= b.Budget {
			return res, nil
		}
		pass, claimed, err := d.pass(ctx, 1)
		res.add(pass)
		if err != nil {
			return res, err
		}
		if claimed == 0 {
			return res, nil
		}
	}
	return res, nil
}

// DrainFor runs the drain the configured mode calls for, which for server and
// off modes is none at all.
func (d *Dispatcher) DrainFor(ctx context.Context, m Mode, b Bound) (Result, error) {
	if !m.DrainsInline() {
		return Result{}, nil
	}
	return d.Drain(ctx, b)
}

// pass claims up to limit deliveries and attempts each outside the claiming
// transaction, so no database lock is held across the network.
func (d *Dispatcher) pass(ctx context.Context, limit int) (Result, int, error) {
	jobs, err := d.claim(ctx, limit)
	if err != nil {
		return Result{}, 0, err
	}
	var res Result
	for _, j := range jobs {
		res.Attempted++
		if err := d.attempt(ctx, j); err != nil {
			return res, len(jobs), err
		}
		if j.outcomeDelivered {
			res.Delivered++
			continue
		}
		res.Failed++
	}
	return res, len(jobs), nil
}

// claim locks due deliveries and loads what each attempt needs. A delivery
// whose endpoint or event is gone can never succeed and is failed here.
func (d *Dispatcher) claim(ctx context.Context, limit int) ([]*job, error) {
	var jobs []*job
	err := d.store.Update(ctx, d.scope, func(tx store.Tx) error {
		now := d.clk.Now()
		claimed, err := tx.ClaimDeliveries(ctx, d.owner, now, now.Add(d.lease), limit)
		if err != nil {
			return err
		}
		for _, delivery := range claimed {
			j, reason := d.load(ctx, tx, delivery)
			if reason != "" {
				if err := tx.MarkFailed(ctx, delivery.ID, 0, reason, now, true); err != nil {
					return err
				}
				continue
			}
			jobs = append(jobs, j)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// load reads the endpoint and event behind a delivery, reporting why the
// delivery can never be attempted when that is the case.
func (d *Dispatcher) load(ctx context.Context, tx store.Tx, delivery core.WebhookDelivery) (*job, string) {
	endpoint, err := tx.GetWebhook(ctx, delivery.EndpointID)
	if err != nil {
		return nil, "endpoint is no longer registered"
	}
	if !endpoint.Active {
		return nil, "endpoint is inactive"
	}
	events, err := tx.ReadEvents(ctx, delivery.EventSeq-1, 1)
	if err != nil || len(events) == 0 || events[0].Seq != delivery.EventSeq {
		return nil, "event is no longer available"
	}
	return &job{delivery: delivery, endpoint: *endpoint, event: events[0]}, ""
}

// attempt posts one delivery and records its outcome.
func (d *Dispatcher) attempt(ctx context.Context, j *job) error {
	body, err := json.Marshal(j.event)
	if err != nil {
		return core.Internal("encoding event %d for delivery", j.event.Seq).Wrap(err)
	}
	code, sendErr := d.post(ctx, j, body)
	now := d.clk.Now()
	if sendErr == nil && code >= 200 && code < 300 {
		j.outcomeDelivered = true
		return d.store.Update(ctx, d.scope, func(tx store.Tx) error {
			return tx.MarkDelivered(ctx, j.delivery.ID, code, now)
		})
	}

	attempts := j.delivery.Attempts + 1
	next, retry := d.schedule.NextAttemptAt(attempts, now)
	reason := responseError(code)
	if sendErr != nil {
		reason = transportError(sendErr)
	}
	return d.store.Update(ctx, d.scope, func(tx store.Tx) error {
		return tx.MarkFailed(ctx, j.delivery.ID, code, reason, next, !retry)
	})
}

// post sends the signed request, returning the response status where one
// arrived. The signing secret is never placed in a header value that is not
// the signature, nor in any error this returns.
func (d *Dispatcher) post(ctx context.Context, j *job, body []byte) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, j.endpoint.URL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}
	timestamp := FormatTimestamp(d.clk.Now())
	req.Header.Set("Content-Type", contentTypeJSON)
	req.Header.Set("User-Agent", userAgentDefault)
	req.Header.Set(HeaderEvent, string(j.event.Type))
	req.Header.Set(HeaderDelivery, j.delivery.ID)
	req.Header.Set(HeaderTimestamp, timestamp)
	req.Header.Set(HeaderSignature, Sign(j.endpoint.Secret, timestamp, body))

	resp, err := d.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode, nil
}

// Redeliver queues a fresh attempt of an existing delivery against the same
// endpoint and the same event. The attempt itself happens on the next drain.
func (d *Dispatcher) Redeliver(ctx context.Context, deliveryID string) (*core.WebhookDelivery, error) {
	var out *core.WebhookDelivery
	err := d.store.Update(ctx, d.scope, func(tx store.Tx) error {
		existing, err := tx.GetDelivery(ctx, deliveryID)
		if err != nil {
			return err
		}
		if err := tx.MarkFailed(ctx, existing.ID, 0, "", d.clk.Now(), false); err != nil {
			return err
		}
		out, err = tx.GetDelivery(ctx, existing.ID)
		return err
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// Backlog reports how many deliveries are pending and how old the oldest is.
func (d *Dispatcher) Backlog(ctx context.Context) (Backlog, error) {
	var b Backlog
	err := d.store.View(ctx, d.scope, func(tx store.Tx) error {
		cursor := ""
		for page := 0; page < maxBacklogPages; page++ {
			rows, err := tx.ListDeliveries(ctx, core.DeliveryFilter{
				Statuses: []core.DeliveryStatus{core.DeliveryPending},
				Page: core.Page{
					Limit:     core.MaxPageLimit,
					Cursor:    cursor,
					Sort:      "created_at",
					Direction: core.Ascending,
				},
			})
			if err != nil {
				return err
			}
			for _, row := range rows {
				if b.Pending == 0 {
					b.OldestPendingAt = row.CreatedAt
				}
				b.Pending++
			}
			if len(rows) < core.MaxPageLimit {
				break
			}
			last := rows[len(rows)-1]
			cursor = core.Cursor{
				SortValue: sqlb.TimeText(last.CreatedAt),
				ID:        last.ID,
				Sort:      "created_at",
				Direction: core.Ascending,
			}.Encode()
		}
		return nil
	})
	if err != nil {
		return Backlog{}, err
	}
	if b.Pending > 0 {
		b.OldestAge = d.clk.Since(b.OldestPendingAt)
	}
	return b, nil
}

// responseError describes a non-success response without quoting its body.
func responseError(code int) string {
	if code == 0 {
		return "no response"
	}
	return fmt.Sprintf("endpoint responded %d", code)
}

// transportError describes a failed request without repeating the target url,
// which may itself carry a credential.
func transportError(err error) string {
	msg := err.Error()
	var ue *url.Error
	if errors.As(err, &ue) && ue.Err != nil {
		msg = ue.Op + ": " + ue.Err.Error()
	}
	if len(msg) > maxErrorLength {
		msg = msg[:maxErrorLength]
	}
	return msg
}
