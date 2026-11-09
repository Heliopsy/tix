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

// gateLog remembers when each person's last refusal was recorded.
type gateLog struct {
	mu   sync.Mutex
	last map[string]time.Time
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
// whether the caller is a person and holds a factor.
func (l *Local) factorStandingOf(ctx context.Context, actor *core.Actor) (factorStanding, error) {
	var out factorStanding
	if actor.Kind == core.ActorSystem {
		return out, nil
	}
	err := l.read(ctx, actor, func(tx store.Tx) error {
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
func (l *Local) recordGate(ctx context.Context, actor *core.Actor, now time.Time) {
	if !l.gates.due(actor.TenantID+"/"+actor.ID, now) {
		return
	}
	// The refused request's idempotency key is left alone: the request wrote
	// nothing, and a key marked applied would replay a refusal as a success.
	ctx = WithIdempotencyClaim(ctx, nil)
	_ = l.write(ctx, actor, func(m *mutation) error {
		return m.Audit(auditTOTPGate, subjectTOTP, actor.ID, nil,
			map[string]any{"actor_id": actor.ID, "reason": core.ReasonSecondFactorEnrollmentRequired})
	})
}
