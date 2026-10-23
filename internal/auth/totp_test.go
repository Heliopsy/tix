// SPDX-License-Identifier: AGPL-3.0-or-later

package auth_test

import (
	"encoding/base32"
	"strings"
	"testing"
	"time"

	"github.com/heliopsy/tix/internal/auth"
	"github.com/heliopsy/tix/internal/clock"
	"github.com/heliopsy/tix/internal/core"
)

// rfcSecret is the twenty byte seed RFC 6238 Appendix B publishes its vectors
// against, "12345678901234567890", in the base32 form an authenticator takes.
var rfcSecret = base32.StdEncoding.WithPadding(base32.NoPadding).
	EncodeToString([]byte("12345678901234567890"))

// TestTOTPMatchesTheRFCVectors checks the generator against the published
// SHA-1 column of RFC 6238 Appendix B. The RFC prints eight digits; tix emits
// six, which is the last six of each.
func TestTOTPMatchesTheRFCVectors(t *testing.T) {
	t.Parallel()
	cases := []struct {
		unix int64
		want string
	}{
		{59, "287082"},
		{1111111109, "081804"},
		{1111111111, "050471"},
		{1234567890, "005924"},
		{2000000000, "279037"},
		{20000000000, "353130"},
	}
	for _, c := range cases {
		at := time.Unix(c.unix, 0).UTC()
		got, err := auth.TOTPCode(rfcSecret, auth.TOTPStep(at))
		if err != nil {
			t.Fatalf("TOTPCode at %d: %v", c.unix, err)
		}
		if got != c.want {
			t.Errorf("TOTPCode at %d = %q, want %q", c.unix, got, c.want)
		}
	}
}

func TestTOTPCodeIsSixDigits(t *testing.T) {
	t.Parallel()
	secret, err := auth.NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret() error = %v", err)
	}
	for step := range int64(200) {
		code, err := auth.TOTPCode(secret, step)
		if err != nil {
			t.Fatalf("TOTPCode: %v", err)
		}
		if len(code) != auth.TOTPDigits {
			t.Fatalf("TOTPCode at step %d = %q, want %d digits", step, code, auth.TOTPDigits)
		}
		if strings.Trim(code, "0123456789") != "" {
			t.Fatalf("TOTPCode at step %d = %q, want digits only", step, code)
		}
	}
}

// TestTOTPAcceptanceWindow is the drift tolerance: one step either side is
// accepted and anything further out is not.
func TestTOTPAcceptanceWindow(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Unix(1_700_000_000, 0).UTC())
	now := clk.Now()
	current := auth.TOTPStep(now)

	cases := []struct {
		name string
		step int64
		want bool
	}{
		{"the previous step", current - 1, true},
		{"the current step", current, true},
		{"the following step", current + 1, true},
		{"two steps behind", current - 2, false},
		{"two steps ahead", current + 2, false},
	}
	for _, c := range cases {
		code, err := auth.TOTPCode(rfcSecret, c.step)
		if err != nil {
			t.Fatalf("TOTPCode: %v", err)
		}
		_, ok := auth.MatchTOTPCode(rfcSecret, code, now)
		if ok != c.want {
			t.Errorf("%s: accepted = %v, want %v", c.name, ok, c.want)
		}
	}
}

// TestACodeIsSingleUse is the high-water mark. The window makes verification
// tolerant of drift; without this a code observed over a shoulder stays usable
// for the whole ninety seconds the window spans.
func TestACodeIsSingleUse(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Unix(1_700_000_000, 0).UTC())
	var a auth.Attempt

	code, err := auth.TOTPCode(rfcSecret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if err := a.Verify(rfcSecret, code, clk.Now()); err != nil {
		t.Fatalf("first use of a valid code: %v", err)
	}
	if a.LastStep != auth.TOTPStep(clk.Now()) {
		t.Fatalf("LastStep = %d, want the step just accepted", a.LastStep)
	}

	// Replayed immediately, and again later but still inside the window the
	// acceptance rule would otherwise allow it through.
	for _, after := range []time.Duration{0, 10 * time.Second, 29 * time.Second} {
		at := clk.Now().Add(after)
		if err := a.Verify(rfcSecret, code, at); err == nil {
			t.Fatalf("the same code was accepted again %s later", after)
		}
	}

	// The next step still works, so single use has not become single sign-in.
	clk.Advance(auth.TOTPPeriod)
	next, err := auth.TOTPCode(rfcSecret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if err := a.Verify(rfcSecret, next, clk.Now()); err != nil {
		t.Fatalf("the following step's code was refused: %v", err)
	}
}

// TestAnEarlierCodeInsideTheWindowIsRefused covers the other half of the
// high-water mark: the window reaches a step behind the one already accepted,
// and that step must not be reachable again either.
func TestAnEarlierCodeInsideTheWindowIsRefused(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Unix(1_700_000_000, 0).UTC())
	var a auth.Attempt

	current := auth.TOTPStep(clk.Now())
	previous, err := auth.TOTPCode(rfcSecret, current-1)
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	code, err := auth.TOTPCode(rfcSecret, current)
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if err := a.Verify(rfcSecret, code, clk.Now()); err != nil {
		t.Fatalf("accepting the current code: %v", err)
	}
	if err := a.Verify(rfcSecret, previous, clk.Now()); err == nil {
		t.Fatal("a code from a step below the high-water mark was accepted")
	}
}

