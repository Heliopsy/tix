// Package lease resolves lease lifetimes, mints lease tokens and materializes
// lease expiry.
package lease

import (
	"crypto/rand"
	"encoding/base64"
	"time"

	"github.com/thereisnotime/tix/internal/core"
)

// DefaultTTL is the lease lifetime used when neither the claim nor the
// workflow supplies one.
const DefaultTTL = 15 * time.Minute

// MaxTTL is the longest lease any single claim may take.
const MaxTTL = 24 * time.Hour

// RenewDivisor is the fraction of a TTL at which a holder is expected to renew.
const RenewDivisor = 3

// TokenBytes is the entropy behind one lease token.
const TokenBytes = 32

// Policy bounds lease lifetimes for one installation.
type Policy struct {
	Default time.Duration
	Max     time.Duration
}

// DefaultPolicy returns the shipped lease policy.
func DefaultPolicy() Policy { return Policy{Default: DefaultTTL, Max: MaxTTL} }

// Resolve picks the most specific lifetime offered: the per-claim value, then
// the workflow's default, then the policy default.
func (p Policy) Resolve(perClaim, workflow core.Duration) (time.Duration, error) {
	def, limit := p.Default, p.Max
	if def <= 0 {
		def = DefaultTTL
	}
	if limit <= 0 {
		limit = MaxTTL
	}

	ttl := def
	switch {
	case perClaim != 0:
		ttl = perClaim.D()
	case workflow != 0:
		ttl = workflow.D()
	}

	if ttl <= 0 {
		return 0, core.Invalid("lease ttl %s must be positive", ttl)
	}
	if ttl > limit {
		return 0, core.Invalid("lease ttl %s exceeds the maximum of %s", ttl, limit)
	}
	return ttl, nil
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

// NewToken mints an opaque lease token from crypto/rand.
func NewToken() (string, error) {
	b := make([]byte, TokenBytes)
	if _, err := rand.Read(b); err != nil {
		return "", core.Internal("minting a lease token: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
