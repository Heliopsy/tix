// SPDX-License-Identifier: AGPL-3.0-or-later

// Package auth hashes credentials, mints tokens and resolves actors from requests.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/heliopsy/tix/internal/core"
	"golang.org/x/crypto/argon2"
)

// algorithm names the only password hash this package mints.
const algorithm = "argon2id"

// argonVersion is the argon2 version the encoding records.
const argonVersion = argon2.Version

// Params are the argon2id cost parameters recorded alongside every hash.
type Params struct {
	Memory     uint32
	Time       uint32
	Threads    uint8
	SaltLength uint32
	KeyLength  uint32
}

// DefaultParams returns the production cost parameters: 46 MiB, two passes,
// one lane. That is twice the passes of OWASP's m=46 MiB, t=1 option; see
// docs/deployment.md#password-hashing-memory for why not 64 MiB.
func DefaultParams() Params {
	return Params{Memory: 46 * 1024, Time: 2, Threads: 1, SaltLength: 16, KeyLength: 32}
}

// Floors on operator-chosen parameters. Every configuration OWASP lists for
// argon2id has at least MinMemory and at least MinMemoryTime of memory times
// passes (m=7 MiB, t=5 is the cheapest), so a setting below either is weaker
// than any of them.
const (
	MinMemory     = 7 * 1024
	MinMemoryTime = 7 * 1024 * 5
)

// CheckPolicy reports whether p is at least as strong as the OWASP floor, for
// parameters an operator configured rather than the test-only ones.
func (p Params) CheckPolicy() error {
	if err := p.Validate(); err != nil {
		return err
	}
	if p.Memory < MinMemory || uint64(p.Memory)*uint64(p.Time) < MinMemoryTime {
		return core.Invalid("argon2id needs at least %d KiB and %d KiB times passes", MinMemory, MinMemoryTime)
	}
	return nil
}

// TestParams returns deliberately cheap parameters for use in tests only.
func TestParams() Params {
	return Params{Memory: 64, Time: 1, Threads: 1, SaltLength: 16, KeyLength: 32}
}

// Bounds on decoded hash components. The encoded hash is attacker-controlled on
// the verify path, so its lengths are checked before any conversion.
const (
	MaxSaltLength = 1024
	MaxKeyLength  = 1024
)

// Validate reports whether the parameters can produce a usable hash.
func (p Params) Validate() error {
	switch {
	case p.Memory < 8:
		return core.Invalid("argon2id memory must be at least 8 KiB")
	case p.Time == 0:
		return core.Invalid("argon2id time must be at least 1")
	case p.Threads == 0:
		return core.Invalid("argon2id threads must be at least 1")
	case p.SaltLength < 8:
		return core.Invalid("argon2id salt must be at least 8 bytes")
	case p.KeyLength < 16:
		return core.Invalid("argon2id key must be at least 16 bytes")
	case p.SaltLength > MaxSaltLength:
		return core.Invalid("argon2id salt must be at most %d bytes", MaxSaltLength)
	case p.KeyLength > MaxKeyLength:
		return core.Invalid("argon2id key must be at most %d bytes", MaxKeyLength)
	}
	return nil
}

// Hasher turns plaintext passwords into encoded argon2id hashes.
type Hasher struct {
	params Params
}

// NewHasher returns a hasher using the production parameters.
func NewHasher() *Hasher { return &Hasher{params: DefaultParams()} }

// NewHasherWithParams returns a hasher using explicit parameters.
func NewHasherWithParams(p Params) *Hasher { return &Hasher{params: p} }

// Params returns the parameters this hasher mints with.
func (h *Hasher) Params() Params { return h.params }

// Hash validates the password and returns its encoded argon2id hash. The
// derivation runs under the process-wide bound; a caller that cannot get a
// slot before ctx ends or HashWait passes gets ErrHashBusy.
func (h *Hasher) Hash(ctx context.Context, password string) (string, error) {
	if err := ValidatePassword(password); err != nil {
		return "", err
	}
	if err := h.params.Validate(); err != nil {
		return "", err
	}
	salt, err := randomBytes(h.params.SaltLength)
	if err != nil {
		return "", err
	}
	var key []byte
	if err := hashGate().do(ctx, func() { key = derivation(password, salt, h.params) }); err != nil {
		return "", err
	}
	return encode(h.params, salt, key), nil
}