// TestVerificationLocksAfterRepeatedFailure is the rate limit, counted on the
// enrolment rather than on the address or the session.
func TestVerificationLocksAfterRepeatedFailure(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Unix(1_700_000_000, 0).UTC())
	var a auth.Attempt

	for i := range auth.TOTPMaxFailures {
		if err := a.Verify(rfcSecret, "000000", clk.Now()); err == nil {
			t.Fatalf("attempt %d: a wrong code was accepted", i+1)
		}
		if core.IsTOTPLocked(a.Verify(rfcSecret, "000000", clk.Now())) && i < auth.TOTPMaxFailures-1 {
			t.Fatalf("locked after %d failures, want %d", i+1, auth.TOTPMaxFailures)
		}
	}

	// The correct code is refused too, and refused as rate limiting rather
	// than as a wrong code.
	good, err := auth.TOTPCode(rfcSecret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	err = a.Verify(rfcSecret, good, clk.Now())
	if err == nil {
		t.Fatal("a correct code was accepted while the enrolment was locked")
	}
	if !core.IsTOTPLocked(err) {
		t.Fatalf("the refusal was %v, want one a surface can tell from a wrong code", err)
	}

	// Once the window passes the same code works again.
	clk.Advance(auth.TOTPLockWindow + time.Second)
	good, err = auth.TOTPCode(rfcSecret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if err := a.Verify(rfcSecret, good, clk.Now()); err != nil {
		t.Fatalf("after the lock expired: %v", err)
	}
}

func TestSuccessClearsTheFailureCount(t *testing.T) {
	t.Parallel()
	clk := clock.NewFake(time.Unix(1_700_000_000, 0).UTC())
	var a auth.Attempt

	for range auth.TOTPMaxFailures - 1 {
		_ = a.Verify(rfcSecret, "000000", clk.Now())
	}
	if a.Failures != auth.TOTPMaxFailures-1 {
		t.Fatalf("Failures = %d, want %d", a.Failures, auth.TOTPMaxFailures-1)
	}
	good, err := auth.TOTPCode(rfcSecret, auth.TOTPStep(clk.Now()))
	if err != nil {
		t.Fatalf("TOTPCode: %v", err)
	}
	if err := a.Verify(rfcSecret, good, clk.Now()); err != nil {
		t.Fatalf("a correct code below the limit: %v", err)
	}
	if a.Failures != 0 {
		t.Fatalf("Failures = %d after a success, want 0", a.Failures)
	}
	if a.LockedUntil != nil {
		t.Fatalf("LockedUntil = %v after a success, want nil", a.LockedUntil)
	}
}

func TestTOTPURINamesTheParametersTheServerAccepts(t *testing.T) {
	t.Parallel()
	uri := auth.TOTPURI("tix (acme)", "ada@example.test", rfcSecret)
	for _, want := range []string{
		"otpauth://totp/", "secret=" + rfcSecret, "algorithm=SHA1", "digits=6", "period=30",
	} {
		if !strings.Contains(uri, want) {
			t.Errorf("TOTPURI() = %q, want it to contain %q", uri, want)
		}
	}
}

func TestNormalizeTOTPCodeStripsSpacing(t *testing.T) {
	t.Parallel()
	if got := auth.NormalizeTOTPCode(" 123 456 "); got != "123456" {
		t.Fatalf("NormalizeTOTPCode() = %q, want %q", got, "123456")
	}
}

func TestAMalformedSecretNeverVerifies(t *testing.T) {
	t.Parallel()
	if _, ok := auth.MatchTOTPCode("not base32!", "123456", time.Unix(0, 0)); ok {
		t.Fatal("a code verified against a secret that does not decode")
	}
}
