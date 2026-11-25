// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// Idempotency key limits.
const (
	// DefaultIdempotencyWindow is how long a kept key answers repeats unless
	// retention.idempotency_keys says otherwise.
	DefaultIdempotencyWindow = 24 * time.Hour
	// MaxIdempotencyKeyLen bounds a key, in bytes.
	MaxIdempotencyKeyLen = 200
	// MaxIdempotentBody bounds a response body that is kept for replay.
	MaxIdempotentBody = 1 << 20
	// defaultIdempotencyLease applies when a caller names no lease.
	defaultIdempotencyLease = time.Minute
)

// IdempotentOutcome says what to do with a request that carries a key.
type IdempotentOutcome int

// Outcomes of BeginIdempotent.
const (
	// IdempotentRun means the request runs; the key is reserved for it.
	IdempotentRun IdempotentOutcome = iota
	// IdempotentReplay means the kept response answers the request.
	IdempotentReplay
	// IdempotentMismatch means the key belongs to a different request.
	IdempotentMismatch
	// IdempotentInFlight means the key's first request is still running.
	IdempotentInFlight
	// IdempotentLost means the key's request committed a write but its
	// response was not kept, so it must not run again.
	IdempotentLost
)

// IdempotentRequest is a request that carries a key.
type IdempotentRequest struct {
	// Key is the client's Idempotency-Key.
	Key string
	// Fingerprint identifies the request the key is used with: a digest of
	// its method, request URI and body.
	Fingerprint string
	// Lease is how long the request counts as running before a retry may
	// take the key over.
	Lease time.Duration
}

// IdempotentResponse is an answer kept for replay.
type IdempotentResponse struct {
	Status      int
	ContentType string
	Body        []byte
	// TooLarge says the body ran past MaxIdempotentBody and was not kept.
	TooLarge bool
}

// IdempotencyClaim names the reserved row of a running request. Carried on
// the request context, it lets the mutation's own transaction record that
// the request committed.
type IdempotencyClaim struct {
	ID       string
	TenantID string
	ActorID  string
	Key      string
}

// IdempotentStart is the answer of BeginIdempotent.
type IdempotentStart struct {
	Outcome IdempotentOutcome
	// Claim is set when the request runs.
	Claim *IdempotencyClaim
	// Response is set on a replay.
	Response *IdempotentResponse
	// RetryAfter is set when the first request is still running.
	RetryAfter time.Duration
}

// ValidateIdempotencyKey refuses a key that is empty, longer than
// MaxIdempotencyKeyLen, or holds anything but printable ASCII.
func ValidateIdempotencyKey(key string) error {
	if key == "" || len(key) > MaxIdempotencyKeyLen {
		return core.Invalid("an idempotency key must be 1 to %d characters", MaxIdempotencyKeyLen)
	}
	for i := 0; i < len(key); i++ {
		if key[i] < 0x20 || key[i] > 0x7e {
			return core.Invalid("an idempotency key must be printable ASCII")
		}
	}
	return nil
}

// idempotencyCtxKey carries the claim of the running request.
type idempotencyCtxKey struct{}

// WithIdempotencyClaim returns a context carrying the claim of a running
// request, so every write it makes marks the key as applied.
func WithIdempotencyClaim(ctx context.Context, c *IdempotencyClaim) context.Context {
	return context.WithValue(ctx, idempotencyCtxKey{}, c)
}

// idempotencyClaimFrom returns the claim a context carries, or nil.
func idempotencyClaimFrom(ctx context.Context) *IdempotencyClaim {
	c, _ := ctx.Value(idempotencyCtxKey{}).(*IdempotencyClaim)
	return c
}

// markIdempotent records, inside a mutation's transaction, that the request
// holding the claim committed a write. A claim from another tenant is left
// alone: the row is not in this transaction's scope.
func markIdempotent(ctx context.Context, tx store.Tx) error {
	c := idempotencyClaimFrom(ctx)
	if c == nil || c.TenantID != tx.Scope().TenantID {
		return nil
	}
	return tx.MarkIdempotencyExecuted(ctx, c.ID)
}

