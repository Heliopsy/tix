// SPDX-License-Identifier: AGPL-3.0-or-later

// Package lease resolves lease lifetimes, mints lease tokens and materializes
// lease expiry.
package lease

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// DefaultTTL is the lease lifetime used when nothing more specific supplies
// one: not the claim, not an authored workflow, not the tenant, not the
// deployment.
//
// Thirty minutes rather than the fifteen this was, because fifteen was never
// what anybody got. Every tenant is created with the builtin workflow, which
// carried a 30m default_lease, so that figure governed every claim on every
// shipped installation and this constant was only ever reached by a custom
// workflow that named no lease of its own. The builtin's figure no longer
// counts, to stop a template nobody chose masking the tenant setting, so the
// number it was supplying moves here and the unconfigured lease stays 30m.
// Keep this equal to service.DefaultLease, which seeds the same value.
const DefaultTTL = 30 * time.Minute

// MaxTTL is the longest lease any single claim may take.
const MaxTTL = 24 * time.Hour

// RenewDivisor is the fraction of a TTL at which a holder is expected to renew.
const RenewDivisor = 3

// TokenBytes is the entropy behind one lease token.
const TokenBytes = 32

// Policy bounds lease lifetimes for one installation. DefaultFrom and
// MaxFrom name the level each bound was last set by; empty reads as builtin.
type Policy struct {
	Default     time.Duration
	Max         time.Duration
	DefaultFrom core.LeaseSource
	MaxFrom     core.LeaseSource
}

// DefaultPolicy returns the shipped lease policy.
func DefaultPolicy() Policy { return Policy{Default: DefaultTTL, Max: MaxTTL} }

// Offer is what the levels between the claim and the tenant say about one
// lease, each zero when that level names nothing.
type Offer struct {
	Claim    core.Duration
	Task     core.Duration
	Project  core.Duration
	Workflow core.Duration
}

// Narrow layers a tenant's own bounds over this one.
//
// The default is an override and the maximum is a clamp, which is the
// difference between the two settings and the reason they are not applied the
// same way. A tenant choosing a longer default is choosing for work it owns,
// so it wins outright. A tenant choosing a longer maximum would be choosing
// for the operator whose machine it runs on, so it cannot: the effective
// ceiling is the lower of the two, and a tenant may only tighten it.
//
// Zero on either side means "nobody chose" and leaves that bound as it was,
// so a tenant that has never been configured resolves exactly as it did
// before these columns existed.
func (p Policy) Narrow(def, max core.Duration) Policy {
	out := p
	if def > 0 {
		out.Default, out.DefaultFrom = def.D(), core.LeaseFromTenant
	}
	return out.Cap(max, core.LeaseFromTenant)
}

// Cap lowers the ceiling to max when max is set and lower, recording from as
// the level that set it. It never raises the ceiling.
func (p Policy) Cap(max core.Duration, from core.LeaseSource) Policy {
	out := p
	if max > 0 && (out.Max <= 0 || max.D() < out.Max) {
		out.Max, out.MaxFrom = max.D(), from
	}
	return out
}

// Resolve picks the most specific lifetime offered: the per-claim value, then
// the workflow's default, then the policy default.
func (p Policy) Resolve(perClaim, workflow core.Duration) (time.Duration, error) {
	g, err := p.Grant(Offer{Claim: perClaim, Workflow: workflow})
	return g.TTL.D(), err
}

// Grant picks the first level of the offer that names a length, falling back
// to the policy default, and holds it to the policy ceiling.
func (p Policy) Grant(o Offer) (core.LeaseGrant, error) {
	def, limit := p.Default, p.Max
	defFrom, maxFrom := sourceOr(p.DefaultFrom), sourceOr(p.MaxFrom)
	if def <= 0 {
		def, defFrom = DefaultTTL, core.LeaseFromBuiltin
	}
	if limit <= 0 {
		limit, maxFrom = MaxTTL, core.LeaseFromBuiltin
	}

	ttl, from := def, defFrom
	for _, level := range []struct {
		d    core.Duration
		from core.LeaseSource
	}{
		{o.Claim, core.LeaseFromClaim},
		{o.Task, core.LeaseFromTask},
		{o.Project, core.LeaseFromProject},
		{o.Workflow, core.LeaseFromWorkflow},
	} {
		if level.d != 0 {
			ttl, from = level.d.D(), level.from
			break
		}
	}
	asked := from == core.LeaseFromClaim
	g := core.LeaseGrant{Source: from, Max: core.Duration(limit), MaxSource: maxFrom}

	if ttl <= 0 {
		return g, core.Invalid("lease ttl %s must be positive", ttl)
	}
	if ttl > limit {
		// Only a caller who named the figure is told it is too long. A
		// default that exceeds the ceiling is clamped to it instead, because
		// the two cases fail differently: naming 48h is one caller's mistake
		// and the error is the answer, while a configured default drifting
		// above a ceiling an operator lowered afterwards would otherwise
		// refuse every claim on the tenant until somebody noticed. Clamping
		// shortens leases; erroring stops work.
		if asked {
			return g, core.Invalid("lease ttl %s exceeds the maximum of %s", ttl, limit)
		}
		ttl, g.Clamped = limit, true
	}
	g.TTL = core.Duration(ttl)
	return g, nil
}

func sourceOr(s core.LeaseSource) core.LeaseSource {
	if s == "" {
		return core.LeaseFromBuiltin
	}
	return s
}

// Resolve picks a lease lifetime under the default policy.
func Resolve(perClaim, workflow core.Duration) (time.Duration, error) {
	return DefaultPolicy().Resolve(perClaim, workflow)
}

// RenewInterval returns the cadence at which a holder should renew a lease.
func RenewInterval(ttl time.Duration) time.Duration {
	if ttl <= 0 {
		return 0
	}
	if iv := ttl / RenewDivisor; iv > 0 {
		return iv
	}
	return ttl
}

// Precision is the resolution a lease expiry is materialized at, and is the
// coarsest any shipped storage engine keeps: PostgreSQL's TIMESTAMPTZ holds
// microseconds and truncates rather than rounds.
//
// Expiry is therefore cut to it before it is stored, so the instant handed to
// the engine is the instant the row holds and the instant the holder is told.
// Left at nanoseconds the two engines recorded different expiries for one
// claim, and on PostgreSQL the sweeper could free a lease up to 999ns before
// the moment its holder was given.
const Precision = time.Microsecond

// Until is when a lease taken at now for ttl expires, at Precision.
func Until(now time.Time, ttl time.Duration) time.Time {
	return now.Add(ttl).Truncate(Precision)
}

// NewToken mints an opaque lease token from crypto/rand.
func NewToken() (string, error) {
	b := make([]byte, TokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", core.Internal("minting a lease token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
