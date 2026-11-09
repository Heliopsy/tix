// SPDX-License-Identifier: AGPL-3.0-or-later

package service

import (
	"context"
	"sync"
	"time"

	"github.com/heliopsy/tix/internal/core"
	"github.com/heliopsy/tix/internal/store"
)

// auditTOTPGate records a request the second factor policy refused.
const auditTOTPGate = "user.totp.gate"

// gateAuditEvery is how often one person's refusals are written to the audit
// log in one process. A browser that keeps polling, or a script retrying a
// refused token, would otherwise fill the log with the same fact.
const gateAuditEvery = time.Hour

// gateMemoFor is how long one actor's standing is reused. A request resolves
// one actor and asks the policy many times, several of them inside a
// transaction it already holds; the standing read once before that
// transaction opened is what those answer from. It also bounds how long a
// long-lived actor, an SSH session's, goes on the standing it last read when
// another process changed it; this process forgets every standing as soon as
// it changes a policy or an enrolment itself.
const gateMemoFor = 5 * time.Second

// gateMemoCap bounds the memo. Every request brings a new actor, so a clock
// that does not move (a test's) would otherwise grow it without end.
const gateMemoCap = 4096

// gateLog remembers when each person's last refusal was recorded, and the
// standing each live actor was last read with.
type gateLog struct {
	mu   sync.Mutex
	last map[string]time.Time
	memo map[*core.Actor]memoEntry

	// pending is the refusals being written to the audit log.
	pending sync.WaitGroup
}

// memoEntry is one actor's standing and when it was read.
type memoEntry struct {
	standing factorStanding
	at       time.Time
}

// recall answers an actor's standing when it was read within gateMemoFor.
func (g *gateLog) recall(a *core.Actor, now time.Time) (factorStanding, bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	e, ok := g.memo[a]
	if !ok || now.Sub(e.at) >= gateMemoFor || now.Before(e.at) {
		return factorStanding{}, false
	}
	return e.standing, true
}

// remember keeps an actor's standing, dropping what has gone stale first.
func (g *gateLog) remember(a *core.Actor, s factorStanding, now time.Time) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.memo == nil {
		g.memo = map[*core.Actor]memoEntry{}
	}
	if len(g.memo) >= gateMemoCap {
		for k, e := range g.memo {
			if now.Sub(e.at) >= gateMemoFor || now.Before(e.at) {
				delete(g.memo, k)
			}
		}
		if len(g.memo) >= gateMemoCap {
			clear(g.memo)
		}
	}
	g.memo[a] = memoEntry{standing: s, at: now}
}

// forget drops every remembered standing, after this process changed a
// policy or an enrolment.
func (g *gateLog) forget() {
	g.mu.Lock()
	defer g.mu.Unlock()
	clear(g.memo)
}

// due reports whether a refusal of key at now should be recorded, and marks it.
func (g *gateLog) due(key string, now time.Time) bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.last == nil {
		g.last = map[string]time.Time{}
	}
	if at, ok := g.last[key]; ok && now.Sub(at) < gateAuditEvery {
		return false
	}
	g.last[key] = now
	return true
}

// factorStanding is what the policy says about one caller right now.
type factorStanding struct {
	// person is false for an actor with no account behind it, which the
	// policy never reaches: there is nobody to enrol.
	person   bool
	enrolled bool
	tenant   *core.Tenant
}

// due is when the policy starts refusing this caller, nil when it never will.
func (s factorStanding) due() *time.Time {
	if !s.person || s.enrolled || s.tenant == nil {
		return nil
	}
	return s.tenant.MFADeadline()
}

// gated reports whether the policy refuses this caller at now.
func (s factorStanding) gated(now time.Time) bool {
	return s.person && !s.enrolled && s.tenant != nil && s.tenant.MFAEnforced(now)
}