// BeginIdempotent decides what a request carrying a key does: run, with the
// key reserved for it, replay the kept answer, or be refused. The key is
// scoped to the tenant and the actor. It writes no audit entry or event: the
// row is bookkeeping about a request, not a change to anything a reader sees.
func (l *Local) BeginIdempotent(ctx context.Context, in IdempotentRequest) (*IdempotentStart, error) {
	actor, err := core.RequireActor(ctx)
	if err != nil {
		return nil, err
	}
	if err := ValidateIdempotencyKey(in.Key); err != nil {
		return nil, err
	}
	if in.Fingerprint == "" {
		return nil, core.Invalid("an idempotent request needs a fingerprint")
	}
	if in.Lease <= 0 {
		in.Lease = defaultIdempotencyLease
	}
	now := l.clock.Now().UTC()
	var out *IdempotentStart
	err = l.store.Update(ctx, core.TenantScope{TenantID: actor.TenantID}, func(tx store.Tx) error {
		rec, err := tx.GetIdempotencyKey(ctx, actor.ID, in.Key)
		switch {
		case core.IsKind(err, core.KindNotFound):
			rec = nil
		case err != nil:
			return err
		}
		if rec != nil {
			if now.Before(rec.ExpiresAt) {
				if out = decideIdempotent(rec, in.Fingerprint, now); out != nil {
					return nil
				}
			}
			if err := tx.DeleteIdempotencyKey(ctx, rec.ID); err != nil {
				return err
			}
		}
		fresh := &store.IdempotencyRecord{
			ID: l.ids.New(), ActorID: actor.ID, Key: in.Key, Fingerprint: in.Fingerprint,
			State: store.IdempotencyReserved, CreatedAt: now,
			LeaseExpiresAt: now.Add(in.Lease), ExpiresAt: now.Add(l.idempotencyWindow),
		}
		if err := tx.CreateIdempotencyKey(ctx, fresh); err != nil {
			return err
		}
		out = &IdempotentStart{Outcome: IdempotentRun, Claim: &IdempotencyClaim{
			ID: fresh.ID, TenantID: actor.TenantID, ActorID: actor.ID, Key: in.Key,
		}}
		return nil
	})
	if core.IsKind(err, core.KindConflict) {
		return &IdempotentStart{Outcome: IdempotentInFlight, RetryAfter: time.Second}, nil
	}
	if err != nil {
		return nil, err
	}
	return out, nil
}

// decideIdempotent reads a live row. It returns nil when the row is a
// reservation whose request died before committing, which a retry takes over.
func decideIdempotent(rec *store.IdempotencyRecord, fingerprint string, now time.Time) *IdempotentStart {
	switch {
	case rec.Fingerprint != fingerprint:
		return &IdempotentStart{Outcome: IdempotentMismatch}
	case rec.State == store.IdempotencyDone:
		return &IdempotentStart{Outcome: IdempotentReplay, Response: &IdempotentResponse{
			Status: rec.Status, ContentType: rec.ContentType, Body: rec.Body,
		}}
	case now.Before(rec.LeaseExpiresAt):
		return &IdempotentStart{Outcome: IdempotentInFlight, RetryAfter: rec.LeaseExpiresAt.Sub(now)}
	case rec.State == store.IdempotencyExecuted:
		return &IdempotentStart{Outcome: IdempotentLost}
	}
	return nil
}

// FinishIdempotent settles a claim once its request has answered. A request
// that committed a write keeps its answer whatever the status, since running
// it again would apply the change twice. One that committed nothing keeps a
// 2xx answer and releases the key otherwise, so a retry runs. An answer over
// MaxIdempotentBody is not kept, and a request that committed one stays
// applied without a kept answer.
func (l *Local) FinishIdempotent(ctx context.Context, c *IdempotencyClaim, resp IdempotentResponse) error {
	if c == nil {
		return nil
	}
	now := l.clock.Now().UTC()
	return l.store.Update(ctx, core.TenantScope{TenantID: c.TenantID}, func(tx store.Tx) error {
		rec, err := tx.GetIdempotencyKey(ctx, c.ActorID, c.Key)
		switch {
		case core.IsKind(err, core.KindNotFound):
			return nil
		case err != nil:
			return err
		}
		if rec.ID != c.ID {
			return nil
		}
		committed := rec.State == store.IdempotencyExecuted
		fits := !resp.TooLarge && len(resp.Body) <= MaxIdempotentBody
		success := resp.Status >= 200 && resp.Status < 300
		if !committed && (!success || !fits) {
			return tx.DeleteIdempotencyKey(ctx, rec.ID)
		}
		rec.ExpiresAt = now.Add(l.idempotencyWindow)
		if fits {
			rec.State, rec.Status, rec.ContentType, rec.Body = store.IdempotencyDone, resp.Status, resp.ContentType, resp.Body
		}
		return tx.FinishIdempotencyKey(ctx, rec)
	})
}