// Dummy returns an encoded hash at this hasher's parameters that no password
// matches, without deriving one. Verifying against it costs exactly what
// verifying against a real hash at the same parameters costs, which is what
// a login for an unknown account needs.
func (h *Hasher) Dummy() (string, error) {
	salt, err := randomBytes(h.params.SaltLength)
	if err != nil {
		return "", err
	}
	key, err := randomBytes(h.params.KeyLength)
	if err != nil {
		return "", err
	}
	return encode(h.params, salt, key), nil
}

// randomBytes returns n bytes from the system's random source.
func randomBytes(n uint32) ([]byte, error) {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return nil, core.Internal("generating password salt").Wrap(err)
	}
	return b, nil
}

// ValidatePassword checks the password against the minimum length policy.
func ValidatePassword(password string) error {
	if len(password) < core.MinPasswordLength {
		return core.Invalid("password must be at least %d characters", core.MinPasswordLength)
	}
	return nil
}

// Verify reports whether password matches the encoded argon2id hash.
//
// Derivation runs under a process-wide concurrency bound, so a burst of logins
// cannot allocate one hash's memory parameter per inbound request. A caller
// that gets no slot gets ErrHashBusy, which is the same answer for every
// account and so leaks nothing. The comparison stays constant time, and both
// the matching and the non-matching path derive exactly once.
func Verify(ctx context.Context, encoded, password string) error {
	params, salt, want, err := decode(encoded)
	if err != nil {
		return err
	}
	var got []byte
	if err := hashGate().do(ctx, func() { got = derivation(password, salt, params) }); err != nil {
		return err
	}
	if subtle.ConstantTimeCompare(got, want) != 1 {
		return core.Unauthenticated("invalid credentials")
	}
	return nil
}

// NeedsRehash reports whether a stored hash was made with other parameters
// than the wanted policy, stronger or weaker. A hash at a costlier setting is
// rehashed down too: its cost is memory every sign-in holds, and the policy
// is what the operator sized the server for.
func NeedsRehash(encoded string, want Params) bool {
	params, _, _, err := decode(encoded)
	if err != nil {
		return true
	}
	return params != want
}

// derivation is derive, as a variable so a test can watch it run.
var derivation = derive

// derive computes the argon2id key for a password.
func derive(password string, salt []byte, p Params) []byte {
	return argon2.IDKey([]byte(password), salt, p.Time, p.Memory, p.Threads, p.KeyLength)
}

// encode renders the self-describing hash string.
func encode(p Params, salt, key []byte) string {
	b64 := base64.RawStdEncoding
	return fmt.Sprintf("$%s$v=%d$m=%d,t=%d,p=%d$%s$%s",
		algorithm, argonVersion, p.Memory, p.Time, p.Threads,
		b64.EncodeToString(salt), b64.EncodeToString(key))
}

// decode parses an encoded hash into its parameters, salt and key.
func decode(encoded string) (Params, []byte, []byte, error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[0] != "" {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash")
	}
	if parts[1] != algorithm {
		return Params{}, nil, nil, core.Unauthenticated("unsupported password hash algorithm")
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argonVersion {
		return Params{}, nil, nil, core.Unauthenticated("unsupported password hash version")
	}
	var p Params
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Time, &p.Threads); err != nil {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash parameters")
	}
	b64 := base64.RawStdEncoding
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash salt")
	}
	key, err := b64.DecodeString(parts[5])
	if err != nil {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash key")
	}
	if len(salt) > MaxSaltLength || len(key) > MaxKeyLength {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash")
	}
	p.SaltLength = uint32(len(salt)) // #nosec G115 -- bounded by MaxSaltLength above
	p.KeyLength = uint32(len(key))   // #nosec G115 -- bounded by MaxKeyLength above
	if err := p.Validate(); err != nil {
		return Params{}, nil, nil, core.Unauthenticated("malformed password hash parameters")
	}
	return p, salt, key, nil
}