// factorStandingOf reads the tenant's policy and, only when it is required,
// whether the caller is a person and holds a factor, answering from the memo
// when this actor was read a moment ago.
//
// It reads through the store directly rather than l.read, because l.read
// warms this very memo first.
func (l *Local) factorStandingOf(ctx context.Context, actor *core.Actor) (factorStanding, error) {
	var out factorStanding
	if actor == nil || actor.Kind == core.ActorSystem {
		return out, nil
	}
	now := l.clock.Now()
	if s, ok := l.gates.recall(actor, now); ok {
		return s, nil
	}
	out, err := l.readStanding(ctx, actor)
	if err == nil {
		l.gates.remember(actor, out, now)
	}
	return out, err
}

// warmFactor reads an actor's standing before a transaction opens, so a
// policy question asked inside that transaction answers from memory. Asked
// inside it instead, the read would wait for a second connection while the
// first is held, and enough concurrent requests doing that hold every
// connection in the pool and wait on each other for good.
func (l *Local) warmFactor(ctx context.Context, actor *core.Actor) {
	_, _ = l.factorStandingOf(ctx, actor)
}

// readStanding is the store read behind factorStandingOf.
func (l *Local) readStanding(ctx context.Context, actor *core.Actor) (factorStanding, error) {
	var out factorStanding
	err := l.store.View(ctx, core.TenantScope{TenantID: actor.TenantID}, func(tx store.Tx) error {
		tenant, err := tx.GetTenant(ctx)
		if err != nil {
			return err
		}
		out.tenant = tenant
		if !tenant.MFARequired() {
			return nil
		}
		user, err := accountOfActor(ctx, tx, actor.ID)
		if core.IsKind(err, core.KindNotFound) {
			return nil
		}
		if err != nil {
			return err
		}
		out.person = true
		out.enrolled, err = totpEnrolled(ctx, tx, user.ID)
		return err
	})
	if core.IsKind(err, core.KindNotFound) {
		return factorStanding{}, nil
	}
	return out, err
}

// requireFactor is the second factor policy's gate. It runs at the single
// enforcement point, so every surface reaching the service meets it: the
// browser, the API and MCP, the terminal interface over SSH and the CLI, with
// a session or with a token the person owns. What it leaves open is what never
// passes through authorize: enrolment, who-am-I, the caller's own account and
// sessions, signing out and leaving for another tenant.
func (l *Local) requireFactor(ctx context.Context, actor *core.Actor) error {
	standing, err := l.factorStandingOf(ctx, actor)
	if err != nil {
		return err
	}
	now := l.clock.Now()
	if !standing.gated(now) {
		return nil
	}
	l.recordGate(ctx, actor, now)
	return errEnrollmentRequired()
}

// errEnrollmentRequired is the refusal the policy answers.
func errEnrollmentRequired() error {
	return core.Forbidden("this tenant requires a second factor and this account has none; "+
		"set one up in the browser at /settings/totp, with tix user totp enrol, or by signing in "+
		"with tix login or over ssh on a terminal").
		WithDetail("reason", core.ReasonSecondFactorEnrollmentRequired)
}

// recordGate writes a refusal to the audit log, at most once an hour per
// person and tenant in this process. It is best effort: the refusal stands
// whether or not it could be written down.
//
// The write runs on its own goroutine, after the refused request lets go of
// what it holds. A refusal can come from inside a write transaction, and a
// second write there waits for the only writer, which is itself.
func (l *Local) recordGate(ctx context.Context, actor *core.Actor, now time.Time) {
	if !l.gates.due(actor.TenantID+"/"+actor.ID, now) {
		return
	}
	// The refused request's idempotency key is left alone: the request wrote
	// nothing, and a key marked applied would replay a refusal as a success.
	ctx = WithIdempotencyClaim(context.WithoutCancel(ctx), nil)
	who := *actor
	l.gates.pending.Add(1)
	go func() {
		defer l.gates.pending.Done()
		_ = l.write(ctx, &who, func(m *mutation) error {
			return m.Audit(auditTOTPGate, subjectTOTP, who.ID, nil,
				map[string]any{"actor_id": who.ID, "reason": core.ReasonSecondFactorEnrollmentRequired})
		})
	}()
}
