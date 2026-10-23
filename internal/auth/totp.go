// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1" // #nosec G505 -- RFC 6238 over HMAC-SHA1; see TOTPDigits for why
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/heliopsy/tix/internal/core"
)

// The RFC 6238 parameters tix uses everywhere.
//
// SHA-256 and eight digits are both in the RFC and both stronger. They are not
// an option: the algorithm and digits parameters of the otpauth:// URI are
// ignored by most authenticator applications, which assume SHA1, six digits
// and thirty seconds whatever the URI declares. Choosing the stronger
// parameters produces an enrolment that silently generates codes the server
// rejects, and the user blames the clock.
//
// SHA1's collision problems do not apply to HMAC-SHA1, and a six-digit code's
// guess probability is handled by the rate limit below, not by digit count.
const (
	TOTPDigits = 6
	TOTPPeriod = 30 * time.Second
	// TOTPSkew accepts the step before and the step after the current one, so
	// a clock drifting in either direction still verifies.
	TOTPSkew = 1
	// TOTPSecretBytes is the shared secret size, the twenty bytes RFC 4226
	// names and every authenticator handles.
	TOTPSecretBytes = 20
)

// The rate limit, counted on the enrolment rather than on the session or the
// network address: an attacker controls both of those and controls neither the
// account nor the clock.
const (
	TOTPMaxFailures = 5
	TOTPLockWindow  = 15 * time.Minute
)

// ErrTOTPLocked refuses verification for rate limiting rather than for a
// wrong code. It carries its own reason detail so a surface can tell the user
// to wait instead of telling them their authenticator is wrong, which is a
// distinction the error kind alone does not carry across a transport.
func ErrTOTPLocked(until time.Time) error {
	return core.Precondition("too many incorrect codes; verification is locked until %s",
		until.UTC().Format(time.RFC3339)).
		WithDetail(core.DetailReason, core.ReasonTOTPLocked)
}

// ErrTOTPInvalid is the single answer every rejected code gives: out of
// window, already spent, or simply wrong.
func ErrTOTPInvalid() error { return core.Unauthenticated("incorrect code") }

// totpEncoding is the unpadded base32 alphabet authenticator applications read.
var totpEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewTOTPSecret mints a shared secret in the base32 form an authenticator takes.
func NewTOTPSecret() (string, error) {
	raw := make([]byte, TOTPSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", core.Internal("reading random bytes").Wrap(err)
	}
	return totpEncoding.EncodeToString(raw), nil
}

// TOTPStep is the RFC 6238 time step containing t.
func TOTPStep(t time.Time) int64 { return t.UTC().Unix() / int64(TOTPPeriod/time.Second) }

// TOTPCode renders the code for one step of one secret.
func TOTPCode(secret string, step int64) (string, error) {
	key, err := decodeTOTPSecret(secret)
	if err != nil {
		return "", err
	}
	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(step)) // #nosec G115 -- a step is a wrapped counter, not a measurement
	mac := hmac.New(sha1.New, key)
	mac.Write(counter[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	truncated := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	mod := uint32(1)
	for range TOTPDigits {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", TOTPDigits, truncated%mod), nil
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	cleaned := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(secret), " ", ""))
	cleaned = strings.TrimRight(cleaned, "=")
	key, err := totpEncoding.DecodeString(cleaned)
	if err != nil || len(key) == 0 {
		return nil, core.Internal("the totp secret is not valid base32")
	}
	return key, nil
}

// TOTPURI renders the otpauth:// URI an authenticator scans. The parameters it
// spells out are the defaults; they are written anyway so a reader can see
// what the server will accept.
func TOTPURI(issuer, account, secret string) string {
	label := url.PathEscape(issuer + ":" + account)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", issuer)
	q.Set("algorithm", "SHA1")
	q.Set("digits", fmt.Sprint(TOTPDigits))
	q.Set("period", fmt.Sprint(int(TOTPPeriod/time.Second)))
	return "otpauth://totp/" + label + "?" + q.Encode()
}

// NormalizeTOTPCode strips the spacing authenticators and humans put in a code.
func NormalizeTOTPCode(code string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, code)
}

// MatchTOTPCode returns the step a code was produced for, searching the
// acceptance window around now, and reports whether any step matched.
//
// It knows nothing about replay: the window is what makes verification
// tolerant of drift, and Attempt is what keeps a code single use. Both are
// required, and the second is the one that gets left out.
func MatchTOTPCode(secret, code string, now time.Time) (int64, bool) {
	code = NormalizeTOTPCode(code)
	if len(code) != TOTPDigits {
		return 0, false
	}
	current := TOTPStep(now)
	matched, found := int64(0), false
	for delta := int64(-TOTPSkew); delta <= TOTPSkew; delta++ {
		step := current + delta
		want, err := TOTPCode(secret, step)
		if err != nil {
			return 0, false
		}
		// Every step in the window is tried even after a match, so the time
		// taken does not say which step was right.
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 && !found {
			matched, found = step, true
		}
	}
	return matched, found
}

// Attempt is the mutable verification state of one enrolment: the highest step
// it has accepted, how many codes have failed in a row, and until when it is
// locked. The caller loads it from the enrolment row, calls Verify, and writes
// it back whatever the answer was.
type Attempt struct {
	LastStep    int64
	Failures    int
	LockedUntil *time.Time
}

// Locked reports whether verification is refused for rate limiting at now.
func (a *Attempt) Locked(now time.Time) bool {
	return a.LockedUntil != nil && a.LockedUntil.After(now)
}

// Verify checks a code against the secret and records the outcome on a.
//
// A code whose step is at or below LastStep is refused however recent it is.
// Without that, the three-step window leaves a code shouted across a room,
// read off a shoulder or captured from a terminal's scrollback usable for
// ninety seconds.
func (a *Attempt) Verify(secret, code string, now time.Time) error {
	if a.Locked(now) {
		return ErrTOTPLocked(*a.LockedUntil)
	}
	step, ok := MatchTOTPCode(secret, code, now)
	if !ok || step <= a.LastStep {
		a.fail(now)
		return ErrTOTPInvalid()
	}
	a.Succeed(step)
	return nil
}

// Succeed records an accepted code: the high-water mark moves up and the
// failure count returns to zero.
func (a *Attempt) Succeed(step int64) {
	if step > a.LastStep {
		a.LastStep = step
	}
	a.Failures = 0
	a.LockedUntil = nil
}

// Fail records a rejected credential that was not a TOTP code, such as a
// recovery code that did not match, so the same limit covers both paths.
func (a *Attempt) Fail(now time.Time) { a.fail(now) }

func (a *Attempt) fail(now time.Time) {
	a.Failures++
	if a.Failures >= TOTPMaxFailures {
		until := now.Add(TOTPLockWindow)
		a.LockedUntil = &until
	}
}
