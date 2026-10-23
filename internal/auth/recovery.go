// SPDX-License-Identifier: AGPL-3.0-or-later

package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/hex"
	"strings"

	"github.com/heliopsy/tix/internal/core"
)

// Recovery codes: ten of them, each carrying 128 bits from crypto/rand,
// rendered base32 and shown exactly once.
const (
	RecoveryCodeCount = 10
	RecoveryCodeBytes = 16
)

// recoveryEncoding renders a code in the alphabet a person can read aloud
// without ambiguity between the characters that look alike.
var recoveryEncoding = base32.StdEncoding.WithPadding(base32.NoPadding)

// NewRecoveryCodes mints a fresh set, returning the codes to show once and the
// hashes to store.
func NewRecoveryCodes() (codes, hashes []string, err error) {
	codes = make([]string, 0, RecoveryCodeCount)
	hashes = make([]string, 0, RecoveryCodeCount)
	for range RecoveryCodeCount {
		raw := make([]byte, RecoveryCodeBytes)
		if _, err := rand.Read(raw); err != nil {
			return nil, nil, core.Internal("reading random bytes").Wrap(err)
		}
		code := recoveryEncoding.EncodeToString(raw)
		codes = append(codes, formatRecoveryCode(code))
		hashes = append(hashes, HashRecoveryCode(code))
	}
	return codes, hashes, nil
}

// formatRecoveryCode groups a code so a person can copy it without losing
// their place. The grouping is cosmetic; NormalizeRecoveryCode removes it.
func formatRecoveryCode(code string) string {
	const group = 4
	var b strings.Builder
	for i, r := range code {
		if i > 0 && i%group == 0 {
			b.WriteByte('-')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// NormalizeRecoveryCode strips the grouping and case a person may have typed.
func NormalizeRecoveryCode(code string) string {
	return strings.ToUpper(strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			return r
		default:
			return -1
		}
	}, code))
}

// HashRecoveryCode is the stored form of a recovery code.
//
// SHA-256 rather than argon2id, deliberately. argon2id exists to make guessing
// a low-entropy human-chosen secret expensive, and a 128-bit random code is
// not guessable at any work factor, so the stretching buys nothing. It costs
// something real: verification has to try a submitted code against every
// unused hash, and ten argon2id comparisons at the parameters tix uses for
// passwords is most of a second of CPU on a path an attacker can call
// repeatedly.
func HashRecoveryCode(code string) string {
	sum := sha256.Sum256([]byte(NormalizeRecoveryCode(code)))
	return hex.EncodeToString(sum[:])
}

// MatchRecoveryCode returns the index of the hash a submitted code belongs to.
// Every candidate is compared, so the time taken does not say which one
// matched or whether any did.
func MatchRecoveryCode(hashes []string, code string) (int, bool) {
	if NormalizeRecoveryCode(code) == "" {
		return 0, false
	}
	want := HashRecoveryCode(code)
	index, found := 0, false
	for i, h := range hashes {
		if subtle.ConstantTimeCompare([]byte(h), []byte(want)) == 1 && !found {
			index, found = i, true
		}
	}
	return index, found
}
